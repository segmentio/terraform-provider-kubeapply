package apply

import (
	"bytes"

	"github.com/olekukonko/tablewriter"
)

// ResultsTextTable returns a pretty table that summarizes the results of
// a kubectl apply run.
func ResultsTextTable(results []Result) string {
	buf := &bytes.Buffer{}

	table := tablewriter.NewWriter(buf)
	table.SetHeader(
		[]string{
			"Op",
			"Namespace",
			"Kind",
			"Name",
			"Created",
			"Old Version",
			"New Version",
		},
	)
	table.SetAutoWrapText(false)
	table.SetColumnAlignment(
		[]int{
			tablewriter.ALIGN_LEFT,
			tablewriter.ALIGN_LEFT,
			tablewriter.ALIGN_LEFT,
			tablewriter.ALIGN_LEFT,
			tablewriter.ALIGN_LEFT,
			tablewriter.ALIGN_LEFT,
		},
	)
	table.SetBorders(
		tablewriter.Border{
			Left:   false,
			Top:    true,
			Right:  false,
			Bottom: true,
		},
	)

	for _, result := range results {
		var op string

		if result.IsCreated() {
			op = "+"
		} else if result.IsUpdated() {
			op = "~"
		}

		table.Append(
			[]string{
				op,
				result.Namespace,
				result.Kind,
				result.Name,
				result.CreatedTimestamp(),
				result.OldVersion,
				result.NewVersion,
			},
		)
	}

	table.Render()
	return string(bytes.TrimRight(buf.Bytes(), "\n"))
}
