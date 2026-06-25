// Copyright Contributors to the Open Cluster Management project

package version

import (
	"fmt"
	"os"
	"testing"
)

func Test_Get(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{
			name: "should return information relatef to runtime version",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := Get()

			if i.GitVersion == "" {
				t.Error("failed to return information related to git version")
			}

			if i.GitCommit == "" {
				t.Error("failed to return information related to git commit")
			}

			if i.GitTreeState == "" {
				t.Error("failed to return information related to git tree state")
			}

			if i.BuildDate == "" {
				t.Error("failed to return information related to build date")
			}

			if i.GoVersion == "" {
				t.Error("failed to return information related to go version")
			}

			if i.Compiler == "" {
				t.Error("failed to return information related to compiler")
			}

			if i.Platform == "" {
				t.Error("failed to return information related to platform")
			}
		})
	}
}

func Test_ValidOCPVersion(t *testing.T) {
	tests := []struct {
		name       string
		ocpVersion string
		envVar     string
		wantErr    bool
	}{
		{
			name:       "above min",
			ocpVersion: "4.99.99",
			wantErr:    false,
		},
		{
			name:       "below min",
			ocpVersion: "4.9.99",
			wantErr:    true,
		},
		{
			name:       "below min ignored",
			ocpVersion: "4.9.99",
			envVar:     "DISABLE_OCP_MIN_VERSION",
			wantErr:    false,
		},
		{
			name:       "no version found",
			ocpVersion: "",
			wantErr:    true,
		},
		{
			name:       "dev version passing",
			ocpVersion: fmt.Sprintf("%s-dev", MinimumOCPVersion),
			wantErr:    false,
		},
		{
			name:       "exact version",
			ocpVersion: MinimumOCPVersion,
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVar != "" {
				os.Setenv(tt.envVar, "true")
				defer os.Unsetenv(tt.envVar)
			}
			if err := ValidOCPVersion(tt.ocpVersion); (err != nil) != tt.wantErr {
				t.Errorf("validOCPVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_ValidMCEVersion(t *testing.T) {
	tests := []struct {
		name       string
		mceVersion string
		envVar     string
		wantErr    bool
	}{
		{
			name:       "above min",
			mceVersion: "4.99.99",
			wantErr:    false,
		},
		{
			name:       "below min",
			mceVersion: "2.1.11",
			wantErr:    true,
		},
		{
			name:       "below min ignored",
			mceVersion: "2.1.11",
			envVar:     "DISABLE_MCE_MIN_VERSION",
			wantErr:    false,
		},
		{
			name:       "no version found",
			mceVersion: "",
			wantErr:    true,
		},
		{
			name:       "dev version passing",
			mceVersion: fmt.Sprintf("%s-dev", RequiredMCEVersion),
			wantErr:    false,
		},
		{
			name:       "exact version",
			mceVersion: RequiredMCEVersion,
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVar != "" {
				os.Setenv(tt.envVar, "true")
				defer os.Unsetenv(tt.envVar)
			}
			if err := ValidMCEVersion(tt.mceVersion); (err != nil) != tt.wantErr {
				t.Errorf("ValidMCEVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_ValidCommunityMCEVersion(t *testing.T) {
	tests := []struct {
		name       string
		mceVersion string
		envVar     string
		wantErr    bool
	}{
		{
			name:       "above min",
			mceVersion: "4.99.99",
			wantErr:    false,
		},
		{
			name:       "below min",
			mceVersion: "0.0.1",
			wantErr:    true,
		},
		{
			name:       "below min ignored",
			mceVersion: "2.1.11",
			envVar:     "DISABLE_MCE_MIN_VERSION",
			wantErr:    false,
		},
		{
			name:       "no version found",
			mceVersion: "",
			wantErr:    true,
		},
		{
			name:       "dev version passing",
			mceVersion: fmt.Sprintf("%s-dev", RequiredCommunityMCEVersion),
			wantErr:    false,
		},
		{
			name:       "exact version",
			mceVersion: RequiredCommunityMCEVersion,
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVar != "" {
				os.Setenv(tt.envVar, "true")
				defer os.Unsetenv(tt.envVar)
			}
			if err := ValidCommunityMCEVersion(tt.mceVersion); (err != nil) != tt.wantErr {
				t.Errorf("ValidCommunityMCEVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
