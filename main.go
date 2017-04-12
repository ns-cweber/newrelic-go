package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"unicode"
)

func trim(s string) string {
	return strings.TrimFunc(s, unicode.IsSpace)
}

type staticColumn struct {
	name, value string
}

func parseFlags() (query, []staticColumn) {
	var q query
	var columns string
	var static string
	flag.StringVar(
		&columns,
		"select",
		"",
		"[OPTIONAL] the comma-delineated column names to query for",
	)
	flag.StringVar(&q.table, "from", "", "[REQUIRED] the table to query from")
	flag.StringVar(&q.where, "where", "", "[OPTIONAL] the WHERE clause")
	flag.StringVar(&q.since, "since", "", "[OPTIONAL] the SINCE clause")
	flag.StringVar(&q.until, "until", "", "[OPTIONAL] the UNTIL clause")
	flag.StringVar(&q.facet, "facet", "", "[OPTIONAL] the FACET column")
	flag.StringVar(
		&static,
		"static",
		"",
		"[OPTIONAL] extra fixed-value columns (e.g., 'col1=val1,col2=val2')",
	)
	flag.IntVar(&q.limit, "limit", -1, "[OPTIONAL] the LIMIT column")
	flag.Parse()

	if columns != "*" && columns != "" {
		for _, col := range strings.Split(columns, ",") {
			q.columns = append(q.columns, trim(col))
		}
	}

	if q.table == "" {
		fmt.Fprintln(os.Stderr, "Missing --from flag")
		flag.Usage()
		os.Exit(-1)
	}

	var staticColumns []staticColumn
	if static != "" {
		for _, column := range strings.Split(static, ",") {
			if idx := strings.IndexRune(column, '='); idx >= 0 {
				sc := staticColumn{
					name:  trim(column[:idx]),
					value: trim(column[idx+1:]),
				}

				// it's ok to have an empty value, but not an empty name
				if sc.name != "" {
					staticColumns = append(staticColumns, sc)
					continue
				}
			}

			fmt.Fprintln(os.Stderr, "Malformed static column:", column)
			flag.Usage()
			os.Exit(-1)
		}
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
	rows, err := q.exec(accountID, queryKey)
	if err != nil {
		abortf("Error for query '%s': %v", q, err)
	}

	// Format the query
	if err := toCSV(os.Stdout, q.columns, staticColumns, rows); err != nil {
		abort(err)
	}
}
