package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/segmentio/terraform-provider-kubeapply/pkg/cluster/diff"
	log "github.com/sirupsen/logrus"
)

// profileResource defines a new kubeapply_profile resource instance. The only required field
// is a path to the manifests
func profileResource() *schema.Resource {
	return &schema.Resource{
		// Wrap schema functions for easier testing; unfortunately, creating schema.ResourceData
		// and schema.ResourceDiff instances directly for testing purposes is fairly tedious.
		CreateContext: func(
			ctx context.Context,
			data *schema.ResourceData,
			provider interface{},
		) diag.Diagnostics {
			return resourceProfileCreate(ctx, data, provider)
		},
		ReadContext: func(
			ctx context.Context,
			data *schema.ResourceData,
			provider interface{},
		) diag.Diagnostics {
			return resourceProfileRead(ctx, data, provider)
		},
		UpdateContext: func(
			ctx context.Context,
			data *schema.ResourceData,
			provider interface{},
		) diag.Diagnostics {
			return resourceProfileUpdate(ctx, data, provider)
		},
		DeleteContext: func(
			ctx context.Context,
			data *schema.ResourceData,
			provider interface{},
		) diag.Diagnostics {
			return resourceProfileDelete(ctx, data, provider)
		},
		CustomizeDiff: func(
			ctx context.Context,
			data *schema.ResourceDiff,
			provider interface{},
		) error {
			return resourceProfileCustomDiff(ctx, data, provider)
		},
		Schema: map[string]*schema.Schema{
			// Inputs
			"no_diff": {
				Type:        schema.TypeBool,
				Description: "Don't do a full diff for this resource",
				Optional:    true,
			},
			"parameters": {
				Type:        schema.TypeMap,
				Description: "Arbitrary parameters that will be used for profile expansion",
				Optional:    true,
			},
			"set": {
				Type:        schema.TypeSet,
				Optional:    true,
				Description: "Custom, JSON-encoded parameters to be merged parameters above",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"value": {
							Type:     schema.TypeString,
							Required: true,
						},
						"placeholder": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "",
						},
					},
				},
			},
			"show_expanded": {
				Type:        schema.TypeBool,
				Description: "Show expanded output",
				Optional:    true,
			},
			"source": {
				Type:        schema.TypeString,
				Description: "Source for profile manifest files in local file system or remote git repo",
				Required:    true,
			},

			// Computed fields
			"diff": {
				Type:        schema.TypeMap,
				Description: "Diff result from applying changed files",
				Computed:    true,
			},
			"expanded_files": {
				Type:        schema.TypeMap,
				Description: "Result of expanding templates; only set if show_expanded is set to true",
				Computed:    true,
			},
			"resources": {
				Type:        schema.TypeMap,
				Description: "Resources in this profile",
				Computed:    true,
			},
			"resources_hash": {
				Type:        schema.TypeString,
				Description: "Hash of all resources in this profile",
				Computed:    true,
			},
		},
	}
}

func resourceProfileCreate(
	ctx context.Context,
	data resourceChangerSetter,
	provider interface{},
) diag.Diagnostics {
	var diags diag.Diagnostics

	log.Infof("Running create for %s", moduleName(data))
	providerCtx := provider.(*providerContext)

	if !providerCtx.canRun {
		err := fmt.Errorf("Cannot create because provider is missing a host or kubeconfig")
		diags = append(diags, diag.FromErr(err)...)
		return diags
	}

	expandResult, err := providerCtx.expand(ctx, data)
	if err != nil {
		diags = append(diags, diag.FromErr(err)...)
		return diags
	}
	defer providerCtx.cleanExpanded(expandResult)

	applyDiags := providerCtx.apply(
		ctx,
		expandResult.expandedDir,
		moduleName(data),
	)
	diags = append(diags, applyDiags...)

	if diags.HasError() {
		return diags
	}

	if err := data.Set("resources", expandResult.resources); err != nil {
		diags = append(diags, diag.FromErr(err)...)
		return diags
	}

	// Null out diff and expanded_files so they're not persisted and we get a clean diff for the
	// next apply.
	if err := data.Set("diff", map[string]interface{}{}); err != nil {
		diags = append(diags, diag.FromErr(err)...)
		return diags
	}
	if err := data.Set("expanded_files", map[string]interface{}{}); err != nil {
		diags = append(diags, diag.FromErr(err)...)
		return diags
	}

	// Just make up an id from the timestamp
	data.SetId(fmt.Sprintf("%d", time.Now().UnixNano()))

	log.Infof("Create successful for %s", moduleName(data))
	return diags
}

