package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"unicode"

	nrql "github.com/ns-cweber/nrql2csv"
)

func trim(s string) string {
	return strings.TrimFunc(s, unicode.IsSpace)
}

func parseFlags() (nrql.Query, []nrql.StaticColumn) {
	var q nrql.Query
	var columns string
	var static string
	var dry bool
	flag.StringVar(
		&columns,
		"select",
		"",
		"[OPTIONAL] the comma-delineated column names to query for",
	)
	flag.StringVar(&q.Table, "from", "", "[REQUIRED] the table to query from")
	flag.StringVar(&q.Where, "where", "", "[OPTIONAL] the WHERE clause")
	flag.StringVar(&q.Since, "since", "", "[OPTIONAL] the SINCE clause")
	flag.StringVar(&q.Until, "until", "", "[OPTIONAL] the UNTIL clause")
	flag.StringVar(&q.Facet, "facet", "", "[OPTIONAL] the FACET column")
	flag.StringVar(
		&static,
		"static",
		"",
		"[OPTIONAL] extra fixed-value columns (e.g., 'col1=val1,col2=val2')",
	)
	flag.IntVar(&q.Limit, "limit", -1, "[OPTIONAL] the LIMIT column")
	flag.BoolVar(&dry, "dry", false, "[OPTIONAL] Prints the query")
	flag.Parse()

	if columns != "*" && columns != "" {
		for _, col := range strings.Split(columns, ",") {
			q.Columns = append(q.Columns, trim(col))
		}
	}

	if q.Table == "" {
		fmt.Fprintln(os.Stderr, "Missing --from flag")
		flag.Usage()
		os.Exit(-1)
	}

	var staticColumns []nrql.StaticColumn
	if static != "" {
		for _, column := range strings.Split(static, ",") {
			if idx := strings.IndexRune(column, '='); idx >= 0 {
				sc := nrql.StaticColumn{
					Name:  trim(column[:idx]),
					Value: trim(column[idx+1:]),
				}

				// it's ok to have an empty value, but not an empty name
				if sc.Name != "" {
					staticColumns = append(staticColumns, sc)
					continue
				}
			}

			fmt.Fprintln(os.Stderr, "Malformed static column:", column)
			flag.Usage()
			os.Exit(-1)
		}
	}

	if dry {
		fmt.Println(q.String())
		os.Exit(0)
	}

	return q, staticColumns
}

func abort(v ...interface{}) {
	fmt.Fprintln(os.Stderr, v...)
	os.Exit(-1)
}

func abortf(format string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, format, v...)
	os.Exit(-1)
}

func main() {
	// Parse the command line flags into a query structure
	q, staticColumns := parseFlags()

	// Make sure we have the account ID
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	if accountID == "" {
		abort(os.Stderr, "Missing $NEW_RELIC_ACCOUNT_ID")
	}

	// Make sure we have the query key
	// (https://docs.newrelic.com/docs/insights/export-insights-data/export-api/query-insights-event-data-api#register)
	queryKey := os.Getenv("NEW_RELIC_QUERY_KEY")
	if queryKey == "" {
		abort("Missing $NEW_RELIC_QUERY_KEY")
	}

	// Execute the query
	payload, err := nrql.Client{AccountID: accountID, QueryKey: queryKey}.Exec(q)
	if err != nil {
		abortf("Error for query '%s': %v", q, err)
	}

	// Add the static columns
	payload = nrql.StaticColumnsPayload{payload, staticColumns}

	// Format the query
	if err := nrql.FormatCSV(os.Stdout, payload); err != nil {
		abort(err)
	}
}
