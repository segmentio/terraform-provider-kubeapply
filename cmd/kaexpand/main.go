package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/ghodss/yaml"
	"github.com/segmentio/cli"
	"github.com/segmentio/terraform-provider-kubeapply/pkg/cluster"
	"github.com/segmentio/terraform-provider-kubeapply/pkg/cluster/diff"
	"github.com/segmentio/terraform-provider-kubeapply/pkg/util"
	log "github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	"k8s.io/klog/v2"
)

const (
	expandHelp = "expand manifests in a profile"
	expandDesc = `
Runs an expansion using the same procedure as the terraform provider.

Usage:

  kaexpand [path] --account-id=[id] --cluster=[cluster] -p [key=value] -p [key=value]

Custom parameters can be specified via '-p', in which case the values will be treated as literal
strings or '-j', which will do a JSON unmarshal before inserting the values into the
Parameters struct. The latter can be used to encode booleans, maps, etc.

The outputs can be fed to 'kubectl diff' or 'kubectl apply'. The tool also exposes '--diff'
and '--apply' flags for running the previous commands automatically after expansion.

This tool is for debugging purposes only and should not be used in production environments.
`
)

type kaExpandConfig struct {
	// Template parameters
	AccountID      string   `flag:"--account-id" help:"Account ID for cluster" default:"accountID"`
	AccountName    string   `flag:"--account-name" help:"Account name for cluster" default:"accountName"`
	Cluster        string   `flag:"--cluster" help:"Cluster name" default:"cluster"`
	Environment    string   `flag:"--environment" help:"Environment for cluster" default:"environment"`
	JSONParameters []string `flag:"-j,--json-parameter" help:"JSON parameters in key=value format" default:"-"`
	Parameters     []string `flag:"-p,--parameter" help:"Parameters in key=value format" default:"-"`
	Region         string   `flag:"--region" help:"Region for cluster" default:"environment"`

	// Behavior parameters
	Apply          bool   `flag:"--apply" help:"Run a kubectl apply after generating expanded outputs and diff" default:"false"`
	Diff           bool   `flag:"--diff" help:"Run a kubectl diff after generating expanded outputs" default:"false"`
	KubeConfigPath string `flag:"--kubeconfig" help:"Path to kubeconfig for diff and apply" default:"-"`
	Output         string `flag:"-o,--output" help:"Directory for output" default:"-"`

	Debug bool `flag:"--debug" help:"Log at debug level" default:"false"`
}

func init() {
	log.SetFormatter(&prefixed.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   true,
	})

	log.SetLevel(log.InfoLevel)
	klog.SetOutput(os.Stderr)
}

var (
	boldPrinter = color.New(color.Bold).SprintfFunc()
	redPrinter  = color.New(color.FgRed).SprintfFunc()
)

func main() {
	cli.Exec(
		&cli.CommandFunc{
			Help: expandHelp,
			Desc: strings.TrimSpace(expandDesc),
			Func: func(config kaExpandConfig, profilePath string) {
				if config.Debug {
					log.SetLevel(log.DebugLevel)
				}

				if profilePath == "" {
					log.Fatal(
						"Must provide a single argument containing path to profile manifests to be expanded.",
					)
				}

				ctx := context.Background()
				clusterConfig, err := config.toClusterConfig()
				if err != nil {
					log.Fatal(err)
				}

				var outputDir string

				if config.Output != "" {
					outputDir = config.Output
				} else {
					var err error
					outputDir, err = ioutil.TempDir("", "expanded_")
					if err != nil {
						log.Fatal(err)
					}
				}

				err = runExpand(ctx, profilePath, outputDir, clusterConfig)
				if err != nil {
					log.Fatal(err)
				}

				if config.Diff || config.Apply {
					var kubeConfigPath string

					if config.KubeConfigPath != "" {
						kubeConfigPath = config.KubeConfigPath
					} else {
						kubeConfigPath = os.Getenv("KUBECONFIG")
						if kubeConfigPath == "" {
							log.Fatal("Must either set --kubeconfig or KUBECONFIG environment variable")
						}
					}

					clusterConfig.KubeConfigPath = kubeConfigPath

					client, err := cluster.NewKubeClient(
						ctx,
						&cluster.ClientConfig{
							Config: clusterConfig,
							Debug:  config.Debug,
						},
					)
					if err != nil {
						log.Fatal(err)
					}

					if err = runDiff(ctx, outputDir, client); err != nil {
						log.Fatal(err)
					}

					if config.Apply {
						if err = runApply(ctx, outputDir, client, kubeConfigPath); err != nil {
							log.Fatal(err)
						}
					}
				} else {
					log.Infof(
						"Run 'kubectl diff -R -f %s' to get a diff against the Kubernetes API.",
						outputDir,
					)
				}
			},
		},
	)
}

func (c kaExpandConfig) toClusterConfig() (*cluster.Config, error) {
	clusterConfig := &cluster.Config{
		Cluster:     c.Cluster,
		Region:      c.Region,
		AccountID:   c.AccountID,
		AccountName: c.AccountName,
		Environment: c.Environment,
		Parameters:  map[string]interface{}{},
	}

	for _, parameter := range c.Parameters {
		components := strings.SplitN(parameter, "=", 2)
		if len(components) != 2 {
			return nil, fmt.Errorf("Parameter is not in key=value format: %s", parameter)
		}

		clusterConfig.Parameters[components[0]] = components[1]
	}

	for _, parameter := range c.JSONParameters {
		components := strings.SplitN(parameter, "=", 2)
		if len(components) != 2 {
			return nil, fmt.Errorf("JSON parameter is not in key=value format: %s", parameter)
		}

		var value interface{}
		if err := json.Unmarshal([]byte(components[1]), &value); err != nil {
			return nil, err
		}

		clusterConfig.Parameters[components[0]] = value
	}

	return clusterConfig, nil
}

func runExpand(
	ctx context.Context,
	profilePath string,
	outputDir string,
	clusterConfig *cluster.Config,
) error {
	log.Infof("Writing expanded outputs to %s", outputDir)

	configBytes, err := yaml.Marshal(clusterConfig)
	if err != nil {
		return err
	}

	log.Infof(
		"Expanding manifests in %s using cluster config:\n%s",
		profilePath,
		string(configBytes),
	)

	if err := util.RecursiveCopy(profilePath, outputDir); err != nil {
		return err
	}

	if err := util.ApplyTemplate(outputDir, clusterConfig, true, true); err != nil {
		return err
	}

	log.Infof("Successfully wrote outputs to %s", outputDir)
	return nil
}

func runDiff(ctx context.Context, path string, client cluster.Client) error {
	log.Infof("Running diff for configs in %s", path)

	results, err := client.DiffStructured(ctx, []string{path}, false)
	if err != nil {
		return err
	}

	diff.PrintFull(results)
	return nil
}

func runApply(
	ctx context.Context,
	path string,
	client cluster.Client,
	kubeConfigPath string,
) error {
	log.Infof("This will run apply for the configs in %s", path)
	log.Warnf(
		"It will use the kubeconfig in %s; %s",
		boldPrinter(kubeConfigPath),
		redPrinter("please be sure that this is a safe operation before continuing."),
	)

	if ok, _ := util.Confirm(ctx, "Ok to continue?", false); !ok {
		return fmt.Errorf("Stopping because of user response")
	}

	results, err := client.Apply(ctx, []string{path}, false)
	if err != nil {
		return nil
	}

	log.Infof("Here are the results:\n%s", string(results))
	return nil
}
