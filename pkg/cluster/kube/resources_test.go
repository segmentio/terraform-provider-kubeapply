package kube

import (
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

func TestParseResourcesTable(t *testing.T) {
	resources, err := parseResourcesTable(testAPIResourcesOutput)
	require.NoError(t, err)
	assert.Equal(
		t,
		[]apiResource{
			{
				name:       "bindings",
				shortNames: []string{},
				apiVersion: "v1",
				namespaced: true,
				kind:       "Binding",
			},
			{
				name:       "componentstatuses",
				shortNames: []string{"cs"},
				apiVersion: "v1",
				namespaced: false,
				kind:       "ComponentStatus",
			},
			{
				name:       "configmaps",
				shortNames: []string{"cm", "cmx"},
				apiVersion: "v1",
				namespaced: true,
				kind:       "ConfigMap",
			},
		},
		resources,
	)

	_, err = parseResourcesTable("bad input")
	require.Error(t, err)
}
