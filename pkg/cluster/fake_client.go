package cluster

import (
	"context"
	"errors"
	"fmt"

	"github.com/segmentio/terraform-provider-kubeapply/pkg/cluster/apply"
	"github.com/segmentio/terraform-provider-kubeapply/pkg/cluster/diff"
)

var _ Client = (*FakeClient)(nil)

// FakeClient is a fake implementation of a Client. For testing purposes only.
type FakeClient struct {
	clusterConfig   *Config
	subpathOverride string
	kubectlErr      error

	NoDiffs bool
	Calls   []FakeClientCall
}

// FakeClientCall records a call that was made using the FakeClient.
type FakeClientCall struct {
	CallType string
	Paths    []string
}

// NewFakeClient returns a FakeClient that works without errors.
func NewFakeClient(
	ctx context.Context,
	config *ClientConfig,
) (Client, error) {
	return &FakeClient{
		clusterConfig: config.Config,
	}, nil
}

// NewFakeClientError returns a FakeClient that simulates an error when
// running kubectl.
func NewFakeClientError(
	ctx context.Context,
	config *ClientConfig,
) (Client, error) {
	return &FakeClient{
		clusterConfig: config.Config,
		kubectlErr:    errors.New("kubectl error"),
	}, nil
}

// Apply runs a fake apply using the configs in the argument path.
func (cc *FakeClient) Apply(
	ctx context.Context,
	paths []string,
	serverSide bool,
) ([]byte, error) {
	cc.Calls = append(
		cc.Calls,
		FakeClientCall{
			CallType: "Apply",
			Paths:    paths,
		},
	)
	return []byte(
			fmt.Sprintf(
				"apply result for %s with paths %+v",
				cc.clusterConfig.Cluster,
				paths,
			),
		),
		cc.kubectlErr
}

// ApplyStructured runs a fake structured apply using the configs in the argument
// path.
func (cc *FakeClient) ApplyStructured(
	ctx context.Context,
	paths []string,
	serverSide bool,
) ([]apply.Result, error) {
	cc.Calls = append(
		cc.Calls,
		FakeClientCall{
			CallType: "ApplyStructured",
			Paths:    paths,
		},
	)
	return []apply.Result{
		{
			Kind: "Deployment",
			Name: fmt.Sprintf(
				"apply result for %s with paths %+v",
				cc.clusterConfig.Cluster,
				paths,
			),
			Namespace:  "test-namespace",
			OldVersion: "1234",
			NewVersion: "5678",
		},
	}, cc.kubectlErr
}

// Delete deletes the resources associated with one or more configs.
func (cc *FakeClient) Delete(
	ctx context.Context,
	ids []string,
) ([]byte, error) {
	cc.Calls = append(
		cc.Calls,
		FakeClientCall{
			CallType: "Delete",
			Paths:    ids,
		},
	)
	return []byte(
			fmt.Sprintf(
				"delete result for %s with ids %+v",
				cc.clusterConfig.Cluster,
				ids,
			),
		),
		cc.kubectlErr
}

// Diff gets the diffs between the configs at the given path and the actual state of resources
// in the cluster. It returns the raw output.
func (cc *FakeClient) Diff(
	ctx context.Context,
	paths []string,
	serverSide bool,
) ([]byte, error) {
	cc.Calls = append(
		cc.Calls,
		FakeClientCall{
			CallType: "Diff",
			Paths:    paths,
		},
	)
	if cc.NoDiffs {
		return []byte{}, nil
	}

	return []byte(
			fmt.Sprintf(
				"diff result for %s with paths %+v",
				cc.clusterConfig.Cluster,
				paths,
			),
		),
		cc.kubectlErr
}

// DiffStructured gets the diffs between the configs at the given path and the actual state of
// resources in the cluster. It returns structured output.
func (cc *FakeClient) DiffStructured(
	ctx context.Context,
	paths []string,
	serverSide bool,
) ([]diff.Result, error) {
	cc.Calls = append(
		cc.Calls,
		FakeClientCall{
			CallType: "DiffStructured",
			Paths:    paths,
		},
	)
	if cc.NoDiffs {
		return []diff.Result{}, nil
	}

	return []diff.Result{
			{
				Name: "result",
				RawDiff: fmt.Sprintf(
					// Don't include paths since this can lead to terraform diff
					// instability
					"structured diff result for %s",
					cc.clusterConfig.Cluster,
				),
			},
		},
		cc.kubectlErr
}

// Config returns this client's cluster config.
func (cc *FakeClient) Config() *Config {
	cc.Calls = append(
		cc.Calls,
		FakeClientCall{
			CallType: "Config",
		},
	)
	return cc.clusterConfig
}

// Close closes the client.
func (cc *FakeClient) Close() error {
	cc.Calls = append(
		cc.Calls,
		FakeClientCall{
			CallType: "Close",
		},
	)
	return nil
}
