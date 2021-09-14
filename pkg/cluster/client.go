package cluster

import (
	"context"

	"github.com/segmentio/terraform-provider-kubeapply/pkg/cluster/apply"
	"github.com/segmentio/terraform-provider-kubeapply/pkg/cluster/diff"
)

// Client is an interface that interacts with the API of a single Kubernetes cluster.
type Client interface {
	// Apply applies all of the configs at the given path.
	Apply(ctx context.Context, paths []string, serverSide bool) ([]byte, error)

	// ApplyStructured applies all of the configs at the given path and returns structured,
	// as opposed to raw, outputs
	ApplyStructured(ctx context.Context, paths []string, serverSide bool) ([]apply.Result, error)

	// Delete deletes the resources associated with one or more configs.
	Delete(ctx context.Context, ids []string) ([]byte, error)

	// Diff gets the diffs between the configs at the given path and the actual state of resources
	// in the cluster. It returns the raw output.
	Diff(ctx context.Context, paths []string, serverSide bool) ([]byte, error)

	// DiffStructured gets the diffs between the configs at the given path and the actual state of
	// resources in the cluster. It returns structured output.
	DiffStructured(ctx context.Context, paths []string, serverSide bool) ([]diff.Result, error)

	// Config returns the config for this cluster.
	Config() *Config

	// GetNamespaceUID returns the kubernetes identifier for a given namespace in this cluster.
	GetNamespaceUID(ctx context.Context, namespace string) (string, error)

	// Close cleans up this client.
	Close() error
}

// ClientConfig stores the configuration necessary to create a Client.
type ClientConfig struct {
	// Config is the config for the cluster that we are communicating with.
	Config *Config

	// Debug indicates whether commands should be run with debug-level logging.
	Debug bool

	// KeepConfigs indicates whether kube client should keep around intermediate
	// yaml manifests. These are useful for debugging when there are apply errors.
	KeepConfigs bool

	// StreamingOutput indicates whether results should be streamed out to stdout and stderr.
	// Currently only applies to apply operations.
	StreamingOutput bool

	// Extra environment variables to add into kubectl calls.
	ExtraEnv []string
}
