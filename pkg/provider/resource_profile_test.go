package provider

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/segmentio/terraform-provider-kubeapply/pkg/cluster"
	"github.com/segmentio/terraform-provider-kubeapply/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestResourceProfile(t *testing.T) {
	ctx := context.Background()
	tempDir, err := ioutil.TempDir("", "kubeapply_test_profile_")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	clusterConfig := cluster.Config{
		Cluster:     "testCluster",
		Region:      "testRegion",
		Environment: "testEnvironment",
		AccountName: "testAccountName",
		AccountID:   "testAccountID",
	}
	clusterClient, err := cluster.NewFakeClient(
		ctx,
		&cluster.ClientConfig{
			Config: &clusterConfig,
		},
	)
	require.NoError(t, err)

	rawClient := fake.NewSimpleClientset(
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testNamespace",
			},
		},
	)

	sourceFetcher, err := newSourceFetcher(&commandLineGitClient{})
	require.NoError(t, err)

	providerCtx := &providerContext{
		autoCreateNamespaces: true,
		canRun:               true,
		clusterClient:        clusterClient,
		clusterConfig:        clusterConfig,
		keepExpanded:         true,
		rawClient:            rawClient,
		sourceFetcher:        sourceFetcher,
		tempDir:              tempDir,
	}

	resource.Test(
		t,
		resource.TestCase{
			IsUnitTest: true,
			Providers: map[string]*schema.Provider{
				"kubeapply": Provider(providerCtx),
			},
			Steps: []resource.TestStep{
				// First, do create
				{
					Config: `
provider "kubeapply" {
  cluster_name = "testCluster"
  cluster_version = "1.19"
  region = "testRegion"
  account_name = "testAccountName"
  account_id = "12345"
  environment = "testEnvironment"
  host = "testHost"
  cluster_ca_certificate = "testCACertificate"
  token = "testToken"
  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    args        = ["eks", "get-token", "--cluster-name", "testCluster"]
    command     = "aws"
  }
}

resource "kubeapply_profile" "main_profile" {
  source = "testdata/app1"

  parameters = {
    value1 = "Value1"
	value2 = "Value2"
	serviceAccount = ""
  }
  set {
    name = "keys"
	value = jsonencode(["a", "b"])
  }
}`,
					Check: func(state *terraform.State) error {
						require.Equal(t, 1, len(state.Modules))
						module := state.Modules[0]
						resource := module.Resources["kubeapply_profile.main_profile"]
						require.NotNil(t, resource)
						return nil
					},
				},
				// Then, do update that actually makes changes
				{
					Config: `
provider "kubeapply" {
  cluster_name = "testCluster"
  cluster_version = "1.19"
  region = "testRegion"
  account_name = "testAccountName"
  account_id = "12345"
  environment = "testEnvironment"
  host = "testHost"
  cluster_ca_certificate = "testCACertificate"
  token = "testToken"
  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    args        = ["eks", "get-token", "--cluster-name", "testCluster"]
    command     = "aws"
  }
}

resource "kubeapply_profile" "main_profile" {
  source = "testdata/app1"

  parameters = {
    value1 = "UpdatedValue1"
	value2 = "UpdatedValue2"
	serviceAccount = ""
  }
  set {
    name = "keys"
	value = jsonencode(["a", "b"])
  }
}`,
				},
				// Then, finally, update that doesn't make any changes
				{
					Config: `
provider "kubeapply" {
  cluster_name = "testCluster"
  cluster_version = "1.19"
  region = "testRegion"
  account_name = "testAccountName"
  account_id = "12345"
  environment = "testEnvironment"
  host = "testHost"
  cluster_ca_certificate = "testCACertificate"
  token = "testToken"
  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    args        = ["eks", "get-token", "--cluster-name", "testCluster"]
    command     = "aws"
  }
}

resource "kubeapply_profile" "main_profile" {
  source = "testdata/app1"

  parameters = {
    value1 = "UpdatedValue1"
	value2 = "UpdatedValue2"
	serviceAccount = ""
  }
  set {
    name = "keys"
	value = jsonencode(["a", "b"])
  }
}`,
				},
				// Test some error cases
				{
					Config: `
provider "kubeapply" {
  host = "testHost"
  cluster_ca_certificate = "testCACertificate"
  token = "testToken"
  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    args        = ["eks", "get-token", "--cluster-name", "testCluster"]
    command     = "aws"
  }
}

resource "kubeapply_profile" "main_profile" {
  source = "testdata/app1"

  parameters = {
    value1 = "UpdatedValue1"
	value2 = "UpdatedValue2"
	serviceAccount = ""
  }
  set {
    name = "keys"
	value = jsonencode(["a", "b"])
  }
}`,
					ExpectError: regexp.MustCompile("is required, but no definition was found"),
				},
				{
					Config: `
provider "kubeapply" {
  cluster_name = "testCluster"
  cluster_version = "1.19"
  region = "testRegion"
  account_name = "testAccountName"
  account_id = "12345"
  environment = "testEnvironment"
  host = "testHost"
  cluster_ca_certificate = "testCACertificate"
  token = "testToken"
  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    args        = ["eks", "get-token", "--cluster-name", "testCluster"]
    command     = "aws"
  }
}

resource "kubeapply_profile" "main_profile" {
  source = "bad dir"

  parameters = {
    value1 = "UpdatedValue1"
	value2 = "UpdatedValue2"
	serviceAccount = ""
  }
}`,
					ExpectError: regexp.MustCompile("stat bad dir: no such file or directory"),
				},
			},
		},
	)

	expandedRoot := filepath.Join(tempDir, "expanded")
	subDirs, err := ioutil.ReadDir(expandedRoot)
	require.NoError(t, err)
	require.Greater(t, len(subDirs), 0)

	// There are lots of expansions, just look at the last one
	lastSubDir := filepath.Join(
		expandedRoot,
		subDirs[len(subDirs)-1].Name(),
	)
	lastSubDirContents := util.GetContents(t, lastSubDir)
	assert.Equal(
		t,
		map[string][]string{
			"deployment.yaml": {
				"apiVersion: apps/v1",
				"kind: Deployment", "metadata:",
				"  labels:",
				"    key1: UpdatedValue1",
				"    cluster: testCluster",
				"    keya: valuea",
				"    keyb: valueb",
				"    configHash: 6ff076f70be152a9f55b62176b5530a1",
				"  name: testName",
				"  namespace: testNamespace",
				"",
			},
			"service.yaml": {
				"apiVersion: v1",
				"kind: Service", "metadata:",
				"  labels:",
				"    key2: UpdatedValue2",
				"    environment: testEnvironment",
				"    accountName: testAccountName",
				"    accountID: testAccountID",
				"  name: testName",
				"  namespace: testNamespace2",
				"",
			},
		},
		lastSubDirContents,
	)

	calls := clusterClient.(*cluster.FakeClient).Calls
	numApplies := 0
	numDeletes := 0

	for _, call := range calls {
		switch call.CallType {
		case "Apply":
			numApplies++
		case "Delete":
			numDeletes++
		default:
		}
	}

	// Have one apply for create, one for update
	assert.Equal(t, 2, numApplies)
	assert.Equal(t, 0, numDeletes)

	namespaces, err := rawClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	namespaceNames := []string{}
	for _, namespace := range namespaces.Items {
		namespaceNames = append(namespaceNames, namespace.Name)
	}
	assert.ElementsMatch(
		t,
		[]string{
			"testNamespace",
			// Extra namespace was created
			"testNamespace2",
		},
		namespaceNames,
	)
}

