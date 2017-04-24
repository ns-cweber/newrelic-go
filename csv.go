package nrql

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
		return "" // nil should be represented as the empty string
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

// `FormatCSV()` writes `payload` to `w` in CSV form.
func FormatCSV(w io.Writer, payload Payload) error {
	// Make a new CSV writer
	wr := csv.NewWriter(w)

	headers := payload.Columns()
	rows := payload.Rows()

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
