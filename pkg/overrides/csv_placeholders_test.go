// Copyright (c) 2026 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package overrides

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

// TestBundleCSVHasARTPlaceholders checks that the ART CSV template contains a
// dummy pullspec for every image-references operand (except the operator
// image itself) as OPERAND_IMAGE_* env vars and spec.relatedImages. ART
// search-replaces those pullspecs at bundle rebase time; the env var names
// are the image-keys MCH looks up at runtime.
func TestBundleCSVHasARTPlaceholders(t *testing.T) {
	root := repoRoot(t)

	csvPath := mustGlobOne(t, filepath.Join(root, "bundle", "manifests", "*clusterserviceversion.yaml"))
	csv := loadYAML(t, csvPath)

	container := csvContainer(t, csv)
	operatorImage := asString(t, container["image"])
	operandEnv := operandImageEnv(t, container)
	related := relatedImages(t, csv)

	refDummies := imageReferenceDummies(t, filepath.Join(root, "bundle", "image-references"))
	operandDummies := map[string]string{}
	for name, dummy := range refDummies {
		if dummy == operatorImage {
			continue
		}
		operandDummies[name] = dummy
	}

	if len(operandEnv) != len(operandDummies) {
		t.Errorf("OPERAND_IMAGE count %d != image-references operands %d", len(operandEnv), len(operandDummies))
	}
	if len(related) != len(operandEnv) {
		t.Errorf("relatedImages count %d != OPERAND_IMAGE count %d", len(related), len(operandEnv))
	}

	for _, err := range validateOperandIdentity(operandEnv, related, operandDummies) {
		t.Error(err)
	}

	helmKeys := helmImageOverrideKeys(t, filepath.Join(root, "pkg", "templates"))
	for _, key := range helmKeys {
		if operandEnv[key] == "" {
			t.Errorf("helm imageOverrides.%s has no OPERAND_IMAGE_%s in the CSV", key, strings.ToUpper(key))
		}
	}
}

func TestValidateOperandIdentityRejectsSwappedPullspecs(t *testing.T) {
	cliDummy := "image-registry.example/open-cluster-management/acm-cli-rhel9:latest"
	consoleDummy := "image-registry.example/open-cluster-management/console-rhel9:latest"
	operandDummies := map[string]string{
		"acm-cli-rhel9": cliDummy,
		"console-rhel9": consoleDummy,
	}
	// Same pullspecs, swapped across OPERAND_IMAGE keys and relatedImages.
	operandEnv := map[string]string{
		"acm_cli": consoleDummy,
		"console": cliDummy,
	}
	related := map[string]string{
		"acm_cli": consoleDummy,
		"console": cliDummy,
	}

	errs := validateOperandIdentity(operandEnv, related, operandDummies)
	if len(errs) == 0 {
		t.Fatal("swapped OPERAND_IMAGE pullspecs passed validation; component identity must be checked")
	}
}

func TestValidateOperandIdentityAcceptsMatchingKeys(t *testing.T) {
	cliDummy := "image-registry.example/open-cluster-management/acm-cli-rhel9:latest"
	consoleDummy := "image-registry.example/open-cluster-management/console-rhel9:latest"
	operandDummies := map[string]string{
		"acm-cli-rhel9": cliDummy,
		"console-rhel9": consoleDummy,
	}
	operandEnv := map[string]string{
		"acm_cli": cliDummy,
		"console": consoleDummy,
	}
	related := map[string]string{
		"acm_cli": cliDummy,
		"console": consoleDummy,
	}

	if errs := validateOperandIdentity(operandEnv, related, operandDummies); len(errs) != 0 {
		t.Fatalf("expected matching component identity to pass, got %v", errs)
	}
}

func TestComponentKeyFromImageRefName(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"acm-cli-rhel9", "acm_cli"},
		{"acm-must-gather-rhel9", "acm_must_gather"},
		{"acm-grafana-rhel9", "grafana"},
		{"acm-prometheus-rhel9", "prometheus_operator"},
		{"acm-search-v2-rhel9", "search_v2_operator"},
		{"acm-siteconfig-rhel9", "siteconfig_operator"},
		{"cluster-backup-rhel9-operator", "cluster_backup_controller"},
		{"obo-prometheus-rhel9-operator", "obo_prometheus_rhel9_operator"},
		{"endpoint-monitoring-rhel9-operator", "endpoint_monitoring_operator"},
		{"observatorium-rhel9-operator", "observatorium_operator"},
		{"configmap_reloader", "configmap_reloader"},
		{"postgresql_16", "postgresql_16"},
		{"console-rhel9", "console"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := componentKeyFromImageRefName(tt.name); got != tt.key {
				t.Errorf("componentKeyFromImageRefName(%q)=%q, want %q", tt.name, got, tt.key)
			}
		})
	}
}

// imageRefComponentKeys maps image-references tag names whose delivery-repo
// spelling does not match the extras image-key used as the OPERAND_IMAGE
// suffix, relatedImages name, and helm imageOverrides key.
var imageRefComponentKeys = map[string]string{
	"acm-cli-rhel9":                 "acm_cli",
	"acm-must-gather-rhel9":         "acm_must_gather",
	"acm-prometheus-rhel9":          "prometheus_operator",
	"acm-search-v2-rhel9":           "search_v2_operator",
	"acm-siteconfig-rhel9":          "siteconfig_operator",
	"cluster-backup-rhel9-operator": "cluster_backup_controller",
	"obo-prometheus-rhel9-operator": "obo_prometheus_rhel9_operator",
}

