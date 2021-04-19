[comment]: # ( Copyright Contributors to the Open Cluster Management project )

# Configuration Policy Controller 
[![License](https://img.shields.io/:license-apache-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0.html)[![KinD tests](https://github.com/open-cluster-management/config-policy-controller/actions/workflows/kind.yml/badge.svg?branch=main&event=push)](https://github.com/open-cluster-management/config-policy-controller/actions/workflows/kind.yml)

## What is Configuration Policy Controller?

Open Cluster Management - Configuration Policy Controller

The Configuration Policy Controller monitors `ConfigurationPolicy` kubenetese resource for the following triggers to execute a reconcile:

1. `ConfigurationPolicy` changes in all watched namespaces on the hub cluster

Every reconcile the controller will:

1. Handle the object template specified in the ConfigurationPolicy and create an object and/or status update depending on the details of the object template
2. If using with [Governance Policy Framework](https://github.com/open-cluster-management/governance-policy-framework), it will also generate an kuberenets event on parent `Policy` to report its compliance status

Below is an example of `ConfigurationPolicy` resource:

```yaml
apiVersion: policy.open-cluster-management.io/v1
kind: ConfigurationPolicy
metadata:
  name: policy-role
  namespace: default
spec:
  complianceType: mustnothave                         # musthave/mustnothave
  remediationAction: inform                           # inform/enforce
  namespaceSelector:                                  # use `namespaceSelector` if the desired resource check should happen in multiple namespaces
    exclude: ["kube-*"]
    include: ["default"]
  object-templates:
    - complianceType: mustonlyhave                    # musthave/mustnothave/mustonlyhave
      objectDefinition:
        apiVersion: rbac.authorization.k8s.io/v1
        kind: Role
        metadata:
          name: pod-reader-thur
        rules:
          - apiGroups: ["extensions", "apps"]
            resources: ["deployments"]
            verbs: ["get", "list", "watch", "delete","patch"]
```

## Getting started

- Steps for development: 

    To run the controller locally, point your CLI to a running cluster and then run:
    ```
    export WATCH_NAMESPACE=cluster_namespace_on_hub
    go run cmd/manager/main.go
    ```


- Steps for deployment:

- Steps for test:

- Check the [Security guide](SECURITY.md) if you need to report a security issue.


<!---
Date: 9/09/2020
-->
