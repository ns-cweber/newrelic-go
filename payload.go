package nrql

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
)

// This is an abstraction over all of the varieties of payloads the New Relic
// API might send down.
type Payload interface {
	Columns() []string
	Rows() [][]interface{}
}

type StaticColumn struct {
	Name, Value string
}

// This type wraps an existing payload and suffixes it with fixed data from
// static columns. Given a table with columns {a, b, c} and static columns
// {d, e} with values {4, 5} respectively, the resultant columns will be {a, b,
// c, d, e} and the last two columns will be entirely 4s and 5s respectively.
type StaticColumnsPayload struct {
	Payload
	StaticColumns []StaticColumn
}

func (p StaticColumnsPayload) Columns() []string {
	columns := p.Payload.Columns()
	staticColumnHeaders := make([]string, len(p.StaticColumns))
	for i, column := range p.StaticColumns {
		staticColumnHeaders[i] = column.Name
	}
	return append(columns, staticColumnHeaders...)
}

func (p StaticColumnsPayload) Rows() [][]interface{} {
	rows := p.Payload.Rows()
	for _, column := range p.StaticColumns {
		for i, row := range rows {
			rows[i] = append(row, column.Value)
		}
	}
	return rows
}

// This represents the basic (no-aggregations, no-facets) payload type.
type PayloadBasic struct {
	// The first time we evaluate the columns, we'll cache them here. This is
	// necessary because "SELECT *" queries don't populate the
	// Metadata.Contents[0].Columns field, and we have to check the first
	// element in Results[0].Events; however, this is a map, and map accesses
	// are random, so subsequent calls will give back the column headers in
	// different orders. Thus, this cache allows us to evaluate the map once
	// at most, so subsequent calls to Columns() always gives the same result.
	cols    []string
	Results [1]struct {
		Events []map[string]interface{} `json:"events"`
	} `json:"results"`
	Metadata struct {
		Contents [1]struct {
			// This will not be populated if the query was "SELECT * ..."
			Columns []string `json:"columns"`
		} `json:"contents"`
	} `json:"metadata"`
}

func (p *PayloadBasic) Columns() []string {
	// This is fixed
	if p.cols != nil {
		return p.cols
	}

	// This is nil for "SELECT * ..." queries
	if p.cols = p.Metadata.Contents[0].Columns; p.cols != nil {
		return p.cols
	}

	// If this is nil, we should look to the first row for our columns. If
	// there are no rows, we're up a creek...
	if len(p.Results[0].Events) < 0 {
		return nil
	}

	// The returned rows will not be in any particular order because map
	// accesses (in most versions of the Go compiler) are random.
	p.cols = make([]string, 0, len(p.Results[0].Events[0]))
	for column := range p.Results[0].Events[0] {
		p.cols = append(p.cols, column)
	}
	return p.cols
}

func (p PayloadBasic) Rows() [][]interface{} {
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

type PayloadAggregation struct {
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

func (p PayloadAggregation) Columns() []string {
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
func (p PayloadAggregation) Rows() [][]interface{} {
	return [][]interface{}{parseRow(p.Results)}
}

type PayloadFacet struct {
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

func (p PayloadFacet) Columns() []string {
	columns := make([]string, len(p.Metadata.Contents.Contents)+1)
	columns[0] = p.Metadata.Facet
	for i, content := range p.Metadata.Contents.Contents {
		columns[i+1] = content.Function
		if content.Function == "alias" {
			columns[i+1] = content.Alias
		}
	}
	return columns
}

func (p PayloadFacet) Rows() [][]interface{} {
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
func unmarshalPayload(data []byte) (Payload, error) {
	// Allocate 3 mutually exclusive payload instances; exactly one of these
	// should match the JSON payload. This is a hack, but I can't think of a
	// better way to cope with NewRelic's wonky API.
	var basic PayloadBasic
	var aggregation PayloadAggregation
	var facet PayloadFacet

	var basicErr error
	if basicErr = json.Unmarshal(data, &basic); basicErr == nil {
		if basic.Results[0].Events != nil {
			return &basic, nil
		}
		basicErr = fmt.Errorf("missing 'results[0].events' field")
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
