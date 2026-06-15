# go generate RBAC Resource Errors

## Symptom

`go generate ./...` fails with panic:

```text
panic: resource rbac.open-cluster-management.io/<ResourceName> not accounted for in RBAC generation
```

## Root Cause

New custom resource added to Helm charts but not registered in RBAC generation script. `pkg/templates/rbac.go` maintains allowlist of known resource kinds.

## Resolution

Add missing resource to `resources` slice in `pkg/templates/rbac.go`.

Example - adding `MulticlusterRoleAssignment`:

```go
var resources = []string{
    "ManagedProxyServiceResolver",
    "MulticlusterRoleAssignment",  // Add alphabetically
    "MutatingWebhookConfiguration",
    // ...
}
```

Then rerun `go generate ./...`.

## Prevention

When adding new CRDs or Kubernetes resources to toggle charts, verify resource kind exists in rbac.go resources list before running generation.