func TestResourceProfileDeletesAllowed(t *testing.T) {
	ctx := context.Background()
	tempDir, err := ioutil.TempDir("", "kubeapply_test_profile_")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	clusterConfig := cluster.Config{
		Cluster:     "testCluster",
		Region:      "testRegion",
		Environment: "testEnvironment",
		AccountName: "testAccountName",
		AccountID:   "testAccountID",
	}
	clusterClient, err := cluster.NewFakeClient(
		ctx,
		&cluster.ClientConfig{
			Config: &clusterConfig,
		},
	)
	require.NoError(t, err)

	rawClient := fake.NewSimpleClientset(
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testNamespace",
			},
		},
	)

	sourceFetcher, err := newSourceFetcher(&commandLineGitClient{})
	require.NoError(t, err)

	providerCtx := &providerContext{
		allowDeletes:         true,
		autoCreateNamespaces: true,
		canRun:               true,
		clusterClient:        clusterClient,
		clusterConfig:        clusterConfig,
		rawClient:            rawClient,
		sourceFetcher:        sourceFetcher,
		tempDir:              tempDir,
	}

	resource.Test(
		t,
		resource.TestCase{
			IsUnitTest: true,
			Providers: map[string]*schema.Provider{
				"kubeapply": Provider(providerCtx),
			},
			Steps: []resource.TestStep{
				// First, do create
				{
					Config: `
provider "kubeapply" {
  cluster_name = "testCluster"
  region = "testRegion"
  account_name = "testAccountName"
  account_id = "12345"
  environment = "testEnvironment"
  host = "testHost"
}

resource "kubeapply_profile" "main_profile" {
  source = "testdata/app1"

  parameters = {
    value1 = "Value1"
	value2 = "Value2"
	serviceAccount = "testAccount"
  }
  set {
    name = "keys"
	value = jsonencode(["a", "b"])
  }
}`,
					Check: func(state *terraform.State) error {
						require.Equal(t, 1, len(state.Modules))
						module := state.Modules[0]
						resource := module.Resources["kubeapply_profile.main_profile"]
						require.NotNil(t, resource)
						return nil
					},
				},
				{
					// Then remove just the service account
					Config: `
provider "kubeapply" {
  cluster_name = "testCluster"
  region = "testRegion"
  account_name = "testAccountName"
  account_id = "12345"
  environment = "testEnvironment"
  host = "testHost"
}

resource "kubeapply_profile" "main_profile" {
  source = "testdata/app1"

  parameters = {
    value1 = "Value1"
	value2 = "Value2"
	serviceAccount = ""
  }
  set {
    name = "keys"
	value = jsonencode(["a", "b"])
  }
}`,
				},
				// Then, do full delete
				{
					Config: `
provider "kubeapply" {
  cluster_name = "testCluster"
  region = "testRegion"
  account_name = "testAccountName"
  account_id = "12345"
  environment = "testEnvironment"
  host = "testHost"
}`,
				},
			},
		},
	)

	calls := clusterClient.(*cluster.FakeClient).Calls
	deletions := [][]string{}
	for _, call := range calls {
		if call.CallType == "Delete" {
			sort.Slice(call.Paths, func(a, b int) bool {
				return call.Paths[a] < call.Paths[b]
			})
			deletions = append(deletions, call.Paths)
		}
	}

	assert.Equal(
		t,
		[][]string{
			{
				// First deletion just removes the service account
				"v1.ServiceAccount.testNamespace2.testAccount",
			},
			{
				// Second deletion removes everything else
				"apps/v1.Deployment.testNamespace.testName",
				"v1.Service.testNamespace2.testName",
			},
		},
		deletions,
	)
}

