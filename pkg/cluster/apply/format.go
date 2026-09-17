package apply

import (
	"bytes"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

// ResultsTextTable returns a pretty table that summarizes the results of
// a kubectl apply run.
func ResultsTextTable(results []Result) string {
	buf := &bytes.Buffer{}

	table := tablewriter.NewTable(
		buf,
		tablewriter.WithRenderer(
			renderer.NewBlueprint(
				tw.Rendition{
					Symbols: tw.NewSymbols(tw.StyleASCII),
					Borders: tw.Border{
						Left:   tw.Off,
						Right:  tw.Off,
						Top:    tw.On,
						Bottom: tw.On,
					},
				},
			),
		),
		tablewriter.WithHeaderAutoWrap(tw.WrapNone),
		tablewriter.WithRowAutoWrap(tw.WrapNone),
		tablewriter.WithRowAlignment(tw.AlignLeft),
	)
	table.Header(
		"Op",
		"Namespace",
		"Kind",
		"Name",
		"Created",
		"Old Version",
		"New Version",
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
