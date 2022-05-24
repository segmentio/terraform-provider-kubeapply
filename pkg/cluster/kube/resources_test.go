package kube

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAPIResourcesOutput = `
NAME                SHORTNAMES   APIVERSION   NAMESPACED   KIND
bindings                         v1           true         Binding
componentstatuses   cs           v1           false        ComponentStatus
configmaps          cm,cmx       v1           true         ConfigMap
`

func TestGetApiResources(t *testing.T) {

	apiResourceLoader = func(kubeConfigPath string) ([]*v1.APIResourceList, error) {
		return []*v1.APIResourceList{
			{
				TypeMeta: v1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "any",
				},
				GroupVersion: "apps/v1",
				APIResources: []v1.APIResource{
					{Name: "deployments", SingularName: "deployment", Namespaced: true, Group: "apps/v1", Version: "apps/v1", Kind: "Deployment", Verbs: []string{"create", "list", "get"}, ShortNames: []string{"deploy"}, Categories: []string{}, StorageVersionHash: "string"},
					{Name: "daemonsets", SingularName: "daemonset", Namespaced: true, Group: "apps/v1", Version: "apps/v1", Kind: "DaemonSet", Verbs: []string{"create", "list", "get"}, ShortNames: []string{"ds"}, Categories: []string{}, StorageVersionHash: "string"},
				},
			},
			{
				TypeMeta: v1.TypeMeta{
					APIVersion: "batch/v1",
					Kind:       "any",
				},
				GroupVersion: "batch/v1",
				APIResources: []v1.APIResource{
					{Name: "jobs", SingularName: "job", Namespaced: true, Group: "batch/v1", Version: "batch/v1", Kind: "Job", Verbs: []string{"create", "list", "get"}, ShortNames: nil, Categories: []string{}, StorageVersionHash: "string"},
				},
			},
		}, nil
	}
	resources, err := getApiResources("/path/to/fake/kubeconfig.yaml")
	require.NoError(t, err)
	assert.Equal(t, []apiResource{
		{name: "deployments", shortNames: []string{"deploy"}, apiVersion: "apps/v1", namespaced: true, kind: "Deployment"},
		{name: "daemonsets", shortNames: []string{"ds"}, apiVersion: "apps/v1", namespaced: true, kind: "DaemonSet"},
		{name: "jobs", apiVersion: "batch/v1", shortNames: nil, namespaced: true, kind: "Job"},
	}, resources)
}
