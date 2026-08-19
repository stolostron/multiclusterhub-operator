# Copyright Contributors to the Open Cluster Management project

# Build the multiclusterhub-operator binary
FROM golang:1.26 as builder

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
COPY pkg/templates/ templates/

# Version information for ldflags
ARG GIT_VERSION="v0.0.1-alpha.0"
ARG GIT_COMMIT="unknown"
ARG GIT_TREE_STATE="unknown"
ARG BUILD_DATE="unknown"
ARG VERSION_PKG=github.com/stolostron/multiclusterhub-operator/pkg/version

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a \
    -ldflags "-X ${VERSION_PKG}.gitVersion=${GIT_VERSION} \
              -X ${VERSION_PKG}.gitCommit=${GIT_COMMIT} \
              -X ${VERSION_PKG}.gitTreeState=${GIT_TREE_STATE} \
              -X ${VERSION_PKG}.buildDate=${BUILD_DATE}" \
    -o multiclusterhub-operator main.go

# Use ubi minimal base image to package the multiclusterhub-operator binary
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
WORKDIR /
COPY --from=builder /workspace/multiclusterhub-operator /usr/local/bin/multiclusterhub-operator
COPY --from=builder /workspace/templates/ /usr/local/templates/

USER 65532:65532

ENTRYPOINT ["multiclusterhub-operator"]
