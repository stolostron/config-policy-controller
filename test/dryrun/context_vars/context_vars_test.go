// Copyright (c) 2025 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package dryruntest

import (
	"embed"
	"testing"

	"open-cluster-management.io/config-policy-controller/test/dryrun"
)

var (
	//go:embed objectns_cluster_scoped
	objNsClusterScoped embed.FS
	//go:embed objectns_templated_empty
	objNsTemplatedEmpty embed.FS
	//go:embed objectns_templated_no_nsselector
	objNsTemplatedNoNsSelector embed.FS

	testCases = map[string]embed.FS{
		"Test ObjectNamespace: unavailable for cluster-scoped objects":     objNsClusterScoped,
		"Test ObjectNamespace: noncompliant for empty templated namespace": objNsTemplatedEmpty,
		"Test ObjectNamespace: available for templated namespace":          objNsTemplatedNoNsSelector,
	}
)

func TestContextVariables(t *testing.T) {
	for name, testFiles := range testCases {
		t.Run(name, dryrun.Run(testFiles))
	}
}
