package kube

import (
	"errors"
	"fmt"
	"strings"
)

type apiResource struct {
	name       string
	shortNames []string
	apiVersion string
	namespaced bool
	kind       string
}

func parseResourcesTable(rawResources string) ([]apiResource, error) {
	apiResources := []apiResource{}

	rows := strings.Split(strings.TrimSpace(rawResources), "\n")

	if len(rows) == 0 {
		return nil, errors.New("No api-resources found")
	}

	header := rows[0]
	columnStarts := []int{}
	var prevChar byte = ' '

	for i := 0; i < len(header); i++ {
		currChar := header[i]

		if prevChar == ' ' && currChar != ' ' {
			columnStarts = append(columnStarts, i)
		}

		prevChar = currChar
	}

	if len(columnStarts) != 5 {
		return nil, fmt.Errorf(
			"Unexpected number of columns; expected 5, got %d",
			len(columnStarts),
		)
	}

	for _, row := range rows[1:] {
		trimmedRow := strings.Trim(row, " ")
		if len(trimmedRow) == 0 {
			continue
		}

		elements := parseRow(trimmedRow, columnStarts)

		if len(elements) < 5 {
			return nil, fmt.Errorf(
				"Unexpected number of columns in row %s; expected 5, got %d",
				trimmedRow,
				len(elements),
			)
		}

		var shortNames []string

		if len(elements[1]) > 0 {
			shortNames = strings.Split(elements[1], ",")
		} else {
			shortNames = []string{}
		}

		apiResources = append(
			apiResources,
			apiResource{
				name:       elements[0],
				shortNames: shortNames,
				apiVersion: elements[2],
				namespaced: elements[3] == "true",
				kind:       elements[4],
			},
		)
	}

	return apiResources, nil
}

func parseRow(row string, columnStarts []int) []string {
	elements := []string{}

	for i := 0; i < len(columnStarts); i++ {
		var end int
		if i < len(columnStarts)-1 {
			end = columnStarts[i+1]
		} else {
			end = len(row)
		}

		if end > len(row) {
			break
		}

		elements = append(
			elements,
			strings.TrimRight(row[columnStarts[i]:end], " "),
		)
	}

	return elements
}