// componentKeyFromImageRefName maps an image-references tag name to the
// component key produced by parseEnvVarByPrefix for OPERAND_IMAGE_*.
func componentKeyFromImageRefName(name string) string {
	if key, ok := imageRefComponentKeys[name]; ok {
		return key
	}
	s := strings.ReplaceAll(name, "-rhel9-", "-")
	s = strings.TrimSuffix(s, "-rhel9")
	s = strings.ReplaceAll(s, "-", "_")
	return strings.TrimPrefix(s, "acm_")
}

// validateOperandIdentity checks OPERAND_IMAGE env and relatedImages against
// image-references by component key, not just pullspec membership.
func validateOperandIdentity(operandEnv, related, operandDummies map[string]string) []string {
	var errs []string
	seenKeys := map[string]string{}
	for name, dummy := range operandDummies {
		key := componentKeyFromImageRefName(name)
		if prev, ok := seenKeys[key]; ok {
			errs = append(errs, fmt.Sprintf("image-references %q and %q both map to component key %q", prev, name, key))
		}
		seenKeys[key] = name

		envDummy, ok := operandEnv[key]
		if !ok {
			errs = append(errs, fmt.Sprintf("CSV is missing OPERAND_IMAGE_%s for image-references %s", strings.ToUpper(key), name))
		} else if envDummy != dummy {
			errs = append(errs, fmt.Sprintf("OPERAND_IMAGE_%s=%s, want image-references dummy %s (%s)", strings.ToUpper(key), envDummy, dummy, name))
		}

		relatedDummy, ok := related[key]
		if !ok {
			errs = append(errs, fmt.Sprintf("relatedImages missing name %q (from image-references %s)", key, name))
		} else if relatedDummy != dummy {
			errs = append(errs, fmt.Sprintf("relatedImages[%s]=%s, want %s (image-references %s)", key, relatedDummy, dummy, name))
		}
	}

	for key := range operandEnv {
		if _, ok := seenKeys[key]; !ok {
			errs = append(errs, fmt.Sprintf("OPERAND_IMAGE_%s is not mapped from any image-references tag", strings.ToUpper(key)))
		}
	}
	return errs
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}

func mustGlobOne(t *testing.T, pattern string) string {
	t.Helper()
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob %s: %v", pattern, err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match for %s, got %v", pattern, matches)
	}
	return matches[0]
}

func loadYAML(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	out := map[string]interface{}{}
	if err := yaml.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return out
}

func csvContainer(t *testing.T, csv map[string]interface{}) map[string]interface{} {
	t.Helper()
	deployments := mustSlice(t, mustMap(t, mustMap(t, mustMap(t, csv["spec"])["install"])["spec"])["deployments"])
	if len(deployments) == 0 {
		t.Fatal("CSV has no deployments")
	}
	containers := mustSlice(t, mustMap(t, mustMap(t, mustMap(t, mustMap(t, deployments[0])["spec"])["template"])["spec"])["containers"])
	if len(containers) == 0 {
		t.Fatal("CSV deployment has no containers")
	}
	return mustMap(t, containers[0])
}

func operandImageEnv(t *testing.T, container map[string]interface{}) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, item := range mustSlice(t, container["env"]) {
		env := mustMap(t, item)
		name := asString(t, env["name"])
		if !strings.HasPrefix(name, OperandImagePrefix) {
			continue
		}
		key := strings.ToLower(strings.TrimPrefix(name, OperandImagePrefix))
		out[key] = asString(t, env["value"])
	}
	return out
}

func relatedImages(t *testing.T, csv map[string]interface{}) map[string]string {
	t.Helper()
	out := map[string]string{}
	raw, ok := mustMap(t, csv["spec"])["relatedImages"]
	if !ok {
		return out
	}
	for _, item := range mustSlice(t, raw) {
		ri := mustMap(t, item)
		out[asString(t, ri["name"])] = asString(t, ri["image"])
	}
	return out
}

func imageReferenceDummies(t *testing.T, path string) map[string]string {
	t.Helper()
	doc := loadYAML(t, path)
	out := map[string]string{}
	for _, item := range mustSlice(t, mustMap(t, doc["spec"])["tags"]) {
		tag := mustMap(t, item)
		from := mustMap(t, tag["from"])
		out[asString(t, tag["name"])] = asString(t, from["name"])
	}
	if len(out) == 0 {
		t.Fatalf("no tags in %s", path)
	}
	return out
}

func helmImageOverrideKeys(t *testing.T, templatesDir string) []string {
	t.Helper()
	re := regexp.MustCompile(`imageOverrides\.([A-Za-z0-9_]+)`)
	seen := map[string]struct{}{}
	err := filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !(strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range re.FindAllStringSubmatch(string(raw), -1) {
			seen[match[1]] = struct{}{}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", templatesDir, err)
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	return keys
}

func mustMap(t *testing.T, v interface{}) map[string]interface{} {
	t.Helper()
	m, ok := v.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", v)
	}
	return m
}

func mustSlice(t *testing.T, v interface{}) []interface{} {
	t.Helper()
	s, ok := v.([]interface{})
	if !ok {
		t.Fatalf("expected slice, got %T", v)
	}
	return s
}

func asString(t *testing.T, v interface{}) string {
	t.Helper()
	s, ok := v.(string)
	if !ok {
		t.Fatalf("expected string, got %T", v)
	}
	return s
}