func TestResourceProfileDeletesDisabled(t *testing.T) {
	ctx := context.Background()
	tempDir, err := ioutil.TempDir("", "kubeapply_test_profile_")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	clusterConfig := cluster.Config{
		Cluster:     "testCluster",
		Region:      "testRegion",
		Environment: "testEnvironment",
		AccountName: "testAccountName",
		AccountID:   "testAccountID",
	}
	clusterClient, err := cluster.NewFakeClient(
		ctx,
		&cluster.ClientConfig{
			Config: &clusterConfig,
		},
	)
	require.NoError(t, err)

	rawClient := fake.NewSimpleClientset(
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testNamespace",
			},
		},
	)

	sourceFetcher, err := newSourceFetcher(&commandLineGitClient{})
	require.NoError(t, err)

	providerCtx := &providerContext{
		allowDeletes:         false,
		autoCreateNamespaces: true,
		canRun:               true,
		clusterClient:        clusterClient,
		clusterConfig:        clusterConfig,
		rawClient:            rawClient,
		sourceFetcher:        sourceFetcher,
		tempDir:              tempDir,
	}

	resource.Test(
		t,
		resource.TestCase{
			IsUnitTest: true,
			Providers: map[string]*schema.Provider{
				"kubeapply": Provider(providerCtx),
			},
			Steps: []resource.TestStep{
				// First, do create
				{
					Config: `
provider "kubeapply" {
  cluster_name = "testCluster"
  region = "testRegion"
  account_name = "testAccountName"
  account_id = "12345"
  environment = "testEnvironment"
  host = "testHost"
}

resource "kubeapply_profile" "main_profile" {
  source = "testdata/app1"

  parameters = {
    value1 = "Value1"
	value2 = "Value2"
	serviceAccount = "testAccount"
  }
  set {
    name = "keys"
	value = jsonencode(["a", "b"])
  }
}`,
					Check: func(state *terraform.State) error {
						require.Equal(t, 1, len(state.Modules))
						module := state.Modules[0]
						resource := module.Resources["kubeapply_profile.main_profile"]
						require.NotNil(t, resource)
						return nil
					},
				},
				{
					// Then remove just the service account
					Config: `
provider "kubeapply" {
  cluster_name = "testCluster"
  region = "testRegion"
  account_name = "testAccountName"
  account_id = "12345"
  environment = "testEnvironment"
  host = "testHost"
}

resource "kubeapply_profile" "main_profile" {
  source = "testdata/app1"

  parameters = {
    value1 = "Value1"
	value2 = "Value2"
	serviceAccount = ""
  }
  set {
    name = "keys"
	value = jsonencode(["a", "b"])
  }
}`,
				},
				// Then, do full delete
				{
					Config: `
provider "kubeapply" {
  cluster_name = "testCluster"
  region = "testRegion"
  account_name = "testAccountName"
  account_id = "12345"
  environment = "testEnvironment"
  host = "testHost"
}`,
				},
			},
		},
	)

	calls := clusterClient.(*cluster.FakeClient).Calls
	deletions := [][]string{}
	for _, call := range calls {
		if call.CallType == "Delete" {
			deletions = append(deletions, call.Paths)
		}
	}
	assert.Equal(
		t,
		// No deletions actually done
		[][]string{},
		deletions,
	)
}

