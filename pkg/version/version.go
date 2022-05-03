// Copyright Contributors to the Open Cluster Management project

package version

import "os"

var Version string

func init() {
	if value, exists := os.LookupEnv("OPERATOR_VERSION"); exists {
		Version = value
	} else {
		panic("OPERATOR_VERSION not defined")
	}
}
