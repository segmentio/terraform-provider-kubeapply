# terraform-provider-kubeapply

This repo contains a Terraform-provider-based version of
[`kubeapply`](https://github.com/segmentio/kubeapply). It supports expanding, diffing,
and applying arbitrary, templated YAML manifests in a Kubernetes cluster via
[Terraform](https://www.terraform.io/).

## Motivation

After using `kubeapply` for a year inside Segment, we decided to support the same flow
via [Terraform](https://www.terraform.io/), which we use for configuring resources in our AWS
accounts.

Although there's an existing [Kubernetes provider for Terraform](https://registry.terraform.io/providers/hashicorp/kubernetes/latest), we've found that it has a number of limitations:

1. Kubernetes configs need to be converted from YAML to HCL, which can be tedious
2. Existing resources need to be explicitly imported into the Terraform state
3. Custom resources are not yet supported in the main provider (but are available in an alpha version)

As an alternative, we've created a provider that supports lightweight YAML
templates (or just plain YAML), arbitrary Kubernetes resources, hassle-free imports of existing
resources, and fast plan/apply flows.

## Differences from `kubeapply`

This provider has the same general philosophy of the full `kubeapply`, but has
a number of differences that are worth noting:

1. Only raw YAML or templates in
  [golang text/template format](https://golang.org/pkg/text/template/)
  are allowed. [Helm](https://helm.sh/) and
  [skycfg](https://github.com/stripe/skycfg) support has been dropped for now. If you need
  the former, please use
  [Hashicorp's Helm provider](https://registry.terraform.io/providers/hashicorp/helm/latest/docs).
2. All expansion is done by the provider; no user-run expansions are required.
3. Namespaces are automatically generated if required.
4. The cluster config has some extra fields that can be used in templates
  including `AccountName`  and `AccountID`. In addition, `Env` has been renamed to `Environment`
  for clarity.

## Usage

### Requirements

1. Terraform v0.13 or later
2. `kubectl` v1.19 or later
3. The `kadiff` utility defined in this repo and installable via `make install-kadiff`. The latter
  is used for generating more structured Kubernetes diffs than the default `diff` command.

### Local Mirror Installation

In order to use the provider when executing Terraform locally, install the provider into a local
filesystem mirror directory. This step is not necessary if some other system (e.g.,
[Atlantis](https://www.runatlantis.io/)) is executing
Terraform.

```sh
make terraform-provider-mirror-install
```

### Configuration spec

See [here](/docs/) for some auto-generated docs on the provider and resource interfaces.

### Configuration walkthrough

First, add `kubeapply` to the `required_providers` in your root module:

```hcl
terraform {
  backend "s3" {
    ...
  }

  required_providers {
    ...

    kubeapply = {
      source  = "segment.io/kubeapply/kubeapply"
      version = ">= 0.0.1"
    }
  }
}
```

Then, configure the provider. The following example assumes that we're managing resources in an
EKS cluster named `my-cluster`:

```hcl
# As an alternative to using these "data" resources, you can just get the parameters from
# an upstream resource or module.
data "aws_eks_cluster" "cluster" {
  name = "my-cluster"
}

data "aws_eks_cluster_auth" "cluster" {
  name = "my-cluster"
}

provider "kubeapply" {
  cluster_name = "my-cluster"

  # These are made available for templating; if they don't apply, you can set them to
  # empty strings.
  region       = "us-west-2"
  environment  = "development"
  account_name = "dev"
  account_id   = "1234567890"

  # Parameters to create or find kubeconfig.
  #
  # The exact things to set here depend on how you're handling cluster auth. You can also
  # just point the provider at an existing kubeconfig via the 'config_path' parameter.
  host                   = data.aws_eks_cluster.cluster.endpoint
  cluster_ca_certificate = data.aws_eks_cluster.cluster.certificate_authority[0].data
  token                  = data.aws_eks_cluster_auth.cluster.token
}
```

As an alternative to the token-based authentication shown above, you can also
use a pre-defined kubeconfig, client certificates, a custom `exec` process, or a username/password
combination. See the [provider docs](/docs/) for more details on how to configure these.

Finally, create one or more `kubeapply_profile` resources:

```hcl
resource "kubeapply_profile" "main_profile" {
  # Where the manifest templates live; can also be a git reference as allowed for module sources
  source = "${path.module}/manifests"

  parameters = {
    # These are generic key/value pairs that will be made available for templating in
    # the '.Parameters' field.
    namespace = "my-namespace"
    version = "v1.9.5"
  }

  # Set blocks allow you to specify values that aren't strings. The latter will be unmarshalled
  # as JSON before being inserted into '.Parameters'.
  set {
    name = "labels"
    value = jsonencode(["a", "b", "c"])
  }
}
```

The manifests referenced in the `source` field can be either plain YAML files or template
files in [golang text/template](https://golang.org/pkg/text/template/) format. The latter must end
in the extension `.gotpl.yaml`.

Templates are executed in the context of
[this struct](https://github.com/segmentio/terraform-provider-kubeapply/blob/main/pkg/cluster/config.go#L12),
with the fields populated from the configuration of the provider and the resource. Note, in
particular, that any custom parameters will be inserted into the `.Parameters` map.

So, for instance, you could have a file named `deployment.gotpl.yaml` with the following contents:

```
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    cluster: {{.Cluster}}
    account: {{.AccountName}}
    version: {{.Parameters.version}}
  name: my-deployment
  namespace: {{.Parameters.namespace}}
```

There can be an arbitrary number of templates or YAML files, and each can contain multiple resources
separated by `---` lines.

### How it works

On each `plan` run, the provider goes through the following steps:

1. Expand out all of the manifests according to the parameters in each `kubeapply_profile` resource
2. Hash the expanded outputs, check whether these match the hashes in the Terraform state
3. If the hashes have changed, run `kubectl diff` and insert the
  diffs into the resource so they can be seen in the `plan` outputs

When the plan is applied, the provider runs `kubectl apply` on the expanded outputs and cleans
the diffs out of the state.

#### Resource deletions

If `allow_deletes` is set to `true` in the provider (which is the default), then the
provider will delete resources from the Kubernetes API if they're removed from the Terraform
state. Note that these deletions are best-effort and non-blocking; after applying a change
that does a deletion, you'll want to do some manual checking in the cluster to verify that
the resources are actually gone.

## Debugging

### Provider implementation

The provider code can be debugged locally by running with the `--debug-server` flag:

1. (If not already done) Install `kadiff` locally: `make install-kadiff`
2. Build the plugin: `make terraform-provider-kubeapply`
3. Run `[full path to repo dir]/build/terraform-provider-kubeapply --debug-server`
  from the root of your terraform module. Keep the debug server running in the
  background, and note the `TF_REATTACH_PROVIDERS` output for use in the next step.
4. (In a separate terminal) Export the value of `TF_REATTACH_PROVIDERS` into your environment: `export TF_REATTACH_PROVIDERS=[value]`
5. Run `terraform init`, `terraform plan`, `terraform apply`, etc. as normal

Note that the value of `TF_REATTACH_PROVIDERS` will change each time you stop
and restart the debug server.

### Manifest templates

#### Via `show_expanded` (easiest)

The profile resource has an optional field, `show_expanded`, that will cause the
full config expansions to be included in the Terraform output. Given how long these
expansions can be, it's advised to only use this option for temporary debugging purposes.

#### Via `kaexpand`

This repo includes a small command-line tool, `kaexpand`, that simulates the expansion process
that would be done by the provider. It may be useful for quick debugging of manifest templates
in a non-production environment:

1. (If not already done) Install `kaexpand` locally: `make install-kaexpand`
2. Run `kaexpand [path to directory with manifests] [flags]`

The flags, which are all optional, allow you to set common parameters like the cluster name as
well as custom parameter key/value pairs. Run the tool with `--help` to see the available options.

The tool will expand the manifests into a temporary directory by default. You can then run things
like `kubectl diff -R -f [expanded path]` or `kubectl apply -R -f [expanded path]`. The latter will
be done automatically if `kaexpand` is run with the `--diff` or `--apply` flags, respectively.

Note that `kaexpand` does not parse your terraform configs so it will not understand things
like module defaults. This may be added in the future.
