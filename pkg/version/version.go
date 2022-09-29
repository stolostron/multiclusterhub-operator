// Copyright Contributors to the Open Cluster Management project

package version

import "os"

// Version is the semver version the operator is reconciling towards
var Version string

func init() {
	if value, exists := os.LookupEnv("OPERATOR_VERSION"); exists {
		Version = value
	} else {
		Version = "9.9.9"
	}
}

var RequiredMCEVersion = "2.2.0"
