package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
)

// Take anything and figure out how to make it into a string; normally we would
// use fmt.Sprintf(), but the default formatting for floats involves
// exponentiation, which isn't awesome for CSVs.
func stringify(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return ""
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

func addStaticColumns(rows [][]interface{}, staticColumns []staticColumn) {
	for _, column := range staticColumns {
		for i, row := range rows {
			rows[i] = append(row, column.value)
		}
	}
}

// `toCSV()` writes `rows` to `w` in CSV form
func toCSV(
	w io.Writer,
	staticColumns []staticColumn,
	payload payload,
) error {
	// Make a new CSV writer
	wr := csv.NewWriter(w)

	// Append static columns
	headers := payload.Columns()
	rows := payload.Rows()
	staticHeaders := make([]string, len(staticColumns))
	for i, column := range staticColumns {
		staticHeaders[i] = column.name
	}
	headers = append(headers, staticHeaders...)
	addStaticColumns(rows, staticColumns)

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
			buffer[i] = stringify(row[i])
		}
		if err := wr.Write(buffer); err != nil {
			return err
		}
	}

	// Flush the CSV writer and return any errors
	wr.Flush()
	return wr.Error()
}
