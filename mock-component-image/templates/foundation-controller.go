// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package main

import (
	"fmt"
	"net/http"
	"os/exec"
)

func MockPing(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, %s!", r.URL.Path[1:])
}

func main() {
	http.HandleFunc("/healthz", MockPing)
	http.HandleFunc("/readyz", MockPing)

	cmd := exec.Command("./bin/kubectl", "apply", "-k", "templates/resources/")
	stdout, err := cmd.Output()

	if err != nil {
		fmt.Println(err.Error())
		return
	}
	// Print the output
	fmt.Println(string(stdout))

	http.ListenAndServe(":8000", nil)
}
