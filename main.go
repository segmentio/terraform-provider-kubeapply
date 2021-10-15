package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
	"github.com/segmentio/cli"
	"github.com/segmentio/terraform-provider-kubeapply/pkg/provider"
	log "github.com/sirupsen/logrus"
)

//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs

type providerConfig struct {
	DebugServer bool `flag:"--debug-server" help:"Run debug server" default:"false"`
	DebugLogs   bool `flag:"--debug-logs" help:"Log at debug level" default:"false"`
}

func init() {
	// Terraform requires a very simple log output format; see
	// https://www.terraform.io/docs/extend/debugging.html#inserting-log-lines-into-a-provider
	// for more details.
	log.SetFormatter(&simpleFormatter{})
	log.SetLevel(log.InfoLevel)
}

func main() {
	cli.Exec(
		cli.Command(
			func(config providerConfig) {
				if config.DebugLogs {
					log.SetLevel(log.DebugLevel)
				}

				ctx := context.Background()
				if err := runProvider(ctx, config.DebugServer); err != nil {
					log.Fatal(err)
				}
			},
		),
	)
}

func runProvider(ctx context.Context, debugServer bool) error {
	log.Info("Starting kubeapply provider")

	opts := &plugin.ServeOpts{
		ProviderFunc: func() *schema.Provider {
			return provider.Provider(nil)
		},
	}

	// Terraform doesn't provide any sort of "cleanup" notification for providers, so there's
	// no way to do any cleanup when the provider exits. Doing the below as a hacky alternative
	// to ensure that the disk doesn't fill up with accumulated junk.
	threshold := time.Now().UTC().Add(-1 * time.Hour)

	tempDirs, _ := filepath.Glob(
		filepath.Join(os.TempDir(), "kubeapply_*"),
	)
	for _, tempDir := range tempDirs {
		info, err := os.Stat(tempDir)
		if err != nil {
			log.Warnf("Error getting info for %s: %+v", tempDir, err)
			continue
		}
		if info.ModTime().Before(threshold) {
			log.Infof("Cleaning temp dir %s", tempDir)
			err = os.RemoveAll(tempDir)
			if err != nil {
				log.Warnf("Error deleting %s: %+v", tempDir, err)
			}
		}
	}

	if debugServer {
		log.Info("Running server in debug mode")
		return plugin.Debug(ctx, "segmentio/kubeapply", opts)
	}

	plugin.Serve(opts)
	return nil
}

type simpleFormatter struct {
}

func (s *simpleFormatter) Format(entry *log.Entry) ([]byte, error) {
	if len(entry.Data) > 0 {
		fieldsJSON, _ := json.Marshal(entry.Data)

		return []byte(
			fmt.Sprintf(
				"[%s] %s (%+v)\n",
				strings.ToUpper(entry.Level.String()),
				entry.Message,
				string(fieldsJSON),
			),
		), nil
	} else {
		return []byte(
			fmt.Sprintf(
				"[%s] %s\n",
				strings.ToUpper(entry.Level.String()),
				entry.Message,
			),
		), nil
	}
}
