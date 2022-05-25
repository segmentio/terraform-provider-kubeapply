package provider

import (
	"context"
	"encoding/base64"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderTokenExec(t *testing.T) {
	ctx := context.Background()
	provider := Provider(nil)

	err := provider.InternalValidate()
	require.NoError(t, err)

	testCACertificate := base64.StdEncoding.EncodeToString([]byte("testClusterCACertificate"))

	config := terraform.NewResourceConfigRaw(
		map[string]interface{}{
			"host":                   "testHost",
			"cluster_ca_certificate": testCACertificate,
			"token":                  "testToken",
			"exec": []interface{}{
				map[string]interface{}{
					"api_version": "client.authentication.k8s.io/v1beta1",
					"command":     "testCommand",
					"env": map[string]interface{}{
						"key1": "value1",
						"key2": "value2",
					},
					"args": []interface{}{
						"arg1",
						"arg2",
					},
				},
			},
		},
	)

	diags := provider.Configure(ctx, config)
	for _, diagObj := range diags {
		if diagObj.Severity == diag.Error {
			assert.Fail(t, diagObj.Summary)
		}
	}
	require.False(t, diags.HasError())

	providerCtx := provider.Meta().(*providerContext)
	kubeConfigPath := providerCtx.clusterClient.Config().KubeConfigPath

	kubeConfig, err := ioutil.ReadFile(kubeConfigPath)
	require.Nil(t, err)

	assert.Equal(
		t,
		strings.TrimSpace(`
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: dGVzdENsdXN0ZXJDQUNlcnRpZmljYXRl
    server: testHost
  name: kubeapply-cluster
contexts:
- context:
  name: default-context
  context:
    cluster: kubeapply-cluster
    user: terraform-user
current-context: default-context
users:
- name: terraform-user
  user:
    token: testToken
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: testCommand
      env:
      - name: "key1"
        value: "value1"
      - name: "key2"
        value: "value2"
      args:
      - "arg1"
      - "arg2"
	`),
		strings.TrimSpace(string(kubeConfig)),
	)
}

func TestProviderBasic(t *testing.T) {
	ctx := context.Background()
	provider := Provider(nil)

	err := provider.InternalValidate()
	require.NoError(t, err)

	testKey := base64.StdEncoding.EncodeToString([]byte("testKey"))
	testCertificate := base64.StdEncoding.EncodeToString([]byte("testCertificate"))

	config := terraform.NewResourceConfigRaw(
		map[string]interface{}{
			"host":               "testHost",
			"client_key":         testKey,
			"client_certificate": testCertificate,
			"insecure":           true,
			"username":           "testUserName",
			"password":           "testPassword",
		},
	)

	diags := provider.Configure(ctx, config)
	for _, diagObj := range diags {
		if diagObj.Severity == diag.Error {
			assert.Fail(t, diagObj.Summary)
		}
	}
	require.False(t, diags.HasError())

	providerCtx := provider.Meta().(*providerContext)
	kubeConfigPath := providerCtx.clusterClient.Config().KubeConfigPath

	kubeConfig, err := ioutil.ReadFile(kubeConfigPath)
	require.Nil(t, err)

	assert.Equal(
		t,
		strings.TrimSpace(`
apiVersion: v1
kind: Config
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: testHost
  name: kubeapply-cluster
contexts:
- context:
  name: default-context
  context:
    cluster: kubeapply-cluster
    user: terraform-user
current-context: default-context
users:
- name: terraform-user
  user:
    client-certificate-data: dGVzdENlcnRpZmljYXRl
    client-key-data: dGVzdEtleQ==
    username: testUserName
    password: testPassword
	`),
		strings.TrimSpace(string(kubeConfig)),
	)
}
