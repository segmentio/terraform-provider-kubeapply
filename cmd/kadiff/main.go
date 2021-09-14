package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/segmentio/cli"
	"github.com/segmentio/terraform-provider-kubeapply/pkg/cluster/diff"
	log "github.com/sirupsen/logrus"
	"k8s.io/klog/v2"
)

const (
	diffHelp = "generate a structured diff between manifests in two directories"
	diffDesc = `
Generates a structured diff between Kubernetes manifests in two directories.

This tool is used by the provider to generate structured diffs.
`
)

type kaDiffConfig struct {
	Debug bool `flag:"--debug" help:"Log at debug level" default:"false"`

	ContextLines  int `flag:"--context-lines" help:"Number of context lines to show in diff outputs" default:"3"`
	MaxLineLength int `flag:"--max-line-length" help:"Max length of lines from diff" default:"256"`
	MaxSize       int `flag:"--max-size" help:"Total maximum size of diff after clipping long lines" default:"3000"`
}

func init() {
	log.SetLevel(log.InfoLevel)
	klog.SetOutput(os.Stderr)
}

func main() {
	cli.Exec(
		&cli.CommandFunc{
			Help: diffHelp,
			Desc: diffDesc,
			Func: func(config kaDiffConfig, old string, new string) {
				if config.Debug {
					log.SetLevel(log.DebugLevel)
				}

				ctx := context.Background()
				if err := runKADiff(
					ctx,
					old,
					new,
					config,
				); err != nil {
					log.Fatal(err)
				}
			},
		},
	)
}

func runKADiff(
	ctx context.Context,
	old string,
	new string,
	config kaDiffConfig,
) error {
	results, err := diff.DiffKube(
		old,
		new,
		diff.DiffConfig{
			ContextLines:  config.ContextLines,
			MaxLineLength: config.MaxLineLength,
			MaxSize:       config.MaxSize,
		},
	)
	if err != nil {
		return err
	}

	wrappedResults := diff.Results{
		Results: results,
	}

	jsonBytes, err := json.MarshalIndent(wrappedResults, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(jsonBytes))

	return nil
}
