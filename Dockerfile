# Copyright Contributors to the Open Cluster Management project

# Build the multiclusterhub-operator binary
FROM golang:1.18 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY pkg/ pkg/

# Copy required files
COPY bin/crds crds/
COPY pkg/templates/ templates/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o multiclusterhub-operator main.go

# Use ubi minimal base image to package the multiclusterhub-operator binary
FROM registry.access.redhat.com/ubi8/ubi-minimal:latest
WORKDIR /
COPY --from=builder /workspace/multiclusterhub-operator /usr/local/bin/multiclusterhub-operator
COPY --from=builder /workspace/crds/ /crds
COPY --from=builder /workspace/templates/ /usr/local/templates/

USER 65532:65532

ENTRYPOINT ["multiclusterhub-operator"]
