package provider

import (
	"context"
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/segmentio/terraform-provider-kubeapply/pkg/cluster"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Provider is the entrypoint for creating a new kubeapply terraform provider instance.
func Provider(providerCtx *providerContext) *schema.Provider {
	return &schema.Provider{
		ConfigureContextFunc: func(
			ctx context.Context,
			data *schema.ResourceData,
		) (interface{}, diag.Diagnostics) {
			// Use a provider context that's injected in for testing
			// purposes.
			if providerCtx != nil {
				return providerCtx, diag.Diagnostics{}
			}
			return providerConfigure(ctx, data)
		},
		Schema: map[string]*schema.Schema{
			// Basic info about the cluster
			"cluster_name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the cluster",
			},
			"cluster_version": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "Cluster Kubernetes version",
			},
			"region": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Region",
			},
			"environment": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Account environment",
			},
			"account_name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Account name",
			},
			"account_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Account ID",
			},

			// Cluster API and auth information
			"client_certificate": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "PEM-encoded client certificate for mTLS",
			},
			"client_key": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "PEM-encoded client key for mTLS",
			},
			"cluster_ca_certificate": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "PEM-encoded root certificates bundle for TLS authentication.",
			},
			"config_path": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Path to kubeconfig to use for cluster access",
			},
			"exec": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"api_version": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "client.authentication.k8s.io/v1beta1",
						},
						"command": {
							Type:     schema.TypeString,
							Required: true,
						},
						"env": {
							Type:     schema.TypeMap,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"args": {
							Type:     schema.TypeList,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
				Description: "",
			},
			"host": {
				Type:         schema.TypeString,
				Optional:     true,
				Description:  "The hostname (in form of URI) of Kubernetes master",
				AtLeastOneOf: []string{"host", "config_path"},
			},
			"insecure": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Skip TLS hostname verification",
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Password for basic HTTP auth",
			},
			"token": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Token to authenticate with the Kubernetes API",
			},
			"username": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Username for basic HTTP auth",
			},

			// Optional behavior settings
			"auto_create_namespaces": {
				Type:        schema.TypeBool,
				Description: "Automatically create namespaces before each diff",
				Default:     true,
				Optional:    true,
			},
			"allow_deletes": {
				Type:        schema.TypeBool,
				Description: "Actually delete kubernetes resources when they're removed from terraform",
				Default:     true,
				Optional:    true,
			},
			"diff_context_lines": {
				Type:        schema.TypeInt,
				Description: "Number of lines of context to show on diffs",
				Default:     2,
				Optional:    true,
			},
			"force_diffs": {
				Type:        schema.TypeBool,
				Description: "Force diffs for all resources managed by this provider",
				Default:     true,
				Optional:    true,
			},
			"max_diff_line_length": {
				Type:        schema.TypeInt,
				Description: "Max line length for all resources managed by this provider",
				Default:     256,
				Optional:    true,
			},
			"max_diff_size": {
				Type:        schema.TypeInt,
				Description: "Max total diff size for all resources managed by this provider",
				Default:     3000,
				Optional:    true,
			},
			"verbose_applies": {
				Type:        schema.TypeBool,
				Description: "Generate verbose output for applies",
				Default:     false,
				Optional:    true,
			},
			"verbose_diffs": {
				Type:        schema.TypeBool,
				Description: "Generate verbose output for diffs",
				Default:     true,
				Optional:    true,
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"kubeapply_profile": profileResource(),
		},
		DataSourcesMap: map[string]*schema.Resource{},
	}
}

func providerConfigure(
	ctx context.Context,
	data resourceChanger,
) (interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	now := time.Now()
	pid := os.Getpid()

	log.Infof("Creating provider for cluster %s in region %s with host %s",
		data.Get("cluster_name").(string),
		data.Get("region").(string),
		data.Get("host").(string),
	)

	clusterConfig := cluster.Config{
		Cluster:     data.Get("cluster_name").(string),
		Region:      data.Get("region").(string),
		AccountName: data.Get("account_name").(string),
		AccountID:   data.Get("account_id").(string),
		Environment: data.Get("environment").(string),
		Version:     data.Get("cluster_version").(string),
	}

	tempDir, err := ioutil.TempDir("", "kubeapply_kubeconfig_")
	if err != nil {
		return nil, diag.FromErr(err)
	}

	kubeConfigPath, err := createKubeConfig(data, tempDir)
	if err != nil {
		return nil, diag.FromErr(err)
	}

	sourceFetcher, err := newSourceFetcher(&commandLineGitClient{})
	if err != nil {
		return nil, diag.FromErr(err)
	}

	log.Infof("Setting provider kubeconfig path to %s", kubeConfigPath)
	clusterConfig.KubeConfigPath = kubeConfigPath

	// We require at least a host or a kubeconfig to run
	canRun := data.Get("host").(string) != "" || data.Get("config_path") != ""

	var clusterClient cluster.Client
	var rawClient *kubernetes.Clientset

	if canRun {
		log.Info("Creating cluster client")
		clusterClient, err = cluster.NewKubeClient(
			ctx,
			&cluster.ClientConfig{
				Config: &clusterConfig,
				// Add extra environment variables that will be used by kadiff to configure diff
				// outputs
				ExtraEnv: []string{
					fmt.Sprintf("KADIFF_CONTEXT_LINES=%d", data.Get("diff_context_lines").(int)),
					fmt.Sprintf("KADIFF_MAX_SIZE=%d", data.Get("max_diff_size").(int)),
					fmt.Sprintf("KADIFF_MAX_LINE_LENGTH=%d", data.Get("max_diff_line_length").(int)),
				},
			},
		)
		if err != nil {
			return nil, diag.FromErr(err)
		}

		log.Info("Creating raw kube client")
		kubeClientConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			return nil, diag.FromErr(err)
		}

		rawClient, err = kubernetes.NewForConfig(kubeClientConfig)
		if err != nil {
			return nil, diag.FromErr(err)
		}
	}

	providerCtx := providerContext{
		allowDeletes:         data.Get("allow_deletes").(bool),
		autoCreateNamespaces: data.Get("auto_create_namespaces").(bool),
		canRun:               canRun,
		clusterConfig:        clusterConfig,
		clusterClient:        clusterClient,
		createdAt:            now,
		forceDiffs:           data.Get("force_diffs").(bool),
		pid:                  pid,
		rawClient:            rawClient,
		sourceFetcher:        sourceFetcher,
		tempDir:              tempDir,
		verboseApplies:       data.Get("verbose_applies").(bool),
		verboseDiffs:         data.Get("verbose_diffs").(bool),
	}

	return &providerCtx, diags
}
