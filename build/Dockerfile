# Set global repo path arg
ARG REPO_PATH=/go/src/github.com/open-cluster-management/config-policy-controller

# Stage 1: Use image builder to build the target binaries
FROM registry.ci.openshift.org/open-cluster-management/builder:go1.14-linux-amd64 AS builder

ENV REPO_PATH=${REPO_PATH}
WORKDIR ${REPO_PATH}
COPY . .
RUN make build

# Stage 2: Copy the binaries from the image builder to the base image
FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ENV OPERATOR=/usr/local/bin/config-policy-controller \
    USER_UID=1001 \
    USER_NAME=config-policy-controller \
    REPO_PATH=${REPO_PATH}

# install operator binary
COPY --from=builder ${REPO_PATH}/build/_output/bin/config-policy-controller ${OPERATOR}

COPY --from=builder ${REPO_PATH}/build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

RUN microdnf update && \
    microdnf clean all

USER ${USER_UID}
