---
page_title: "kubeapply_profile Resource - terraform-provider-kubeapply"
subcategory: ""
description: |-

---

# kubeapply_profile (Resource)

A `profile` represents a bundle of YAML resources that will be expanded, diffed, and applied
as a single unit.

## Configuration

Each `profile` instance contains a source path and (optionally) parameters that will be used
for templating. Here's an example:

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

- **source** (String) Source for profile manifest files in local file system or remote git repo

### Optional

- **id** (String) The ID of this resource
- **parameters** (Map of String) Arbitrary parameters that will be used for profile expansion
- **set** (Block Set) Custom, JSON-encoded parameters to be merged parameters above (see [below for nested schema](#nestedblock--set))
- **show_expanded** (Boolean) Show expanded output

### Read-Only

- **diff** (Map of String) Diff result from applying changed files
- **expanded_files** (Map of String) Result of expanding templates; only set if show_expanded is set to true
- **resources** (Map of String) Resources in this profile
- **resources_hash** (String) Hash of all resources in this profile

<a id="nestedblock--set"></a>
### Nested Schema for `set`

Required:

- **name** (String)
- **value** (String)

Optional:

- **placeholder** (String)
