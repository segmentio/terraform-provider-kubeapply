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

## Installation and usage

See the
[documentation in the Terraform Registry](https://registry.terraform.io/providers/segmentio/kubeapply/latest/docs). A local version of this content can be found [here](/docs).

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
