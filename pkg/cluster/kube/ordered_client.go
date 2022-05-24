package kube

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/segmentio/terraform-provider-kubeapply/pkg/util"
	log "github.com/sirupsen/logrus"
)

var (
	//go:embed scripts/raw-diff.sh
	rawDiffScript string
)

// TODO: Switch to a YAML library that supports doing this splitting for us.
var sep = regexp.MustCompile("(?:^|\\s*\n)---\\s*")

// OrderedClient is a kubectl-wrapped client that tries to be clever about the order
// in which resources are created or destroyed.
type OrderedClient struct {
	kubeConfigPath string
	keepConfigs    bool
	extraEnv       []string
	debug          bool
	serverSide     bool
}

// NewOrderedClient returns a new OrderedClient instance.
func NewOrderedClient(
	kubeConfigPath string,
	keepConfigs bool,
	extraEnv []string,
	debug bool,
	serverSide bool,
) *OrderedClient {
	return &OrderedClient{
		kubeConfigPath: kubeConfigPath,
		keepConfigs:    keepConfigs,
		extraEnv:       extraEnv,
		debug:          debug,
		serverSide:     serverSide,
	}
}

// Apply runs kubectl apply on the manifests in the argument path. The apply is done
// in the optimal order based on resource type.
func (k *OrderedClient) Apply(
	ctx context.Context,
	applyPaths []string,
	output bool,
	format string,
	dryRun bool,
) ([]byte, error) {
	tempDir, err := ioutil.TempDir("", "kubeapply_manifests_")
	if err != nil {
		return nil, err
	}
	defer func() {
		if k.keepConfigs {
			log.Infof("Keeping temporary configs in %s", tempDir)
		} else {
			os.RemoveAll(tempDir)
		}
	}()

	manifests, err := GetManifests(applyPaths)
	if err != nil {
		return nil, err
	}
	SortManifests(manifests)

	for m, manifest := range manifests {
		// kubectl applies resources in their lexicographic ordering, so this naming scheme
		// should force it to apply the manifests in the order we want.

		var name string
		var namespace string

		if manifest.Head.Metadata != nil {
			name = manifest.Head.Metadata.Name
			namespace = manifest.Head.Metadata.Namespace
		}

		tempPath := filepath.Join(
			tempDir,
			fmt.Sprintf(
				"%06d_%s_%s_%s.yaml",
				m,
				name,
				namespace,
				manifest.Head.Kind,
			),
		)

		err = ioutil.WriteFile(tempPath, []byte(manifest.Contents), 0644)
		if err != nil {
			return nil, err
		}
	}

	args := []string{
		"apply",
		"--kubeconfig",
		k.kubeConfigPath,
		"-R",
		"-f",
		tempDir,
	}
	if k.serverSide {
		args = append(args, "--server-side", "true")
	}
	if k.debug {
		args = append(args, "-v", "8")
	}
	if format != "" {
		args = append(args, "-o", format)
	}
	if dryRun {
		args = append(args, "--dry-run")
	}

	if output {
		return runKubectlOutput(
			ctx,
			args,
			k.extraEnv,
		)
	}
	return nil, runKubectl(
		ctx,
		args,
		k.extraEnv,
	)
}

// Diff runs kubectl diff for the configs at the argument path.
func (k *OrderedClient) Diff(
	ctx context.Context,
	configPaths []string,
	structured bool,
) ([]byte, error) {
	var diffCmd string

	tempDir, err := ioutil.TempDir("", "kubeapply_diff_")
	if err != nil {
		return nil, err
	}
	defer func() {
		if k.keepConfigs {
			log.Infof("Keeping temporary configs in %s", tempDir)
		} else {
			os.RemoveAll(tempDir)
		}
	}()

	args := []string{
		"--kubeconfig",
		k.kubeConfigPath,
		"diff",
		"-R",
	}

	for _, configPath := range configPaths {
		args = append(args, "-f", configPath)
	}

	if k.serverSide {
		args = append(args, "--server-side", "true")
	}
	if k.debug {
		args = append(args, "-v", "8")
	}

	envVars := []string{}
	var diffScriptPath string
	var diffScriptContents string

	if structured {
		diffCmd = "kadiff"
	} else {
		diffScriptPath = "raw-diff.sh"
		diffScriptPath = rawDiffScript

		diffCmd = filepath.Join(tempDir, diffScriptPath)
		err = ioutil.WriteFile(
			diffCmd,
			[]byte(diffScriptContents),
			0755,
		)
		if err != nil {
			return nil, err
		}
	}

	envVars = append(
		envVars,
		fmt.Sprintf("KUBECTL_EXTERNAL_DIFF=%s", diffCmd),
	)
	for _, extraEnv := range k.extraEnv {
		envVars = append(envVars, extraEnv)
	}

	return runKubectlOutput(
		ctx,
		args,
		envVars,
	)
}

type deleteResource struct {
	kind      string
	name      string
	namespace string
}

// Delete runs kubectl delete for the configs at the argument path for
// manifests that match the provided ids.
func (k *OrderedClient) Delete(
	ctx context.Context,
	ids []string,
) ([]byte, error) {
	toDelete := []idComponents{}

	for _, id := range ids {
		idComponents := manifestIDToComponents(id)
		if idComponents.name == "" {
			log.Warnf("Could not parse id %s", id)
		}
		toDelete = append(toDelete, idComponents)
	}

	apiResources, err := getApiResources(k.kubeConfigPath)
	if err != nil {
		return nil, err
	}
	resourceNames := map[string]string{}

	for _, apiResource := range apiResources {
		resourceNames[apiResource.kind] = apiResource.name
	}

	allResults := [][]byte{}

	for _, idComponents := range toDelete {
		resourceName, ok := resourceNames[idComponents.kind]
		if !ok {
			log.Warnf(
				"Could not find resource name for kind %s; skipping delete",
				idComponents.kind,
			)
			continue
		}

		args := []string{
			"--kubeconfig",
			k.kubeConfigPath,
			"--ignore-not-found=true",
			"--wait=false",
			"delete",
			resourceName,
			idComponents.name,
		}
		if idComponents.namespace != "" {
			args = append(
				args,
				"-n",
				idComponents.namespace,
			)
		}

		results, err := runKubectlOutput(
			ctx,
			args,
			nil,
		)
		if err != nil {
			return results, err
		}
		allResults = append(allResults, bytes.TrimSpace(results))
	}

	return bytes.Join(allResults, []byte("\n")), nil
}

func runKubectl(ctx context.Context, args []string, extraEnv []string) error {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return err
	}

	return util.RunCmdWithPrinters(
		ctx,
		kubectlPath,
		args,
		extraEnv,
		nil,
		util.LogrusInfoPrinter("[kubectl]"),
		util.LogrusInfoPrinter("[kubectl]"),
	)
}

func runKubectlOutput(
	ctx context.Context,
	args []string,
	extraEnv []string,
) ([]byte, error) {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return nil, err
	}

	log.Infof("Running kubectl with args %+v", args)
	cmd := exec.CommandContext(ctx, kubectlPath, args...)

	envVars := os.Environ()
	envVars = append(envVars, extraEnv...)
	cmd.Env = envVars

	return cmd.CombinedOutput()
}
