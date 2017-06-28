package nrql

import (
	"strconv"
	"strings"
)

type Query struct {
	// A `nil` columns slice denotes `*`
	Columns []string
	Table   string
	Where   string
	Since   string
	Until   string
	Facet   string
	Limit   int
}

func (q Query) String() string {
	columns := strings.Join(q.Columns, ", ")
	if columns == "" {
		columns = "*"
	}

	var where string
	if q.Where != "" {
		where = " WHERE " + q.Where
	}

	var facet string
	if q.Facet != "" {
		facet = " FACET " + q.Facet
	}

	var limit string
	if q.Limit >= 0 {
		limit = " LIMIT " + strconv.Itoa(q.Limit)
	}

	var since string
	if q.Since != "" {
		since = " SINCE '" + q.Since + "'"
	}

	var until string
	if q.Until != "" {
		until = " UNTIL '" + q.Until + "'"
	}

	return "SELECT " + columns + " FROM " + q.Table + where + since + until +
		facet + limit
}
