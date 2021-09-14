package provider

import (
	"reflect"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakeDiffChangerSetter struct {
	newComputed map[string]struct{}
	oldValues   map[string]interface{}
	newValues   map[string]interface{}
}

func (f fakeDiffChangerSetter) Get(key string) interface{} {
	return f.newValues[key]
}

func (f fakeDiffChangerSetter) GetOk(key string) (interface{}, bool) {
	value, ok := f.newValues[key]
	return value, ok
}

func (f fakeDiffChangerSetter) HasChange(key string) bool {
	return !reflect.DeepEqual(f.oldValues[key], f.newValues[key])
}

func (f fakeDiffChangerSetter) GetChange(key string) (interface{}, interface{}) {
	return f.oldValues[key], f.newValues[key]
}

func (f fakeDiffChangerSetter) SetNew(key string, value interface{}) error {
	f.newValues[key] = value
	return nil
}

func (f fakeDiffChangerSetter) SetNewComputed(key string) error {
	f.newComputed[key] = struct{}{}
	return nil
}

var _ resourceDiffChangerSetter = (*fakeDiffChangerSetter)(nil)

func TestGetResourceChanges(t *testing.T) {
	changer := fakeDiffChangerSetter{
		oldValues: map[string]interface{}{
			"resources": map[string]interface{}{
				"resource1": "hash1",
				"resource2": "hash2",
				"resource3": "hash3",
			},
		},
		newValues: map[string]interface{}{
			"resources": map[string]interface{}{
				"resource1": "hash1",
				"resource2": "hash2updated",
				"resource4": "hash4",
				"resource5": "hash5",
			},
		},
	}

	changes := getResourceChanges(changer)
	sortStringSlice(changes.added)
	sortStringSlice(changes.updated)
	sortStringSlice(changes.removed)
	sortStringSlice(changes.unchanged)

	assert.Equal(
		t,
		resourceChanges{
			added:     []string{"resource4", "resource5"},
			updated:   []string{"resource2"},
			removed:   []string{"resource3"},
			unchanged: []string{"resource1"},
		},
		changes,
	)
}

func sortStringSlice(strs []string) {
	sort.Slice(strs, func(a, b int) bool {
		return strs[a] < strs[b]
	})
}
