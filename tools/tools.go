//go:build tools

// Package tools pins tool dependencies so they are tracked in go.mod/go.sum.
// These imports are never compiled into production binaries (build tag "tools"
// is never set in normal builds) but cause `go mod tidy` to retain the entries.
package tools

import (
	_ "github.com/mikefarah/yq/v3"
	_ "github.com/onsi/ginkgo/v2/ginkgo"
)
