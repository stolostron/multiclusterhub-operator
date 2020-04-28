// Copyright (c) 2020 Red Hat, Inc.

// Package license scans the repo for missing license or copyright headers
package license

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// slashScanner is for validating the copyright comment in Go files
var slashScanner = regexp.MustCompile(`// Copyright \(c\) 2020 Red Hat, Inc\.`)

// poundScanner is for validating the copyright comment in shell and Python files
var poundScanner = regexp.MustCompile(`\# Copyright \(c\) 2020 Red Hat, Inc\.`)

var skip = map[string]bool{
	// Operator SDK boilerplate
	"pkg/apis/operators/v1beta1/doc.go":                   true,
	"pkg/apis/operators/v1beta1/register.go":              true,
	"pkg/apis/operators/v1beta1/zz_generated.deepcopy.go": true,
	"pkg/apis/operators/group.go":                         true,
	"pkg/apis/addtoscheme_operators_v1beta1.go":           true,
	"pkg/apis/apis.go":                                    true,
	"tools.go":                                            true,

	// Build Harness
	"vbh": true,
}

func TestLicense(t *testing.T) {
	// Run from base dir instead of package dir
	os.Chdir("..")
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if skip[path] {
			fmt.Printf("skipping file or dir: %q\n", path)
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if err != nil {
			return err
		}

		// Capture Go code, Python code, and shell scripts
		if filepath.Ext(path) != ".go" && filepath.Ext(path) != ".sh" && filepath.Ext(path) != ".py" {
			return nil
		}

		src, err := ioutil.ReadFile(path)
		if err != nil {
			return nil
		}

		// Find license
		if filepath.Ext(path) == ".go" {
			if !slashScanner.Match(src) {
				t.Errorf("%v: license header not present", path)
				return nil
			}
		} else {
			if !poundScanner.Match(src) {
				t.Errorf("%v: license header not present", path)
				return nil
			}
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
