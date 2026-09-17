package diff

import (
	"bytes"
	"fmt"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

// ResultsTable returns a table that summarizes a slice of result diffs.
func ResultsTable(results []Result) string {
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
		"Namespace",
		"Kind",
		"Name",
		"Changed Lines",
	)

	for _, result := range results {
		var kind string
		var name string
		var namespace string

		if result.Object != nil {
			kind = result.Object.Kind
			name = result.Object.KubeMetadata.Name
			namespace = result.Object.KubeMetadata.Namespace
		} else {
			name = result.Name
		}

		table.Append(
			[]string{
				namespace,
				kind,
				name,
				fmt.Sprintf("%d", result.NumChangedLines()),
			},
		)
	}

	table.Render()
	return string(bytes.TrimRight(buf.Bytes(), "\n"))
}
