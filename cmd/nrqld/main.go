package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	nrql "github.com/ns-cweber/nrql2csv"
)

type NRQLDaemon struct {
	nrql.Client
}

func (d NRQLDaemon) handleRequest(w io.Writer, qstring string) (int, error) {
	log.Println("Executing query:", qstring)
	p, err := d.Client.ExecRaw(qstring)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	if err := nrql.FormatCSV(w, p); err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}

func (d NRQLDaemon) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if st, err := d.handleRequest(w, r.URL.Query().Get("nrql")); err != nil {
		http.Error(w, http.StatusText(st), st)
		log.Println(st, err)
		return
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	if accountID == "" {
		fmt.Fprintln(os.Stderr, "Missing $NEW_RELIC_ACCOUNT_ID")
		os.Exit(-1)
	}

	queryKey := os.Getenv("NEW_RELIC_QUERY_KEY")
	if queryKey == "" {
		fmt.Fprintln(os.Stderr, "Missing $NEW_RELIC_QUERY_KEY")
		os.Exit(-1)
	}

	log.Println("Listening at", addr)
	if err := http.ListenAndServe(
		addr,
		NRQLDaemon{nrql.Client{AccountID: accountID, QueryKey: queryKey}},
	); err != nil {
		log.Fatal(err)
	}
}
