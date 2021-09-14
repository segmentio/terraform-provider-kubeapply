package provider

import (
	"fmt"
	"regexp"
	"strings"
)

// Terraform gets upset if the same diff run multiple times yields any differences. This
// regexp helps to replace the variable parts with fixed placeholders.
var sanitizationRegexp = regexp.MustCompile(`(\s+)(creationTimestamp|uid)[:]([^\n]+)`)

func sanitizeDiff(rawDiff string) string {
	return sanitizationRegexp.ReplaceAllString(rawDiff, "${1}${2}: OMITTED")
}

func prettyResults(rawResults []byte) string {
	lines := strings.Split(string(rawResults), "\n")
	output := []string{}

	for _, line := range lines {
		if line != "" {
			output = append(output, fmt.Sprintf("> %s", line))
		}
	}

	return strings.Join(output, "\n")
}