func resourceProfileRead(
	ctx context.Context,
	data resourceChangerSetter,
	provider interface{},
) diag.Diagnostics {
	log.Infof("Running read for %s", moduleName(data))
	var diags diag.Diagnostics

	// There's nothing to do here since we only read the remote kube state if we're doing a
	// diff.
	return diags
}

func resourceProfileCustomDiff(
	ctx context.Context,
	data resourceDiffChangerSetter,
	provider interface{},
) error {
	log.Infof("Running custom diff for %s", moduleName(data))

	providerCtx := provider.(*providerContext)

	hasUnknownParameters := getHasUnknownParameters(data)
	hasUnknownSetValues := getHasUnknownSetValues(data)

	log.Infof(
		"Unknown parameters: %+v; unknown set values: %+v",
		hasUnknownParameters,
		hasUnknownSetValues,
	)

	if !providerCtx.canRun {
		// There's not much we can do if the provider can't run due to missing parameters
		log.Infof(
			"Not doing expansion and diff for module %s because the provider is missing a host or kubeconfig",
			moduleName(data),
		)
		if err := data.SetNew(
			"diff",
			map[string]interface{}{
				"": "DIFFS UNKNOWN because provider missing host or kubeconfig",
			},
		); err != nil {
			return err
		}
		if err := data.SetNew("expanded_files", map[string]interface{}{}); err != nil {
			return err
		}
		if err := data.SetNewComputed("resources"); err != nil {
			return err
		}
		if err := data.SetNewComputed("resources_hash"); err != nil {
			return err
		}
		return nil
	} else if hasUnknownParameters {
		// There's not much we can do if we have known parameters since terraform sets the
		// entire parameters map to an empty value
		log.Infof(
			"Not doing expansion and diff for module %s because one or more parameters are unknown",
			moduleName(data),
		)
		if err := data.SetNew(
			"diff",
			map[string]interface{}{
				"": "DIFFS UNKNOWN because one or more parameters are unknown",
			},
		); err != nil {
			return err
		}
		if err := data.SetNew("expanded_files", map[string]interface{}{}); err != nil {
			return err
		}
		if err := data.SetNewComputed("resources"); err != nil {
			return err
		}
		if err := data.SetNewComputed("resources_hash"); err != nil {
			return err
		}
		return nil
	}

	expandResult, err := providerCtx.expand(ctx, data)
	if err != nil {
		return err
	}
	defer providerCtx.cleanExpanded(expandResult)

	log.Infof(
		"Found %d manifests with overall hash of %s for %s",
		len(expandResult.manifests),
		expandResult.totalHash,
		moduleName(data),
	)

	if providerCtx.shouldShowExpanded(data) {
		if err := data.SetNew("expanded_files", expandResult.expandedFiles); err != nil {
			return err
		}
	} else {
		if err := data.SetNew("expanded_files", map[string]interface{}{}); err != nil {
			return err
		}
	}

	// Set resources
	if hasUnknownSetValues {
		// If there are unknown set values and we're using placeholders, we don't want to
		// set these because this will cause terraform to complain about inconsistent diffs
		// after applying.
		log.Infof("Setting resources for module %s as unknown", moduleName(data))
		if err := data.SetNewComputed("resources"); err != nil {
			return err
		}
		if err := data.SetNewComputed("resources_hash"); err != nil {
			return err
		}
	} else {
		if err := data.SetNew("resources", expandResult.resources); err != nil {
			return err
		}
		if err := data.SetNew("resources_hash", expandResult.totalHash); err != nil {
			return err
		}
	}

	changes := getResourceChanges(data)
	log.Infof(
		"%d/%d/%d/%d resources are added/updated/removed/unchanged for %s",
		len(changes.added),
		len(changes.updated),
		len(changes.removed),
		len(changes.unchanged),
		moduleName(data),
	)

	if providerCtx.shouldDiff(data) {
		log.Infof(
			"Doing full diff for %s",
			moduleName(data),
		)
		var results map[string]interface{}

		if err := providerCtx.createNamespaces(ctx, expandResult.manifests); err != nil {
			return err
		}

		diffs, err := providerCtx.diff(ctx, expandResult.expandedDir)
		if err != nil {
			return err
		}
		log.Infof(
			"Got structured diff output for %s with %d resources changed",
			moduleName(data),
			len(diffs),
		)

		results = map[string]interface{}{}
		for _, diffObj := range diffs {
			if providerCtx.verboseDiffs {
				results[diffObj.Name] = sanitizeDiff(diffObj.RawDiff)
			} else {
				switch diffObj.Operation {
				case diff.OperationCreate:
					results[diffObj.Name] = fmt.Sprintf(
						"Creating new resource (%d lines added)",
						diffObj.NumAdded,
					)
				case diff.OperationDelete:
					results[diffObj.Name] = fmt.Sprintf(
						"Completely removing resource (%d lines removed)",
						diffObj.NumRemoved,
					)
				default:
					results[diffObj.Name] = sanitizeDiff(diffObj.RawDiff)
				}
			}
		}

		if providerCtx.canDelete(data) {
			for _, id := range changes.removed {
				results[id] = "TO BE DELETED"
			}
		} else {
			for _, id := range changes.removed {
				results[id] = "TO BE REMOVED from Terraform but will not be deleted from cluster due to value of allow_deletes.\nPlease delete manually."
			}
		}

		if len(results) == 0 && data.HasChange("resources") {
			// Add an explicit placeholder so that terraform doesn't show "(known after apply)"
			// when creating a resource with existing kube configs for the first time
			results[""] = "NO DIFFS FOUND"
		}

		if err := data.SetNew("diff", results); err != nil {
			return err
		}
	} else {
		data.SetNew("diff", map[string]interface{}{})
		log.Infof(
			"Skipping diff for %s",
			moduleName(data),
		)
	}

	return nil
}

