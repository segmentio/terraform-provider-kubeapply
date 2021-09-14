package provider

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	// Value that's used as a sentinel by Terraform to indicate that a value is unknown
	// at diff time.
	unknownValue = "74D93920-ED26-11E3-AC10-0800200C9A66"
)

type resourceGetter interface {
	GetOk(key string) (interface{}, bool)
	Get(key string) interface{}
}

var _ resourceGetter = (*schema.ResourceData)(nil)
var _ resourceGetter = (*schema.ResourceDiff)(nil)

type resourceChanger interface {
	resourceGetter
	HasChange(key string) bool
	GetChange(key string) (interface{}, interface{})
}

var _ resourceChanger = (*schema.ResourceData)(nil)
var _ resourceChanger = (*schema.ResourceDiff)(nil)

// Set methods are slightly different in ResourceData vs. ResourceDiff.
type resourceChangerSetter interface {
	resourceChanger
	Set(key string, value interface{}) error
	SetId(id string)
}

var _ resourceChangerSetter = (*schema.ResourceData)(nil)

type resourceDiffChangerSetter interface {
	resourceChanger
	SetNew(key string, value interface{}) error
	SetNewComputed(key string) error
}

var _ resourceDiffChangerSetter = (*schema.ResourceDiff)(nil)

func moduleName(data resourceGetter) string {
	source := data.Get("source").(string)
	components := strings.Split(source, "/")
	if len(components) >= 3 && components[0] == ".terraform" && components[1] == "modules" {
		return components[2]
	}

	return "module"
}

type resourceChanges struct {
	added     []string
	updated   []string
	removed   []string
	unchanged []string
}

func getResourceChanges(changer resourceChanger) resourceChanges {
	changes := resourceChanges{}

	oldResources, newResources := changer.GetChange("resources")

	oldKeys := map[string]struct{}{}
	for key := range oldResources.(map[string]interface{}) {
		oldKeys[key] = struct{}{}
	}

	newKeys := map[string]struct{}{}
	for key := range newResources.(map[string]interface{}) {
		newKeys[key] = struct{}{}
	}

	for key := range oldResources.(map[string]interface{}) {
		if _, ok := newKeys[key]; !ok {
			changes.removed = append(changes.removed, key)
		} else {
			oldValue := oldResources.(map[string]interface{})[key].(string)
			newValue := newResources.(map[string]interface{})[key].(string)

			if oldValue != newValue {
				changes.updated = append(changes.updated, key)
			} else {
				changes.unchanged = append(changes.unchanged, key)
			}
		}
	}

	for key := range newResources.(map[string]interface{}) {
		if _, ok := oldKeys[key]; !ok {
			changes.added = append(changes.added, key)
		}
	}

	return changes
}

func getHasUnknownParameters(changer resourceChanger) bool {
	hasChange := changer.HasChange("parameters")
	value, ok := changer.GetOk("parameters")
	valueMap := value.(map[string]interface{})

	// If there's a change, the value exists, and the map is empty, then we have unknown
	// parameter inputs
	return hasChange && ok && len(valueMap) == 0
}

func getHasUnknownSetValues(changer resourceChanger) bool {
	setParams := changer.Get("set").(*schema.Set).List()

	for _, setParam := range setParams {
		rawMap := setParam.(map[string]interface{})
		strValue := rawMap["value"].(string)

		// If any values are unknown, then we have unknown set inputs
		if strValue == unknownValue {
			return true
		}
	}

	return false
}