func TestResourceProfileCanNotRun(t *testing.T) {
	ctx := context.Background()
	tempDir, err := ioutil.TempDir("", "kubeapply_test_profile_")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	clusterConfig := cluster.Config{
		Cluster:     "testCluster",
		Region:      "testRegion",
		Environment: "testEnvironment",
		AccountName: "testAccountName",
		AccountID:   "testAccountID",
	}
	clusterClient, err := cluster.NewFakeClient(
		ctx,
		&cluster.ClientConfig{
			Config: &clusterConfig,
		},
	)
	require.NoError(t, err)

	rawClient := fake.NewSimpleClientset(
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testNamespace",
			},
		},
	)

	sourceFetcher, err := newSourceFetcher(&commandLineGitClient{})
	require.NoError(t, err)

	providerCtx := &providerContext{
		allowDeletes:         false,
		autoCreateNamespaces: true,
		canRun:               false,
		clusterClient:        clusterClient,
		clusterConfig:        clusterConfig,
		rawClient:            rawClient,
		sourceFetcher:        sourceFetcher,
		tempDir:              tempDir,
	}

	resource.Test(
		t,
		resource.TestCase{
			IsUnitTest: true,
			Providers: map[string]*schema.Provider{
				"kubeapply": Provider(providerCtx),
			},
			Steps: []resource.TestStep{
				{
					Config: `
provider "kubeapply" {
  cluster_name = "testCluster"
  region = "testRegion"
  account_name = "testAccountName"
  account_id = "12345"
  environment = "testEnvironment"
  host = ""
}

resource "kubeapply_profile" "main_profile" {
  source = "testdata/app1"

  parameters = {
    value1 = "Value1"
	value2 = "Value2"
	serviceAccount = "testAccount"
  }
  set {
    name = "keys"
	value = jsonencode(["a", "b"])
  }
}`,
					ExpectError: regexp.MustCompile("provider is missing a host"),
				},
			},
		},
	)
}

