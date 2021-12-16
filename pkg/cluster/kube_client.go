package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/segmentio/terraform-provider-kubeapply/pkg/cluster/apply"
	"github.com/segmentio/terraform-provider-kubeapply/pkg/cluster/diff"
	"github.com/segmentio/terraform-provider-kubeapply/pkg/cluster/kube"
	log "github.com/sirupsen/logrus"
)

var _ Client = (*KubeClient)(nil)

// KubeClient is an implementation of a Client that hits an actual Kubernetes API.
// It's backed by a kube.OrderedClient which, in turn, wraps kubectl.
type KubeClient struct {
	clusterConfig  *Config
	kubeConfigPath string
	kubeClient     *kube.OrderedClient
}

// kubeapplyDiffEvent is used for storing the last successful diff in the kubeStore.
// This value is checked before applying to ensure that the SHAs match.
type kubeapplyDiffEvent struct {
	SHA string `json:"sha"`

	UpdatedAt time.Time `json:"updatedAt"`
	UpdatedBy string    `json:"updatedBy"`
}

// NewKubeClient creates a new Client instance for a real
// Kubernetes cluster.
func NewKubeClient(
	ctx context.Context,
	config *ClientConfig,
) (Client, error) {
	var kubeConfigPath string

	if config.Config.KubeConfigPath != "" {
		kubeConfigPath = config.Config.KubeConfigPath
	} else {
		return nil, fmt.Errorf("Must provide a kubeconfig")
	}

	kubeClient := kube.NewOrderedClient(
		kubeConfigPath,
		config.KeepConfigs,
		config.ExtraEnv,
		config.Debug,
		config.Config.ServerSideApply,
	)

	hostName, err := os.Hostname()
	if err != nil {
		log.Warnf("Error getting hostname, using generic string: %s", hostName)
		hostName = "kubeapply"
	}

	return &KubeClient{
		clusterConfig:  config.Config,
		kubeConfigPath: kubeConfigPath,
		kubeClient:     kubeClient,
	}, nil
}

// Apply does a kubectl apply for the resources at the argument path.
func (cc *KubeClient) Apply(
	ctx context.Context,
	paths []string,
	serverSide bool,
) ([]byte, error) {
	return cc.execApply(ctx, paths, "", false)
}

// ApplyStructured does a structured kubectl apply for the resources at the
// argument path.
func (cc *KubeClient) ApplyStructured(
	ctx context.Context,
	paths []string,
	serverSide bool,
) ([]apply.Result, error) {
	oldContents, err := cc.execApply(ctx, paths, "json", true)
	if err != nil {
		return nil,
			fmt.Errorf(
				"Error running apply dry-run: %+v; output: %s",
				err,
				string(oldContents),
			)
	}

	oldObjs, err := apply.KubeJSONToObjects(oldContents)
	if err != nil {
		return nil, err
	}

	newContents, err := cc.execApply(ctx, paths, "json", false)
	if err != nil {
		return nil,
			fmt.Errorf(
				"Error running apply: %+v; output: %s",
				err,
				string(newContents),
			)
	}
	newObjs, err := apply.KubeJSONToObjects(newContents)
	if err != nil {
		return nil, err
	}

	results, err := apply.ObjsToResults(oldObjs, newObjs)
	if err != nil {
		return nil, err
	}
	return sortedApplyResults(results), nil
}

// Delete deletes one or more resources associated with the argument paths.
func (cc *KubeClient) Delete(
	ctx context.Context,
	ids []string,
) ([]byte, error) {
	return cc.kubeClient.Delete(ctx, ids)
}

// Diff gets the diffs between the configs at the given path and the actual state of resources
// in the cluster. It returns the raw output.
func (cc *KubeClient) Diff(
	ctx context.Context,
	paths []string,
	serverSide bool,
) ([]byte, error) {
	rawResults, err := cc.execDiff(ctx, paths, false)
	if err != nil {
		return nil, fmt.Errorf(
			"Error running diff: %+v (output: %s)",
			err,
			string(rawResults),
		)
	}

	return rawResults, nil
}

// DiffStructured gets the diffs between the configs at the given path and the actual state of
// resources in the cluster. It returns structured output.
func (cc *KubeClient) DiffStructured(
	ctx context.Context,
	paths []string,
	serverSide bool,
) ([]diff.Result, error) {
	rawResults, err := cc.execDiff(ctx, paths, true)
	if err != nil {
		return nil, fmt.Errorf(
			"Error running diff: %+v (output: %s)",
			err,
			string(rawResults),
		)
	}

	// Strip everything before the initial "{"; kubectl can insert arbitrary warnings, etc.
	// that can cause the result to not be valid JSON.
	jsonStart := bytes.Index(rawResults, []byte("{"))
	if jsonStart > 0 {
		rawResults = rawResults[jsonStart:]
	}

	results := diff.Results{}
	if err := json.Unmarshal(rawResults, &results); err != nil {
		return nil, err
	}
	return sortedDiffResults(results.Results), nil
}

// Config returns this client's cluster config.
func (cc *KubeClient) Config() *Config {
	return cc.clusterConfig
}

// Close closes the client and cleans up all of the associated resources.
func (cc *KubeClient) Close() error {
	return nil
}

func (cc *KubeClient) execApply(
	ctx context.Context,
	paths []string,
	format string,
	dryRun bool,
) ([]byte, error) {
	return cc.kubeClient.Apply(
		ctx,
		paths,
		true,
		format,
		dryRun,
	)
}

func (cc *KubeClient) execDiff(
	ctx context.Context,
	paths []string,
	structured bool,
) ([]byte, error) {
	return cc.kubeClient.Diff(
		ctx,
		paths,
		structured,
	)
}
