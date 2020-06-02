// Copyright (c) 2020 Red Hat, Inc.

package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/config-policy-controller/test/utils"
)

const case4ConfigPolicyName string = "openshift-upgrade-channel-e2e"
const case4PolicyYaml string = "../resources/case4_clusterversion/case4_clusterversion.yaml"

var _ = Describe("Test role obj template handling", func() {
	Describe("enforce patch on unnamespaced resource clusterversion "+testNamespace, func() {
		It("should be created properly on the managed cluster", func() {
			By("Creating " + case4ConfigPolicyName + " on managed")
			utils.Kubectl("apply", "-f", case4PolicyYaml, "-n", testNamespace)
			plc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case4ConfigPolicyName, testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case4ConfigPolicyName, testNamespace, true, defaultTimeoutSeconds)
				return utils.GetComplianceState(managedPlc)
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
		})
	})
})
