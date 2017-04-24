package nrql

import (
	"encoding/json"
	"io"
)

func FormatJSON(w io.Writer, p Payload) error {
	data, err := json.Marshal(struct {
		Columns []string
		Rows    [][]interface{}
	}{
		Columns: p.Columns(),
		Rows:    p.Rows(),
	})
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
