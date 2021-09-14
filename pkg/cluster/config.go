package cluster

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

// Config represents the configuration for a single Kubernetes cluster in a single
// region and environment / account.
type Config struct {
	// Cluster is the name of the cluster.
	//
	// Required.
	Cluster string `json:"cluster"`

	// Region is the region for this cluster, e.g. us-west-2.
	//
	// Required.
	Region string `json:"region"`

	// Env is the environment for this cluster, e.g. production.
	//
	// Required.
	Environment string `json:"environment"`

	// AccountName is the name of the account where this cluster is running.
	//
	// Required.
	AccountName string `json:"accountName"`

	// AccountID is the ID of the account where this cluster is running.
	//
	// Required.
	AccountID string `json:"accountID"`

	// Version is the cluster version.
	//
	// Optional.
	Version string `json:"version"`

	// ConfigHash is a hash of the input configs and parameters for this cluster. It
	// can be used to compare the configuration states of various clusters.
	ConfigHash string `json:"configHash"`

	// ExpandedPath is the path to the results of expanding out all of the configs for this cluster.
	//
	// Optional, defaults to "expanded/[env]/[region]" if not set.
	ExpandedPath string `json:"expandedPath"`

	// Parameters are key/value pairs to be used for go templating.
	//
	// Optional.
	Parameters map[string]interface{} `json:"parameters"`

	// KubeConfigPath is the path to a kubeconfig that can be used with this cluster.
	//
	// Optional, defaults to value set on command-line (when running kubeapply manually) or
	// automatically generated via AWS API (when running in lambdas case).
	KubeConfigPath string `json:"kubeConfig"`

	// ServerSideApply sets whether we should be using server-side applies and diffs for this
	// cluster.
	ServerSideApply bool `json:"serverSideApply"`
}

// ShortRegion converts the region in the cluster config to a short form that
// may be used in some templates.
func (c Config) ShortRegion() string {
	components := strings.Split(c.Region, "-")
	if len(components) != 3 {
		log.Warnf("Cannot convert region %s to short form", c.Region)
		return c.Region
	}

	return fmt.Sprintf("%s%s%s", components[0][0:2], components[1][0:1], components[2])
}