func TestDiffs(t *testing.T) {
	ctx := context.Background()
	tempDir, err := ioutil.TempDir("", "kubeapply_test_profile_")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	clusterConfig := cluster.Config{
		Cluster:     "testCluster",
		Region:      "testRegion",
		Environment: "testEnvironment",
		AccountName: "testAccountName",
		AccountID:   "testAccountID",
	}
	clusterClient, err := cluster.NewFakeClient(
		ctx,
		&cluster.ClientConfig{
			Config: &clusterConfig,
		},
	)
	require.NoError(t, err)

	rawClient := fake.NewSimpleClientset(
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testNamespace",
			},
		},
	)

	sourceFetcher, err := newSourceFetcher(&commandLineGitClient{})
	require.NoError(t, err)

	providerCtx := &providerContext{
		allowDeletes:         false,
		autoCreateNamespaces: true,
		canRun:               true,
		clusterClient:        clusterClient,
		clusterConfig:        clusterConfig,
		rawClient:            rawClient,
		sourceFetcher:        sourceFetcher,
		tempDir:              tempDir,
	}

	type testCase struct {
		description                   string
		data                          resourceDiffChangerSetter
		allowDeletes                  bool
		canRun                        bool
		forceDiffs                    bool
		noDiffs                       bool
		expectedDiff                  map[string]interface{}
		expectedExpandedFiles         map[string]interface{}
		expectedResources             map[string]interface{}
		expectedResourcesHash         string
		expectedResourcesComputed     bool
		expectedResourcesHashComputed bool
	}

	testCases := []testCase{
		{
			description:  "no changes",
			allowDeletes: false,
			canRun:       true,
			forceDiffs:   false,
			data: &fakeDiffChangerSetter{
				newComputed: map[string]struct{}{},
				oldValues: map[string]interface{}{
					"show_expanded": false,
					"no_diff":       false,
					"diff":          map[string]string{},
					"parameters": map[string]interface{}{
						"serviceAccount": "testServiceAccount",
						"value2":         "test2",
					},
					"resources": map[string]interface{}{
						"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
						"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
					},
					"resources_hash": "4e02e6b6e828fc728bb440b2c1dc518b",
					"set":            &schema.Set{},
					"source":         "testdata/app2",
				},
				newValues: map[string]interface{}{
					"show_expanded": false,
					"no_diff":       false,
					"diff":          map[string]string{},
					"parameters": map[string]interface{}{
						"serviceAccount": "testServiceAccount",
						"value2":         "test2",
					},
					"resources": map[string]interface{}{
						"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
						"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
					},
					"resources_hash": "4e02e6b6e828fc728bb440b2c1dc518b",
					"set":            &schema.Set{},
					"source":         "testdata/app2",
				},
			},
			expectedDiff:          map[string]interface{}{},
			expectedExpandedFiles: map[string]interface{}{},
			expectedResources: map[string]interface{}{
				"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
				"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
			},
			expectedResourcesHash: "4e02e6b6e828fc728bb440b2c1dc518b",
		},
		{
			description:  "show_expanded set to true",
			allowDeletes: false,
			canRun:       true,
			forceDiffs:   false,
			data: &fakeDiffChangerSetter{
				newComputed: map[string]struct{}{},
				oldValues: map[string]interface{}{
					"show_expanded": true,
					"no_diff":       false,
					"diff":          map[string]string{},
					"parameters": map[string]interface{}{
						"serviceAccount": "testServiceAccount",
						"value2":         "test2",
					},
					"resources": map[string]interface{}{
						"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
						"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
					},
					"resources_hash": "4e02e6b6e828fc728bb440b2c1dc518b",
					"set":            &schema.Set{},
					"source":         "testdata/app2",
				},
				newValues: map[string]interface{}{
					"show_expanded": true,
					"no_diff":       false,
					"diff":          map[string]string{},
					"parameters": map[string]interface{}{
						"serviceAccount": "testServiceAccount",
						"value2":         "test2",
					},
					"resources": map[string]interface{}{
						"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
						"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
					},
					"resources_hash": "4e02e6b6e828fc728bb440b2c1dc518b",
					"set":            &schema.Set{},
					"source":         "testdata/app2",
				},
			},
			expectedDiff: map[string]interface{}{},
			expectedExpandedFiles: map[string]interface{}{
				"service.yaml": `apiVersion: v1
kind: Service
metadata:
  labels:
    key2: test2
    environment: testEnvironment
    accountName: testAccountName
    accountID: testAccountID
  name: testName
  namespace: testNamespace2
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: testServiceAccount
  namespace: testNamespace2
`,
			},
			expectedResources: map[string]interface{}{
				"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
				"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
			},
			expectedResourcesHash: "4e02e6b6e828fc728bb440b2c1dc518b",
		},
		{
			description:  "no changes with forced diffs",
			allowDeletes: false,
			canRun:       true,
			forceDiffs:   true,
			data: &fakeDiffChangerSetter{
				newComputed: map[string]struct{}{},
				oldValues: map[string]interface{}{
					"show_expanded": false,
					"no_diff":       false,
					"diff":          map[string]string{},
					"parameters": map[string]interface{}{
						"serviceAccount": "testServiceAccount",
						"value2":         "test2",
					},
					"resources": map[string]interface{}{
						"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
						"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
					},
					"resources_hash": "4e02e6b6e828fc728bb440b2c1dc518b",
					"set":            &schema.Set{},
					"source":         "testdata/app2",
				},
				newValues: map[string]interface{}{
					"show_expanded": false,
					"no_diff":       false,
					"diff":          map[string]string{},
					"parameters": map[string]interface{}{
						"serviceAccount": "testServiceAccount",
						"value2":         "test2",
					},
					"resources": map[string]interface{}{
						"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
						"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
					},
					"resources_hash": "4e02e6b6e828fc728bb440b2c1dc518b",
					"set":            &schema.Set{},
					"source":         "testdata/app2",
				},
			},
			expectedDiff: map[string]interface{}{
				"result": "structured diff result for testCluster",
			},
			expectedExpandedFiles: map[string]interface{}{},
			expectedResources: map[string]interface{}{
				"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
				"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
			},
			expectedResourcesHash: "4e02e6b6e828fc728bb440b2c1dc518b",
		},
		{
			description:  "simple static change",
			allowDeletes: false,
			canRun:       true,
			forceDiffs:   false,
			data: &fakeDiffChangerSetter{
				newComputed: map[string]struct{}{},
				oldValues: map[string]interface{}{
					"show_expanded": false,
					"no_diff":       false,
					"diff":          map[string]string{},
					"parameters": map[string]interface{}{
						"serviceAccount": "testServiceAccount",
						"value2":         "test2",
					},
					"resources": map[string]interface{}{
						"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
						"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
					},
					"resources_hash": "4e02e6b6e828fc728bb440b2c1dc518b",
					"set":            &schema.Set{},
					"source":         "testdata/app2",
				},
				newValues: map[string]interface{}{
					"show_expanded": false,
					"no_diff":       false,
					"diff":          map[string]string{},
					"parameters": map[string]interface{}{
						"serviceAccount": "testServiceAccount",
						"value2":         "new test2",
					},
					"resources": map[string]interface{}{
						"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
						"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
					},
					"resources_hash": "b645c4392b2eeb4020a7be44bb3c3c1d",
					"set":            &schema.Set{},
					"source":         "testdata/app2",
				},
			},
			expectedDiff: map[string]interface{}{
				"result": "structured diff result for testCluster",
			},
			expectedExpandedFiles: map[string]interface{}{},
			expectedResources: map[string]interface{}{
				"v1.Service.testNamespace2.testName":                  "8095dc3f74b345c1477271bbcbdd907f",
				"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
			},
			expectedResourcesHash: "b645c4392b2eeb4020a7be44bb3c3c1d",
		},
		{
			description:  "simple static change with no_diff set to true",
			allowDeletes: false,
			canRun:       true,
			forceDiffs:   false,
			data: &fakeDiffChangerSetter{
				newComputed: map[string]struct{}{},
				oldValues: map[string]interface{}{
					"show_expanded": false,
					"no_diff":       true,
					"diff":          map[string]string{},
					"parameters": map[string]interface{}{
						"serviceAccount": "testServiceAccount",
						"value2":         "test2",
					},
					"resources": map[string]interface{}{
						"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
						"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
					},
					"resources_hash": "4e02e6b6e828fc728bb440b2c1dc518b",
					"set":            &schema.Set{},
					"source":         "testdata/app2",
				},
				newValues: map[string]interface{}{
					"show_expanded": false,
					"no_diff":       true,
					"diff":          map[string]string{},
					"parameters": map[string]interface{}{
						"serviceAccount": "testServiceAccount",
						"value2":         "new test2",
					},
					"resources": map[string]interface{}{
						"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
						"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
					},
					"resources_hash": "b645c4392b2eeb4020a7be44bb3c3c1d",
					"set":            &schema.Set{},
					"source":         "testdata/app2",
				},
			},
			expectedDiff:          map[string]interface{}{},
			expectedExpandedFiles: map[string]interface{}{},
			expectedResources: map[string]interface{}{
				"v1.Service.testNamespace2.testName":                  "8095dc3f74b345c1477271bbcbdd907f",
				"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
			},
			expectedResourcesHash: "b645c4392b2eeb4020a7be44bb3c3c1d",
		},
		{
			description:  "can't run",
			allowDeletes: false,
			canRun:       false,
			forceDiffs:   false,
			data: &fakeDiffChangerSetter{
				newComputed: map[string]struct{}{},
				oldValues: map[string]interface{}{
					"show_expanded": false,
					"no_diff":       false,
					"diff":          map[string]string{},
					"parameters": map[string]interface{}{
						"serviceAccount": "testServiceAccount",
						"value2":         "test2",
					},
					"resources": map[string]interface{}{
						"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
						"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
					},
					"resources_hash": "4e02e6b6e828fc728bb440b2c1dc518b",
					"set":            &schema.Set{},
					"source":         "testdata/app2",
				},
				newValues: map[string]interface{}{
					"show_expanded": false,
					"no_diff":       false,
					"diff":          map[string]string{},
					"parameters": map[string]interface{}{
						"serviceAccount": "testServiceAccount",
						"value2":         "new test2",
					},
					"resources": map[string]interface{}{
						"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
						"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
					},
					"resources_hash": "4e02e6b6e828fc728bb440b2c1dc518b",
					"set":            &schema.Set{},
					"source":         "testdata/app2",
				},
			},
			expectedDiff: map[string]interface{}{
				"": "DIFFS UNKNOWN because provider missing host or kubeconfig",
			},
			expectedExpandedFiles: map[string]interface{}{},
			expectedResources: map[string]interface{}{
				// No expansion done
				"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
				"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
			},
			expectedResourcesHash:         "4e02e6b6e828fc728bb440b2c1dc518b",
			expectedResourcesComputed:     true,
			expectedResourcesHashComputed: true,
		},
		{
			description:  "parameters missing due to depending on upstream evaluation",
			allowDeletes: false,
			canRun:       true,
			forceDiffs:   false,
			data: &fakeDiffChangerSetter{
				newComputed: map[string]struct{}{},
				oldValues: map[string]interface{}{
					"show_expanded": false,
					"no_diff":       false,
					"diff":          map[string]string{},
					"parameters": map[string]interface{}{
						"serviceAccount": "testServiceAccount",
						"value2":         "test2",
					},
					"resources": map[string]interface{}{
						"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
						"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
					},
					"resources_hash": "4e02e6b6e828fc728bb440b2c1dc518b",
					"set":            &schema.Set{},
					"source":         "testdata/app2",
				},
				newValues: map[string]interface{}{
					"show_expanded": false,
					"no_diff":       false,
					"diff":          map[string]string{},
					"parameters":    map[string]interface{}{},
					"resources": map[string]interface{}{
						"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
						"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
					},
					"resources_hash": "4e02e6b6e828fc728bb440b2c1dc518b",
					"set":            &schema.Set{},
					"source":         "testdata/app2",
				},
			},
			expectedDiff: map[string]interface{}{
				"": "DIFFS UNKNOWN because one or more parameters are unknown",
			},
			expectedExpandedFiles: map[string]interface{}{},
			expectedResources: map[string]interface{}{
				// No expansion done
				"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
				"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
			},
			expectedResourcesHash:         "4e02e6b6e828fc728bb440b2c1dc518b",
			expectedResourcesComputed:     true,
			expectedResourcesHashComputed: true,
		},
		{
			description:  "set values missing due to depending on upstream evaluation",
			allowDeletes: false,
			canRun:       true,
			forceDiffs:   true,
			data: &fakeDiffChangerSetter{
				newComputed: map[string]struct{}{},
				oldValues: map[string]interface{}{
					"show_expanded": false,
					"no_diff":       false,
					"diff":          map[string]string{},
					"parameters": map[string]interface{}{
						"serviceAccount": "testServiceAccount",
						"value2":         "test2",
					},
					"resources": map[string]interface{}{
						"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
						"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
					},
					"resources_hash": "4e02e6b6e828fc728bb440b2c1dc518b",
					"set": createSet([]map[string]interface{}{
						{
							"name":        "testName1",
							"value":       `"testValue1"`,
							"placeholder": `""`,
						},
						{
							"name":        "testName2",
							"value":       `"testValue2"`,
							"placeholder": `""`,
						},
					}),
					"source": "testdata/app2",
				},
				newValues: map[string]interface{}{
					"show_expanded": false,
					"no_diff":       false,
					"diff":          map[string]string{},
					"parameters": map[string]interface{}{
						"serviceAccount": "testServiceAccount",
						"value2":         "test2Updated",
					},
					"resources": map[string]interface{}{
						"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
						"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
					},
					"resources_hash": "4e02e6b6e828fc728bb440b2c1dc518b",
					"set": createSet([]map[string]interface{}{
						{
							"name":        "testName1",
							"value":       `"testValue1"`,
							"placeholder": `""`,
						},
						{
							"name":        "testName2",
							"value":       unknownValue,
							"placeholder": `""`,
						},
					}),
					"source": "testdata/app2",
				},
			},
			expectedDiff: map[string]interface{}{
				// Diff result generated
				"result": "structured diff result for testCluster",
			},
			expectedExpandedFiles: map[string]interface{}{},
			expectedResources: map[string]interface{}{
				// Expansion done, but resources not updated even though parameters have changed
				"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
				"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
			},
			expectedResourcesHash:         "4e02e6b6e828fc728bb440b2c1dc518b",
			expectedResourcesComputed:     true,
			expectedResourcesHashComputed: true,
		},
		{
			description:  "removing resource with allow_deletes false",
			allowDeletes: false,
			canRun:       true,
			forceDiffs:   false,
			data: &fakeDiffChangerSetter{
				newComputed: map[string]struct{}{},
				oldValues: map[string]interface{}{
					"show_expanded": false,
					"no_diff":       false,
					"diff":          map[string]string{},
					"parameters": map[string]interface{}{
						"serviceAccount": "testServiceAccount",
						"value2":         "test2",
					},
					"set": &schema.Set{},
					"resources": map[string]interface{}{
						"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
						"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
					},
					"resources_hash": "4e02e6b6e828fc728bb440b2c1dc518b",
					"source":         "testdata/app2",
				},
				newValues: map[string]interface{}{
					"show_expanded": false,
					"no_diff":       false,
					"diff":          map[string]string{},
					"parameters": map[string]interface{}{
						"serviceAccount": "",
						"value2":         "test2",
					},
					"set": &schema.Set{},
					"resources": map[string]interface{}{
						"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
						"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
					},
					"resources_hash": "4e02e6b6e828fc728bb440b2c1dc518b",
					"source":         "testdata/app2",
				},
			},
			expectedDiff: map[string]interface{}{
				// Diff result generated
				"result": "structured diff result for testCluster",
				"v1.ServiceAccount.testNamespace2.testServiceAccount": "TO BE REMOVED from Terraform but will not be deleted from cluster due to value of allow_deletes.\nPlease delete manually.",
			},
			expectedExpandedFiles: map[string]interface{}{},
			expectedResources: map[string]interface{}{
				"v1.Service.testNamespace2.testName": "c81b9e717544afb0556f57c002ee6f60",
			},
			expectedResourcesHash: "d2d8681bf8a7a87ff2e5dedd3e3f1bfb",
		},
		{
			description:  "removing resource with allow_deletes true",
			allowDeletes: true,
			canRun:       true,
			forceDiffs:   false,
			data: &fakeDiffChangerSetter{
				newComputed: map[string]struct{}{},
				oldValues: map[string]interface{}{
					"show_expanded": false,
					"no_diff":       false,
					"diff":          map[string]string{},
					"parameters": map[string]interface{}{
						"serviceAccount": "testServiceAccount",
						"value2":         "test2",
					},
					"set": &schema.Set{},
					"resources": map[string]interface{}{
						"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
						"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
					},
					"resources_hash": "4e02e6b6e828fc728bb440b2c1dc518b",
					"source":         "testdata/app2",
				},
				newValues: map[string]interface{}{
					"show_expanded": false,
					"no_diff":       false,
					"diff":          map[string]string{},
					"parameters": map[string]interface{}{
						"serviceAccount": "",
						"value2":         "test2",
					},
					"set": &schema.Set{},
					"resources": map[string]interface{}{
						"v1.Service.testNamespace2.testName":                  "c81b9e717544afb0556f57c002ee6f60",
						"v1.ServiceAccount.testNamespace2.testServiceAccount": "9a754595e5b2796e3fa641d1078d47e9",
					},
					"resources_hash": "4e02e6b6e828fc728bb440b2c1dc518b",
					"source":         "testdata/app2",
				},
			},
			expectedDiff: map[string]interface{}{
				// Diff result generated
				"result": "structured diff result for testCluster",
				"v1.ServiceAccount.testNamespace2.testServiceAccount": "TO BE DELETED",
			},
			expectedExpandedFiles: map[string]interface{}{},
			expectedResources: map[string]interface{}{
				"v1.Service.testNamespace2.testName": "c81b9e717544afb0556f57c002ee6f60",
			},
			expectedResourcesHash: "d2d8681bf8a7a87ff2e5dedd3e3f1bfb",
		},
	}

	for _, testCase := range testCases {
		providerCtx.allowDeletes = testCase.allowDeletes
		providerCtx.canRun = testCase.canRun
		providerCtx.forceDiffs = testCase.forceDiffs
		providerCtx.clusterClient.(*cluster.FakeClient).NoDiffs = testCase.noDiffs

		err = resourceProfileCustomDiff(ctx, testCase.data, providerCtx)
		require.NoError(t, err, testCase.description)
		assert.Equal(
			t,
			testCase.expectedDiff,
			testCase.data.Get("diff"),
			testCase.description,
		)
		assert.Equal(
			t,
			testCase.expectedExpandedFiles,
			testCase.data.Get("expanded_files"),
			testCase.description,
		)
		assert.Equal(
			t,
			testCase.expectedResources,
			testCase.data.Get("resources"),
			testCase.description,
		)
		assert.Equal(
			t,
			testCase.expectedResourcesHash,
			testCase.data.Get("resources_hash"),
			testCase.description,
		)
		_, computed := testCase.data.(*fakeDiffChangerSetter).newComputed["resources"]
		assert.Equal(t, testCase.expectedResourcesComputed, computed, testCase.description)
		_, computed = testCase.data.(*fakeDiffChangerSetter).newComputed["resources_hash"]
		assert.Equal(t, testCase.expectedResourcesHashComputed, computed, testCase.description)
	}
}

func createSet(values []map[string]interface{}) *schema.Set {
	setValues := []interface{}{}
	for _, value := range values {
		setValues = append(setValues, value)
	}

	return schema.NewSet(
		func(v interface{}) int {
			hash := sha1.New()
			io.WriteString(hash, fmt.Sprintf("%+v", v))
			sum := hash.Sum(nil)
			return int(binary.BigEndian.Uint32(sum[0:4]))
		},
		setValues,
	)
}