func resourceProfileUpdate(
	ctx context.Context,
	data resourceChangerSetter,
	provider interface{},
) diag.Diagnostics {
	var diags diag.Diagnostics
	providerCtx := provider.(*providerContext)

	if !providerCtx.canRun {
		err := fmt.Errorf("Cannot update because provider is missing a host or kubeconfig")
		diags = append(diags, diag.FromErr(err)...)
		return diags
	}

	changes := getResourceChanges(data)

	if len(changes.removed) > 0 {
		deleteDiags := providerCtx.delete(ctx, data, changes.removed)
		diags = append(diags, deleteDiags...)
		if diags.HasError() {
			return diags
		}
	}

	log.Infof("Running update for %s", moduleName(data))
	diffValue := data.Get("diff").(map[string]interface{})

	// Null out diff and expanded_files so they're not persisted and we get a clean diff for the
	// next apply.
	if err := data.Set("diff", map[string]interface{}{}); err != nil {
		diags = append(diags, diag.FromErr(err)...)
		return diags
	}
	if err := data.Set("expanded_files", map[string]interface{}{}); err != nil {
		diags = append(diags, diag.FromErr(err)...)
		return diags
	}

	if len(diffValue) > 0 {
		expandResult, err := providerCtx.expand(ctx, data)
		if err != nil {
			diags = append(diags, diag.FromErr(err)...)
			return diags
		}
		defer providerCtx.cleanExpanded(expandResult)

		applyDiags := providerCtx.apply(
			ctx,
			expandResult.expandedDir,
			moduleName(data),
		)
		diags = append(diags, applyDiags...)

		if diags.HasError() {
			return diags
		}
	} else {
		log.Infof(
			"Diff is empty for %s, so not running apply",
			moduleName(data),
		)
	}

	diags = append(diags, resourceProfileRead(ctx, data, provider)...)
	return diags
}

func resourceProfileDelete(
	ctx context.Context,
	data resourceChangerSetter,
	provider interface{},
) diag.Diagnostics {
	providerCtx := provider.(*providerContext)

	if !providerCtx.canRun && providerCtx.canDelete(data) {
		err := fmt.Errorf("Cannot delete because provider is missing a host or kubeconfig")
		return diag.FromErr(err)
	}

	// Delete all resources
	resources := data.Get("resources").(map[string]interface{})

	ids := []string{}
	for id := range resources {
		ids = append(ids, id)
	}

	return providerCtx.delete(ctx, data, ids)
}
