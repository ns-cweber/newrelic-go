package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
)

// This is an abstraction over all of the varieties of payloads the New Relic
// API might send down.
type payload interface {
	columns() []string
	rows() [][]interface{}
}

// This type wraps an existing payload and suffixes it with fixed data from
// static columns. Given a table with columns {a, b, c} and static columns
// {d, e} with values {4, 5} respectively, the resultant columns will be {a, b,
// c, d, e} and the last two columns will be entirely 4s and 5s respectively.
type staticColumnsPayload struct {
	payload
	staticColumns []staticColumn
}

func (p staticColumnsPayload) columns() []string {
	columns := p.payload.columns()
	staticColumnHeaders := make([]string, len(p.staticColumns))
	for i, column := range p.staticColumns {
		staticColumnHeaders[i] = column.name
	}
	return append(columns, staticColumnHeaders...)
}

func (p staticColumnsPayload) rows() [][]interface{} {
	rows := p.payload.rows()
	for _, column := range p.staticColumns {
		for i, row := range rows {
			rows[i] = append(row, column.value)
		}
	}
	return rows
}

// This represents the basic (no-aggregations, no-facets) payload type.
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

func (p payloadBasic) columns() []string {
	return p.Metadata.Contents[0].Columns
}

func (p payloadBasic) rows() [][]interface{} {
	var rows [][]interface{}
	columns := p.columns()
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

func (p payloadAggregation) columns() []string {
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
func (p payloadAggregation) rows() [][]interface{} {
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

func (p payloadFacet) columns() []string {
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

func (p payloadFacet) rows() [][]interface{} {
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

// This function tries to guess the type of New Relic payload and decode it
// accordingly
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
			basicErr = fmt.Errorf("missing 'metadata.contents[0].columns' field")
		} else {
			basicErr = fmt.Errorf("missing 'results[0].events' field")
		}
	}

	var aggregationErr error
	if aggregationErr = json.Unmarshal(data, &aggregation); aggregationErr == nil {
		if aggregation.Results != nil {
			return aggregation, nil
		}
		aggregationErr = fmt.Errorf("missing 'results' field")
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
