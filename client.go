package nrql

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type Client struct {
	AccountID string
	QueryKey  string
}

func execRaw(accountID, queryKey, nrql string) (Payload, error) {
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

func (c Client) Exec(q Query) (Payload, error) {
	return execRaw(c.AccountID, c.QueryKey, q.String())
}

func (c Client) ExecRaw(nrql string) (Payload, error) {
	return execRaw(c.AccountID, c.QueryKey, nrql)
}
