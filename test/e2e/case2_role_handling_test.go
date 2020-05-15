// Copyright (c) 2020 Red Hat, Inc.

package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/config-policy-controller/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const case2ConfigPolicyName string = "policy-role-create"
const case2roleName string = "pod-reader-e2e"
const case2PolicyYaml string = "../resources/case2_role_handling/case2_role_create.yaml"
const case2PolicyCheckMNHYaml string = "../resources/case2_role_handling/case2_role_check-mnh.yaml"

var _ = Describe("Test role obj template handling", func() {
	Describe("Create a policy on managed cluster in ns:"+testNamespace, func() {
		It("should be created properly on the managed cluster", func() {
			By("Creating " + case2PolicyYaml + " on managed")
			utils.Kubectl("apply", "-f", case2PolicyYaml, "-n", testNamespace)
			plc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case2ConfigPolicyName, testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case2ConfigPolicyName, testNamespace, true, defaultTimeoutSeconds)
				return utils.GetComplianceState(managedPlc)
			}, defaultTimeoutSeconds, 1).Should(Equal("NonCompliant"))
		})
		It("should create role on managed cluster", func() {
			By("Patching " + case2PolicyYaml + " on hub with spec.remediationAction = enforce")
			managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case2ConfigPolicyName, testNamespace, true, defaultTimeoutSeconds)
			Expect(managedPlc).NotTo(BeNil())
			Expect(managedPlc.Object["spec"].(map[string]interface{})["remediationAction"]).To(Equal("inform"))
			managedPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "enforce"
			managedPlc, err := clientManagedDynamic.Resource(gvrPolicy).Namespace(testNamespace).Update(managedPlc, metav1.UpdateOptions{})
			Expect(err).To(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case2ConfigPolicyName, testNamespace, true, defaultTimeoutSeconds)
				return utils.GetComplianceState(managedPlc)
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
			role := utils.GetWithTimeout(clientManagedDynamic, gvrRole, case2roleName, testNamespace, true, defaultTimeoutSeconds)
			Expect(role).NotTo(BeNil())
		})
		It("should create violations properly", func() {
			By("Creating " + case2PolicyYaml + " on managed")
			utils.Kubectl("apply", "-f", case2PolicyCheckMNHYaml, "-n", testNamespace)
			plc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, "policy-role-check-mnh", testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, "policy-role-check-mnh", testNamespace, true, defaultTimeoutSeconds)
				return utils.GetComplianceState(managedPlc)
			}, defaultTimeoutSeconds, 1).Should(Equal("NonCompliant"))
			utils.Kubectl("apply", "-f", case2PolicyCheckMNHYaml, "-n", testNamespace)
			plc = utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, "policy-role-check-moh", testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, "policy-role-check-moh", testNamespace, true, defaultTimeoutSeconds)
				return utils.GetComplianceState(managedPlc)
			}, defaultTimeoutSeconds, 1).Should(Equal("NonCompliant"))
		})
	})
})
