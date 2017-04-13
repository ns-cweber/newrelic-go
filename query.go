package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

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

func (q query) exec(accountID, queryKey string) (payload, error) {
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

	return unmarshalPayload(data)
}
