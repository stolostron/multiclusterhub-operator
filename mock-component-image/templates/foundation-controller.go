// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package main

import (
	"fmt"
	"net/http"
)

func MockPing(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, %s!", r.URL.Path[1:])
}

func main() {
	http.HandleFunc("/healthz", MockPing)
	http.HandleFunc("/readyz", MockPing)
	http.ListenAndServe(":8000", nil)
}