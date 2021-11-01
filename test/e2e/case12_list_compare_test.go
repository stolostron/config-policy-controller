// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/config-policy-controller/test/utils"
	"time"
)

const case12ConfigPolicyNameInform string = "policy-pod-mh-listinform"
const case12ConfigPolicyNameEnforce string = "policy-pod-create-listinspec"
const case12InformYaml string = "../resources/case12_list_compare/case12_pod_inform.yaml"
const case12EnforceYaml string = "../resources/case12_list_compare/case12_pod_create.yaml"

const case12ConfigPolicyNameRoleInform string = "policy-role-mh-listinform"
const case12ConfigPolicyNameRoleEnforce string = "policy-role-create-listinspec"
const case12RoleInformYaml string = "../resources/case12_list_compare/case12_role_inform.yaml"
const case12RoleEnforceYaml string = "../resources/case12_list_compare/case12_role_create.yaml"

const case12RoleToPatch string = "topatch-role-configpolicy"
const case12RoleToPatchYaml string = "../resources/case12_list_compare/case12_role_create_small.yaml"
const case12RolePatchEnforce string = "patch-role-configpolicy"
const case12RolePatchEnforceYaml string = "../resources/case12_list_compare/case12_role_patch.yaml"
const case12RolePatchInform string = "patch-role-configpolicy-inform"
const case12RolePatchInformYaml string = "../resources/case12_list_compare/case12_role_patch_inform.yaml"

const case12ByteCreate string = "policy-byte-create"
const case12ByteCreateYaml string = "../resources/case12_list_compare/case12_byte_create.yaml"
const case12ByteInform string = "policy-byte-inform"
const case12ByteInformYaml string = "../resources/case12_list_compare/case12_byte_inform.yaml"

var _ = Describe("Test list handling for musthave", func() {
	Describe("Create a policy with a nested list on managed cluster in ns:"+testNamespace, func() {
		It("should be created properly on the managed cluster", func() {
			By("Creating " + case12ConfigPolicyNameEnforce + " and " + case12ConfigPolicyNameInform + " on managed")
			utils.Kubectl("apply", "-f", case12EnforceYaml, "-n", testNamespace)
			plc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case12ConfigPolicyNameEnforce, testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case12ConfigPolicyNameEnforce, testNamespace, true, defaultTimeoutSeconds)
				return utils.GetComplianceState(managedPlc)
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
			utils.Kubectl("apply", "-f", case12InformYaml, "-n", testNamespace)
			plc = utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case12ConfigPolicyNameInform, testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case12ConfigPolicyNameInform, testNamespace, true, defaultTimeoutSeconds)
				return utils.GetComplianceState(managedPlc)
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
		})
	})
	Describe("Create a policy with a list field on managed cluster in ns:"+testNamespace, func() {
		It("should be created properly on the managed cluster", func() {
			By("Creating " + case12ConfigPolicyNameRoleEnforce + " and " + case12ConfigPolicyNameRoleInform + " on managed")
			utils.Kubectl("apply", "-f", case12RoleEnforceYaml, "-n", testNamespace)
			plc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case12ConfigPolicyNameRoleEnforce, testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case12ConfigPolicyNameRoleEnforce, testNamespace, true, defaultTimeoutSeconds)
				return utils.GetComplianceState(managedPlc)
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
			utils.Kubectl("apply", "-f", case12RoleInformYaml, "-n", testNamespace)
			plc = utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case12ConfigPolicyNameRoleInform, testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case12ConfigPolicyNameRoleInform, testNamespace, true, defaultTimeoutSeconds)
				return utils.GetComplianceState(managedPlc)
			}, defaultTimeoutSeconds, 1).Should(Equal("NonCompliant"))
		})
	})
	Describe("Create and patch a role on managed cluster in ns:"+testNamespace, func() {
		It("should be created properly on the managed cluster", func() {
			By("Creating " + case12RoleToPatch + " and " + case12RolePatchEnforce + " on managed")
			utils.Kubectl("apply", "-f", case12RoleToPatchYaml, "-n", testNamespace)
			plc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case12RoleToPatch, testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case12RoleToPatch, testNamespace, true, defaultTimeoutSeconds)
				return utils.GetComplianceState(managedPlc)
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
			utils.Kubectl("apply", "-f", case12RolePatchEnforceYaml, "-n", testNamespace)
			plc = utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case12RolePatchEnforce, testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case12RolePatchEnforce, testNamespace, true, defaultTimeoutSeconds)
				return utils.GetComplianceState(managedPlc)
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
			utils.Kubectl("apply", "-f", case12RolePatchInformYaml, "-n", testNamespace)
			plc = utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case12RolePatchInform, testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case12RolePatchInform, testNamespace, true, defaultTimeoutSeconds)
				return utils.GetComplianceState(managedPlc)
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
		})
	})
	Describe("Create a statefulset object with a byte quantity field on managed cluster in ns:"+testNamespace, func() {
		It("should only add the list item with the rounded byte value once", func() {
			By("Creating " + case12ByteCreate + " and " + case12ByteInform + " on managed")
			utils.Kubectl("apply", "-f", case12ByteCreateYaml, "-n", testNamespace)
			plc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case12ByteCreate, testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case12ByteCreate, testNamespace, true, defaultTimeoutSeconds)
				return utils.GetComplianceState(managedPlc)
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
			// Ensure it remains compliant for a while - need to ensure there were multiple enforce checks/attempts.
			Consistently(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case12ByteCreate, testNamespace, true, defaultTimeoutSeconds)
				return utils.GetComplianceState(managedPlc)
			}, time.Second*20, 1).Should(Equal("Compliant"))

			// Verify that the conatiner list and its environment variable list is correct (there are no duplicates)
			utils.Kubectl("apply", "-f", case12ByteInformYaml, "-n", testNamespace)
			plc = utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case12ByteInform, testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case12ByteInform, testNamespace, true, defaultTimeoutSeconds)
				return utils.GetComplianceState(managedPlc)
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
		})
	})
})
