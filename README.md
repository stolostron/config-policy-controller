# Configuration Policy Controller
Red Hat Advanced Cluster Management - Governance - Configuration Policy Controller

## How it works

This operator watches for the following changes to trigger reconcile:

1. ConfigurationPolicy changes in all watched namespaces on hub

Every reconcile:

1. Create/update/delete the replicated policy on the managed cluster in the cluster namespace
2. Handles the object template specified in the ConfigurationPolicy and creates an object and/or status update depending on the details of the object template

## Run

To run the controller locally, point your CLI to a running cluster and then run:
```
export WATCH_NAMESPACE=cluster_namespace_on_hub
go run cmd/manager/main.go
```
<!---
Date: 9/09/2020
-->