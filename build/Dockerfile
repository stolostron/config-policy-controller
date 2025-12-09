# Copyright Contributors to the Open Cluster Management project

# Stage 1: Use image builder to build the target binaries
FROM registry.ci.openshift.org/stolostron/builder:go1.25-linux AS builder

ENV COMPONENT=config-policy-controller
ENV REPO_PATH=/go/src/github.com/stolostron/${COMPONENT}
WORKDIR ${REPO_PATH}
COPY . .
RUN make build

# Stage 2: Copy the binaries from the image builder to the base image
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

ENV COMPONENT=config-policy-controller
ENV REPO_PATH=/go/src/github.com/stolostron/${COMPONENT}
ENV OPERATOR=/usr/local/bin/${COMPONENT} \
    USER_UID=1001 \
    USER_NAME=${COMPONENT}

# install operator binary
COPY --from=builder ${REPO_PATH}/build/_output/bin/${COMPONENT} ${OPERATOR}

COPY --from=builder ${REPO_PATH}/build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint", "controller"]

USER ${USER_UID}

LABEL name="rhacm2/config-policy-controller-rhel9"
LABEL summary="Audit and enforce cluster configuration"
LABEL description="The ConfigurationPolicy kind compares the desired object defined in the policy with the objects on the cluster. \
    The OperatorPolicy kind determines whether operators deployed on the cluster match the configuration in the policy."
LABEL io.k8s.display-name="Configuration policy controller"
LABEL io.k8s.description="The ConfigurationPolicy kind compares the desired object defined in the policy with the objects on the cluster. \
    The OperatorPolicy kind determines whether operators deployed on the cluster match the configuration in the policy."
LABEL com.redhat.component="acm-config-policy-controller-container"
LABEL io.openshift.tags="data,images"
