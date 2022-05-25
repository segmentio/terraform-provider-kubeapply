---
page_title: "kubeapply Provider"
subcategory: ""
description: |-

---

# kubeapply Provider

The kubeapply provider exposes an easy way to expand, diff, and apply templated YAML resources
in a Kubernetes cluster via Terraform. It's inspired by the
[kubeapply](https://github.com/segmentio/kubeapply) tool, which enables a similar flow but
without Terraform.

Unlike other Terraform-based solutions, this provider does not require:

1. Converting manifests from YAML to HCL
2. Importing existing resources into the Terraform state
3. Creating a separate Terraform resource for each Kubernetes resource

Instead, it exposes a high-level `profile` resource that operates on
arbitrary bundles of YAML or YAML templates. Profiles can be added for existing resources in the
cluster without doing any state imports and (optionally) can be removed from Terraform without
forcing the underlying resources to be deleted.

## Installation

### Requirements

The provider requires Terraform v0.13 or later. It will not work with older versions.

Also, the following tools need to be installed locally and in the `PATH` of whatever host is
running Terraform with the provider:

1. `kubectl` v1.19 or later
2. The `kadiff` utility defined in the
  [provider git repo](https://github.com/segmentio/terraform-provider-kubeapply) and installable
  via `make install-kadiff`. The latter is used for generating more structured Kubernetes diffs
  than the default diff command.

### Including in workspace

Once the above requirements are met, the provider can be included in a Terraform workspace
by adding it to the `required_providers` block, e.g.:

```hcl
terraform {
  required_version = ">= 0.13"

  required_providers {
    kubeapply = {
      source  = "segmentio/kubeapply"
      version = ">= 0.0.7"
    }
  }
}
```

Terraform will pull the provider from the registry when `terraform init` is run.

## Configuration

Each provider instance references a single Kubernetes cluster in which one or
more `profile` resources will be applied. The following shows an example of configuring
the provider for an EKS cluster named "my-cluster":

```hcl
# As an alternative to using these "data" resources, you can just get
# the parameters from an upstream resource or module.
data "aws_eks_cluster" "cluster" {
  name = "my-cluster"
}

data "aws_eks_cluster_auth" "cluster" {
  name = "my-cluster"
}

provider "kubeapply" {
  cluster_name = "my-cluster"

  # These are made available for templating; if they don't apply,
  # you can set them to empty strings.
  region       = "us-west-2"
  environment  = "development"
  account_name = "dev"
  account_id   = "1234567890"

  # Parameters to create or find kubeconfig.
  #
  # The exact things to set here depend on how you're handling cluster auth.
  # You can also just point the provider at an existing kubeconfig via the
  # 'config_path' parameter.
  host                   = data.aws_eks_cluster.cluster.endpoint
  cluster_ca_certificate = data.aws_eks_cluster.cluster.certificate_authority[0].data
  token                  = data.aws_eks_cluster_auth.cluster.token
}
```

As an alternative to the token-based authentication shown above, you can also
use a pre-defined kubeconfig, client certificates, a custom `exec` process, or a username/password
combination. The options here are adopted from the ones exposed by the Hashicorp Terraform
provider; see the [Schema](#schema) section below for more details.

### Resource deletions

If `allow_deletes` is set to `true` in the provider (which is the default), then the
provider will delete resources from the Kubernetes API if they're removed from the Terraform
state. Note that these deletions are best-effort and non-blocking; after applying a change
that does a deletion, you'll want to do some manual checking in the cluster to verify that
the resources are actually gone.

## How it works

On each `plan` run, the provider goes through the following steps:

1. Expand out all of the manifests according to the parameters in each `kubeapply_profile` resource
2. Run `kubectl diff` and insert the diffs into the resource so they can be seen in the `plan` outputs

When the plan is applied, the provider runs `kubectl apply` on the expanded outputs and cleans
the diffs out of the state.

## Schema

### Required

- `account_id` - (String) Account ID; used for templating only, set to an empty string if not applicable
- `account_name` - (String) Account name; used for templating only, set to an empty string if not applicable
- `cluster_name` - (String) Name of the cluster
- `environment` - (String) Account environment; used for templating only, set to an empty string if not applicable
- `region` - (String) Region; used for templating only, set to an empty string if not applicable

### Optional

- `allow_deletes` - (Boolean) Actually delete kubernetes resources when they're removed from terraform; defaults to `true`
- `auto_create_namespaces` - (Boolean) Automatically create namespaces before each diff; defaults to `true`
- `client_certificate` - (String) PEM-encoded client certificate for mTLS
- `client_key` - (String) PEM-encoded client key for mTLS
- `cluster_ca_certificate` - (String) PEM-encoded root certificates bundle for TLS authentication
- `cluster_version` - (String) Cluster Kubernetes version
- `config_path` - (String) Path to kubeconfig to use for cluster access
- `diff_context_lines` - (Number) Number of lines of context to show on diffs; defaults to 2
- `exec` - (Block List, Max: 1) (see [below for nested schema](#nestedblock--exec))
- `force_diffs` - (Boolean) Force diffs for all resources managed by this provider; defaults to `true`
- `host` - (String) The hostname (in form of URI) of Kubernetes master
- `insecure` - (Boolean) Skip TLS hostname verification
- `max_diff_line_length` - (Number) Max line length for all resources managed by this provider; defaults to 256
- `max_diff_size` - (Number) Max total diff size for all resources managed by this provider; defaults to 3000
- `password` - (String) Password for basic HTTP auth
- `token` - (String) Token to authenticate with the Kubernetes API
- `username` - (String) Username for basic HTTP auth
- `verbose_applies` - (Boolean) Generate verbose output for applies; defaults to `false`
- `verbose_diffs` = (Boolean) Generate verbose output for diffs; defaults to `true`

<a id="nestedblock--exec"></a>
### Nested Schema for `exec`

Required:

- `command` - (String) Command to run to get credentials

Optional:

- `api_version` - (String) API version, e.g. `client.authentication.k8s.io/v1beta1` __IMPORTANT__: For EKS, if you use `aws`CLI v1.24+ or 2.6.3+, you can leave this as the default (`v1beta1`). If you use `aws`CLI <=v1.23 or <2.6.3 , you will need to manually set this value to `client.authentication.k8s.io/v1alpha1` for versions of this provider after `0.0.12`. 
- `args` - (List of String) List of args to pass to command
- `env` - (Map of String) Environment variables to set
