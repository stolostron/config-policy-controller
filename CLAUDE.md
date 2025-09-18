# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

This is the Open Cluster Management Configuration Policy Controller, a Kubernetes controller that manages `ConfigurationPolicy` and `OperatorPolicy` resources to ensure desired cluster state compliance.

## Development Commands

### Building and Running
```bash
# Build the controller binary
make build

# Run controller locally against current kubectl context
export WATCH_NAMESPACE=<namespace>
make run

# Build container images
make build-images
```

### Testing
```bash
# Install test dependencies
make test-dependencies

# Run unit tests
make test

# Run unit tests with coverage
make test-coverage

# Run linting (configured in build/common/Makefile.common.mk)
make lint

# Run E2E tests (requires KinD cluster)
make kind-bootstrap-cluster-dev
export WATCH_NAMESPACE=managed
make e2e-test
```

### KinD Development Environment
```bash
# Bootstrap complete KinD development cluster
make kind-bootstrap-cluster-dev

# Deploy controller to KinD cluster
make kind-deploy-controller-dev

# Delete KinD cluster
make kind-delete-cluster

# Full test cycle with KinD
make kind-tests
```

### Deployment
```bash
# Deploy to current cluster context
make deploy

# Generate manifests and operator YAML
make manifests
make generate-operator-yaml
```

## Architecture

### Core Components

**API Types (`api/`):**
- `v1/ConfigurationPolicy` - Main policy resource for declarative configuration management
- `v1beta1/OperatorPolicy` - Manages OLM operators through policies

**Controllers (`controllers/`):**
- `configurationpolicy_controller.go` - Reconciles ConfigurationPolicy resources, supports `musthave`, `mustnothave`, `mustonlyhave` compliance types
- `operatorpolicy_controller.go` - Reconciles OperatorPolicy resources for OLM operator lifecycle management
- `hubtemplates.go` - Handles template processing for hub-managed clusters

**Core Packages (`pkg/`):**
- `pkg/common/` - Shared utilities across controllers
- `pkg/dryrun/` - Dry-run functionality for policy evaluation
- `pkg/mappings/` - Resource mapping utilities
- `pkg/triggeruninstall/` - Handles controller uninstallation triggers

### Key Features

**Template Processing:**
- Supports Go text templates with custom functions: `fromSecret`, `fromConfigMap`, `fromClusterClaim`, `lookup`
- Templates resolved at runtime using target cluster configuration
- Hub templates support for multi-cluster scenarios

**Policy Enforcement:**
- `inform` mode: Reports compliance status without changes
- `enforce` mode: Actively creates/updates/deletes resources to achieve desired state
- Namespace selector support for scoped policy application

**Compliance Types:**
- `musthave`: Resource must exist with specified properties
- `mustnothave`: Resource must not exist
- `mustonlyhave`: Only specified resources should exist (others removed)

## Configuration

**Environment Variables:**
- `WATCH_NAMESPACE` - Namespace to monitor for policies (required)
- `KIND_NAMESPACE` - Namespace for controller deployment (default: `open-cluster-management-agent-addon`)

**Key Dependencies:**
- Uses `controller-runtime` framework for Kubernetes controllers
- OLM (Operator Lifecycle Manager) integration for OperatorPolicy
- Open Cluster Management addon-framework integration
- Supports both standalone and OCM-managed deployment modes

## Test Structure

**E2E Tests (`test/e2e/`):**
- Comprehensive scenario testing with real Kubernetes resources
- Supports different modes: standard, hosted-mode, hub-templates
- Uses Ginkgo testing framework with parallel execution

**Unit Tests:**
- Controller logic testing with envtest
- Template processing validation
- Policy compliance verification

The controller integrates with Open Cluster Management governance framework but can run standalone for single-cluster policy management.