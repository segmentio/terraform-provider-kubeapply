package diff

import (
	"fmt"
	"strings"

	"github.com/segmentio/terraform-provider-kubeapply/pkg/cluster/apply"
	log "github.com/sirupsen/logrus"
)

// Operation describes whether a diff block represents a creation, deletion, or update.
type Operation string

const (
	OperationCreate Operation = "create"
	OperationDelete Operation = "delete"
	OperationUpdate Operation = "update"
)

// Results contains all results from a given diff run. It's used for wrapping so that
// everything can be put in a single struct when exported by kubeapply kdiff.
type Results struct {
	Results []Result `json:"results"`
}

// Result contains the results of diffing a single object.
type Result struct {
	Object     *apply.TypedKubeObj `json:"object"`
	Name       string              `json:"name"`
	RawDiff    string              `json:"rawDiff"`
	NumAdded   int                 `json:"numAdded"`
	NumRemoved int                 `json:"numRemoved"`
	Operation  Operation           `json:"operation"`
}

// PrintFull prints out a table and the raw diffs for a results slice.
func PrintFull(results []Result) {
	if len(results) == 0 {
		log.Infof("No diffs found")
		return
	}

	log.Infof("Diffs summary:\n%s", ResultsTable(results))
	log.Info("Raw diffs:")
	for _, result := range results {
		result.PrintRaw()
	}
}

// PrintSummary prints out a summary table for a results slice.
func PrintSummary(results []Result) {
	if len(results) == 0 {
		log.Infof("No diffs found")
		return
	}

	log.Infof("Diffs summary:\n%s", ResultsTable(results))
}

// PrintRaw prints out the raw diffs for a single resource.
func (r *Result) PrintRaw() {
	lines := strings.Split(r.RawDiff, "\n")
	for _, line := range lines {
		var prefix string
		if len(line) > 0 {
			prefix = line[0:1]
		}

		switch prefix {
		case "+":
			fmt.Println(line)
		case "-":
			fmt.Println(line)
		default:
			if len(line) > 0 {
				fmt.Println(line)
			}
		}
	}
}

// ClippedRawDiff returns a clipped version of the raw diff for this result. Used
// in the Github diff comment template.
func (r *Result) ClippedRawDiff(maxLen int) string {
	if len(r.RawDiff) > maxLen {
		return fmt.Sprintf(
			"%s\n... (%d chars omitted)",
			r.RawDiff[0:maxLen],
			len(r.RawDiff)-maxLen,
		)
	}
	return r.RawDiff
}

// NumChangedLines returns the rough number of lines changed (taken as the max of the num
// added and num removed).
func (r *Result) NumChangedLines() int {
	if r.NumAdded > r.NumRemoved {
		return r.NumAdded
	}
	return r.NumRemoved
}
