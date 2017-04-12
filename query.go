package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// "row" is easier to type and more expressive than "map[string]interface{}"
type row map[string]interface{}

type query struct {
	// A `nil` columns slice denotes `*`
	columns []string
	table   string
	where   string
	since   string
	until   string
	facet   string
	limit   int
}

func (q query) String() string {
	columns := strings.Join(q.columns, ", ")
	if columns == "" {
		columns = "*"
	}

	var where string
	if q.where != "" {
		where = " WHERE " + q.where
	}

	var facet string
	if q.facet != "" {
		facet = " FACET " + q.facet
	}

	var limit string
	if q.limit >= 0 {
		limit = " LIMIT " + strconv.Itoa(q.limit)
	}

	var since string
	if q.since != "" {
		since = " SINCE " + q.since
	}

	var until string
	if q.until != "" {
		until = " UNTIL " + q.until
	}

	return "SELECT " + columns + " FROM " + q.table + where + since + until +
		facet + limit
}

func (q query) exec(accountID, queryKey string) ([]row, error) {
	// Build a new request
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf(
			"https://insights-api.newrelic.com/v1/accounts/%s/query?%s",
			accountID,
			url.Values{"nrql": []string{q.String()}}.Encode(),
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

	// Check the status code
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"Wanted HTTP 200; got %d: %s",
			rsp.StatusCode,
			data,
		)
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
