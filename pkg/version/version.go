// Copyright Contributors to the Open Cluster Management project

package version

import (
	"fmt"
	"os"
	"runtime"

	"github.com/Masterminds/semver/v3"
)

// Version is the semver version the operator is reconciling towards
var Version string

// MinimumOCPVersion is the minimum version of OCP this operator supports.
// Can be overridden by setting the env variable DISABLE_OCP_MIN_VERSION
var MinimumOCPVersion string = "4.10.0"

// RequiredMCEVersion is the minimum version of MCE this operator expects.
// The reconciler will wait until MCE has installed to at least this version
// before proceeding with installing ACM.
var RequiredMCEVersion = "2.9.0"
var RequiredCommunityMCEVersion = "0.7.0"

func init() {
	if value, exists := os.LookupEnv("OPERATOR_VERSION"); exists {
		Version = value
	} else {
		Version = "9.9.9"
	}
}

// Info contains versioning information.
type Info struct {
	GitVersion   string `json:"gitVersion"`
	GitCommit    string `json:"gitCommit"`
	GitTreeState string `json:"gitTreeState"`
	BuildDate    string `json:"buildDate"`
	GoVersion    string `json:"goVersion"`
	Compiler     string `json:"compiler"`
	Platform     string `json:"platform"`
}

// Get returns the overall codebase version. It's for detecting
// what code a binary was built from.
func Get() Info {
	// These variables typically come from -ldflags settings and in
	// their absence fallback to the settings in pkg/version/base.go
	return Info{
		GitVersion:   gitVersion,
		GitCommit:    gitCommit,
		GitTreeState: gitTreeState,
		BuildDate:    buildDate,
		GoVersion:    runtime.Version(),
		Compiler:     runtime.Compiler,
		Platform:     fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// ValidMCEVersion returns an error if MCE does not satisfy the minimum version requirement
func ValidMCEVersion(mceVersion string) error {
	if _, exists := os.LookupEnv("DISABLE_MCE_MIN_VERSION"); exists {
		return nil
	}
	return validVersion(mceVersion, RequiredMCEVersion)
}

// ValidCommunityMCEVersion returns an error if MCE does not satisfy the minimum version requirement
// when running in community mode
func ValidCommunityMCEVersion(mceVersion string) error {
	if _, exists := os.LookupEnv("DISABLE_MCE_MIN_VERSION"); exists {
		return nil
	}
	return validVersion(mceVersion, RequiredCommunityMCEVersion)
}

// ValidOCPVersion returns an error if ocpVersion does not satisfy the minimum OCP version requirement
func ValidOCPVersion(ocpVersion string) error {
	if _, exists := os.LookupEnv("DISABLE_OCP_MIN_VERSION"); exists {
		return nil
	}
	return validVersion(ocpVersion, MinimumOCPVersion)
}

// validVersion checks that "have" is semantically greater than "required", which should be in the form 'x.y.z'
func validVersion(have, required string) error {
	aboveMinVersion, err := semver.NewConstraint(fmt.Sprintf(">= %s-0", required))
	if err != nil {
		return err
	}
	currentVersion, err := semver.NewVersion(have)
	if err != nil {
		return err
	}
	if !aboveMinVersion.Check(currentVersion) {
		return fmt.Errorf("version %s did not meet minimum version requirement of %s", have, required)
	}
	return nil
}
