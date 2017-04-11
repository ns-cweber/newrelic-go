package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

// "row" is easier to type and more expressive than "map[string]interface{}"
type row map[string]interface{}

// `query()` takes a NRQL string and returns the result set (or an error)
func query(accountID, queryKey, nrql string) ([]row, error) {
	// Build a new request
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf(
			"https://insights-api.newrelic.com/v1/accounts/%s/query?%s",
			accountID,
			url.Values{"nrql": []string{nrql}}.Encode(),
		),
		nil,
	)
	if err != nil {
		return nil, err
	}

	// Set the requisite headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Query-Key", queryKey)

	// Dispatch the request
	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close() // close the http body when done

	// Read the body into memory
	data, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	// Allocate a thing that looks vaguely like the payload. The JSON library
	// will unmarshal the JSON into this structure.
	// https://godoc.org/encoding/json#Unmarshal
	var payload struct {
		// This should only be populated if `Metadata.Facet` is not `""`
		Facets []struct {
			Name    string `json:"name"`
			Results []row  `json:"results"`
		} `json:"facets"`

		// This should only be populated if `Metadata.Facet` is `""`
		Results []struct {
			Events []row `json:"events"`
		} `json:"results"`

		// `Metadata.Facet` tells us whether to look in the `Facets` or
		// `Results` fields based on whether or not it's empty. If it's not
		// empty, its value is the name of the facet column.
		Metadata struct {
			Facet string `json:"facet"`
		} `json:"metadata"`
	}

	// Unmarshal the JSON into the payload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}

	// Collect the output rows
	var rows []row
	if payload.Metadata.Facet != "" {
		// if there is a facet, collect the facet results
		for _, facet := range payload.Facets {
			for _, result := range facet.Results {
				result[payload.Metadata.Facet] = facet.Name
				rows = append(rows, result)
			}
		}
	} else {
		// otherwise collect the normal results
		for _, result := range payload.Results {
			rows = append(rows, result.Events...)
		}
	}

	return rows, nil
}

// Take anything and figure out how to make it into a string; normally we would
// use fmt.Sprintf(), but the default formatting for floats involves
// exponentiation, which isn't awesome for CSVs.
func stringify(v interface{}) string {
	switch x := v.(type) {
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case string:
		return x
	default:
		return fmt.Sprint(x)
	}
}

// `toCSV()` writes `rows` to `w` in CSV form
func toCSV(w io.Writer, rows []row) error {
	// Bail if no rows were received
	if len(rows) < 1 {
		return nil
	}

	// Make a new CSV writer
	wr := csv.NewWriter(w)

	// Collect the headers from the first row; this order will be pseudo-
	// random, but we'll use this order for all the rows going forward
	headers := make([]string, 0, len(rows[0]))
	for header := range rows[0] {
		headers = append(headers, header)
	}

	// Write the headers to the CSV writer
	if err := wr.Write(headers); err != nil {
		return err
	}

	// Allocate a row buffer
	buffer := make([]string, len(headers))

	// For each row, copy the values into the buffer in the order specified by
	// the headers. Write the row to the CSV writer.
	for _, row := range rows {
		for i := range headers {
			buffer[i] = stringify(row[headers[i]])
		}
		if err := wr.Write(buffer); err != nil {
			return err
		}
	}

	// Flush the CSV writer and return any errors
	wr.Flush()
	return wr.Error()
}

func main() {
	// Make sure we received a query string; if not, print an error and bail
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "USAGE: %s <nrql>\n", os.Args[0])
		os.Exit(-1)
	}

	// Make sure we have the account ID
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	if accountID == "" {
		fmt.Fprintln(os.Stderr, "Missing $NEW_RELIC_ACCOUNT_ID")
		os.Exit(-1)
	}

	// Make sure we have the query key
	// (https://docs.newrelic.com/docs/insights/export-insights-data/export-api/query-insights-event-data-api#register)
	queryKey := os.Getenv("NEW_RELIC_QUERY_KEY")
	if queryKey == "" {
		fmt.Fprintln(os.Stderr, "Missing $NEW_RELIC_QUERY_KEY")
		os.Exit(-1)
	}

	// Execute the query
	rows, err := query(accountID, queryKey, os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	// Format the query
	if err := toCSV(os.Stdout, rows); err != nil {
		log.Fatal(err)
	}
}
