package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
)

type payload interface {
	Columns() []string
	Rows() [][]interface{}
}

type payloadBasic struct {
	Results [1]struct {
		Events []map[string]interface{} `json:"events"`
	} `json:"results"`
	Metadata struct {
		Contents [1]struct {
			Columns []string `json:"columns"`
		} `json:"contents"`
	} `json:"metadata"`
}

func (p payloadBasic) Columns() []string {
	return p.Metadata.Contents[0].Columns
}

func (p payloadBasic) Rows() [][]interface{} {
	var rows [][]interface{}
	columns := p.Columns()
	for _, event := range p.Results[0].Events {
		row := make([]interface{}, len(columns))
		for i, column := range columns {
			row[i] = event[column]
		}
		rows = append(rows, row)
	}
	return rows
}

type payloadAggregation struct {
	Results  []map[string]interface{} `json:"results"`
	Metadata struct {
		Contents []struct {
			Function string `json:"function"`

			// Only populated if Function == "alias"
			Alias string `json:"alias"`

			// Only populated if Function == "alias"
			Contents struct {
				Function  string `json:"function"`
				Attribute string `json:"attribute"`
			}

			// Empty if Function == "alias"
			Attribute string `json:"attribute"`
		} `json:"contents"`
	} `json:"metadata"`
}

func (p payloadAggregation) Columns() []string {
	columns := make([]string, len(p.Metadata.Contents))
	for i, content := range p.Metadata.Contents {
		columns[i] = content.Function
		if columns[i] == "alias" {
			columns[i] = content.Alias
		}
	}
	return columns
}

// A cell is a single-element mapping between a string (usually a function
// name) and a scalar value. I don't understand why NewRelic chose a map to
// represent a single element (perhaps there are edge cases where there might
// be more than one element, but I can't imagine what they might be). If there
func parseCell(cell map[string]interface{}) interface{} {
	if len(cell) != 1 {
		// This shouldn't happen; for debugging purposes, we'll just fail
		// loudly if it does
		log.Fatal("WARNING: multiple key/value pairs found in cell:", cell)
	}
	for _, v := range cell {
		return v
	}
	panic("Unreachable")
}

func parseRow(row []map[string]interface{}) []interface{} {
	out := make([]interface{}, len(row))
	for i, cell := range row {
		out[i] = parseCell(cell)
	}
	return out
}

// This always returns one row
func (p payloadAggregation) Rows() [][]interface{} {
	return [][]interface{}{parseRow(p.Results)}
}

type payloadFacet struct {
	Facets []struct {
		Name    string                   `json:"name"`
		Results []map[string]interface{} `json:"results"`
	} `json:"facets"`
	TotalResult struct {
		Results []map[string]interface{} `json:"results"`
	} `json:"totalResult"`
	UnknownGroup struct {
		Results []map[string]interface{} `json:"results"`
	} `json:"unknownGroup"`
	Metadata struct {
		Facet    string `json:"facet"`
		Contents struct {
			Contents []struct {
				Function string `json:"function"`

				// Only populated if Function == "alias"
				Alias string `json:"alias"`

				// Only populated if Function == "alias"
				Contents struct {
					Function  string `json:"function"`
					Attribute string `json:"attribute"`
				}

				// Empty if Function == "alias"
				Attribute string `json:"attribute"`
			} `json:"contents"`
		} `json:"contents"`
	} `json:"metadata"`
}

func (p payloadFacet) Columns() []string {
	columns := make([]string, len(p.Metadata.Contents.Contents)+1)
	columns[0] = p.Metadata.Facet
	for i, content := range p.Metadata.Contents.Contents {
		columns[i+1] = content.Function
		if content.Function == "alias" {
			println("ALIAS:", content.Alias)
			columns[i+1] = content.Alias
		}
	}
	return columns
}

func (p payloadFacet) Rows() [][]interface{} {
	rows := make([][]interface{}, len(p.Facets))
	for i, facet := range p.Facets {
		row := make([]interface{}, len(facet.Results)+1)
		row[0] = facet.Name
		for j, cell := range facet.Results {
			row[j+1] = parseCell(cell)
		}
		rows[i] = row
	}
	return rows
}

func unmarshalPayload(data []byte) (payload, error) {
	// Allocate 3 mutually exclusive payload instances; exactly one of these
	// should match the JSON payload. This is a hack, but I can't think of a
	// better way to cope with NewRelic's wonky API.
	var basic payloadBasic
	var aggregation payloadAggregation
	var facet payloadFacet

	var basicErr error
	if basicErr = json.Unmarshal(data, &basic); basicErr == nil {
		if basic.Results[0].Events != nil {
			if basic.Metadata.Contents[0].Columns != nil {
				return basic, nil
			}
		}
	}

	var aggregationErr error
	if aggregationErr = json.Unmarshal(data, &aggregation); aggregationErr == nil {
		if aggregation.Results != nil {
			return aggregation, nil
		}
	}

	var facetErr error
	if facetErr = json.Unmarshal(data, &facet); facetErr == nil {
		return facet, nil
	}

	// pretty print payload data for error message
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "    "); err != nil {
		panic(err)
	}

	// pretty print error data for error message
	errorJSON, err := json.MarshalIndent(
		map[string]string{
			"basic":       basicErr.Error(),
			"aggregation": aggregationErr.Error(),
			"facet":       facetErr.Error(),
		},
		"",
		"    ",
	)
	if err != nil {
		panic(err)
	}

	return nil, fmt.Errorf(
		"Couldn't find a match for payload.\nErrors: %s\nData: %s",
		errorJSON,
		buf.String(),
	)
}
