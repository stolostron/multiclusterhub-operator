FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ENV OPERATOR=/usr/local/bin/multiclusterhub-operator \
    USER_UID=1001 \
    USER_NAME=multiclusterhub-operator

LABEL org.label-schema.vendor="Red Hat" \
      org.label-schema.name="multiclusterhub-operator" \
      org.label-schema.description="Installer operator for Red Hat Advanced Cluster Management" \
      org.label-schema.license="Red Hat Advanced Cluster Management for Kubernetes EULA"

# install manifest files
COPY image-manifests ${HOME}/image-manifests
COPY COMPONENT_VERSION ${HOME}/COMPONENT_VERSION

# install templates
COPY templates /usr/local/templates

# install operator binary
COPY build/_output/bin/multiclusterhub-operator ${OPERATOR}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
