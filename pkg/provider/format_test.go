package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeDiff(t *testing.T) {
	assert.Equal(
		t,
		`
			key: value
			key2: value2
			creationTimestamp: OMITTED
			uid: OMITTED
			key3: value3
		`,
		sanitizeDiff(`
			key: value
			key2: value2
			creationTimestamp: 2020-01-10
			uid: 12345h3123
			key3: value3
		`),
	)
}
