// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"open-cluster-management.io/config-policy-controller/test/utils"
)

const (
	case5ConfigPolicyNameInform  string = "policy-pod-multi-mh"
	case5ConfigPolicyNameEnforce string = "policy-pod-multi-create"
	case5ConfigPolicyNameCombo   string = "policy-pod-multi-combo"
	case5PodName1                string = "nginx-pod-1"
	case5PodName2                string = "nginx-pod-1"
	case5InformYaml              string = "../resources/case5_multi/case5_multi_mh.yaml"
	case5EnforceYaml             string = "../resources/case5_multi/case5_multi_enforce.yaml"
	case5ComboYaml               string = "../resources/case5_multi/case5_multi_combo.yaml"
)

var _ = Describe("Test multiple obj template handling", func() {
	Describe("Create a policy on managed cluster in ns:"+testNamespace, func() {
		It("should be created properly on the managed cluster", func() {
			By("Creating " + case5ConfigPolicyNameInform + " and " + case5ConfigPolicyNameCombo + " on managed")
			utils.Kubectl("apply", "-f", case5InformYaml, "-n", testNamespace)
			plc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy,
				case5ConfigPolicyNameInform, testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy,
					case5ConfigPolicyNameInform, testNamespace, true, defaultTimeoutSeconds)

				return utils.GetComplianceState(managedPlc)
			}, defaultTimeoutSeconds, 1).Should(Equal("NonCompliant"))
			utils.Kubectl("apply", "-f", case5ComboYaml, "-n", testNamespace)
			plc = utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy,
				case5ConfigPolicyNameCombo, testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy,
					case5ConfigPolicyNameCombo, testNamespace, true, defaultTimeoutSeconds)

				return utils.GetComplianceState(managedPlc)
			}, defaultTimeoutSeconds, 1).Should(Equal("NonCompliant"))
		})
		It("should create pods on managed cluster", func() {
			By("creating " + case5ConfigPolicyNameEnforce + " on hub with spec.remediationAction = enforce")
			utils.Kubectl("apply", "-f", case5EnforceYaml, "-n", testNamespace)
			plc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy,
				case5ConfigPolicyNameEnforce, testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy,
					case5ConfigPolicyNameEnforce, testNamespace, true, defaultTimeoutSeconds)

				return utils.GetComplianceState(managedPlc)
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
			Eventually(func() interface{} {
				informPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy,
					case5ConfigPolicyNameInform, testNamespace, true, defaultTimeoutSeconds)

				return utils.GetComplianceState(informPlc)
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
			Eventually(func() interface{} {
				comboPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy,
					case5ConfigPolicyNameCombo, testNamespace, true, defaultTimeoutSeconds)

				return utils.GetComplianceState(comboPlc)
			}, defaultTimeoutSeconds, 1).Should(Equal("NonCompliant"))
			pod1 := utils.GetWithTimeout(clientManagedDynamic, gvrPod,
				case5PodName1, "default", true, defaultTimeoutSeconds)
			Expect(pod1).NotTo(BeNil())
			pod2 := utils.GetWithTimeout(clientManagedDynamic, gvrPod,
				case5PodName2, "default", true, defaultTimeoutSeconds)
			Expect(pod2).NotTo(BeNil())
		})
		It("Cleans up", func() {
			policies := []string{
				case5ConfigPolicyNameInform,
				case5ConfigPolicyNameEnforce,
				case5ConfigPolicyNameCombo,
			}

			deleteConfigPolicies(policies)
		})
	})

	Describe("Check messages when it is multiple namesapces and multiple obj-templates", Ordered, func() {
		const (
			case5MultiNamespace1               string = "n1"
			case5MultiNamespace2               string = "n2"
			case5MultiNamespace3               string = "n3"
			case5MultiNSConfigPolicyName       string = "policy-multi-namespace-enforce"
			case5MultiNSInformConfigPolicyName string = "policy-multi-namespace-inform"
			case5MultiObjNSConfigPolicyName    string = "policy-pod-multi-obj-temp-enforce"
			case5InformYaml                    string = "../resources/case5_multi/case5_multi_namespace_inform.yaml"
			case5EnforceYaml                   string = "../resources/case5_multi/case5_multi_namespace_enforce.yaml"
			case5MultiObjTmpYaml               string = "../resources/case5_multi/case5_multi_obj_template_enforce.yaml"
		)
		BeforeAll(func() {
			nss := []string{
				case5MultiNamespace1,
				case5MultiNamespace2,
				case5MultiNamespace3,
			}

			for _, ns := range nss {
				utils.Kubectl("create", "ns", ns)
			}
		})
		It("Should show merged Noncompliant messages when it is multiple namespaces and inform", func() {
			expectedMsg := "pods not found: [case5-multi-namespace-inform-pod] " +
				"in namespace n1 missing; [case5-multi-namespace-inform-pod] " +
				"in namespace n2 missing; [case5-multi-namespace-inform-pod] " +
				"in namespace n3 missing"
			utils.Kubectl("apply", "-f", case5InformYaml)
			utils.DoConfigPolicyMessageTest(clientManagedDynamic, gvrConfigPolicy, testNamespace,
				case5MultiNSInformConfigPolicyName, 0, defaultTimeoutSeconds, expectedMsg)
		})
		It("Should show merged messages when it is multiple namespaces", func() {
			expectedMsg := "Pod [case5-multi-namespace-enforce-pod] in namespace n1 found; " +
				"[case5-multi-namespace-enforce-pod] in namespace n2 found; " +
				"[case5-multi-namespace-enforce-pod] in namespace n3 found " +
				"as specified, therefore this Object template is compliant"
			utils.Kubectl("apply", "-f", case5EnforceYaml)
			utils.DoConfigPolicyMessageTest(clientManagedDynamic, gvrConfigPolicy, testNamespace,
				case5MultiNSConfigPolicyName, 0, defaultTimeoutSeconds, expectedMsg)
		})
		It("Should show 3 merged messages when it is multiple namespaces and multiple obj-template", func() {
			firstMsg := "Pod [case5-multi-obj-temp-pod-11] in namespace n1 found; " +
				"[case5-multi-obj-temp-pod-11] in namespace n2 found; " +
				"[case5-multi-obj-temp-pod-11] in namespace n3 found " +
				"as specified, therefore this Object template is compliant"
			secondMsg := "Pod [case5-multi-obj-temp-pod-22] in namespace n1 found; " +
				"[case5-multi-obj-temp-pod-22] in namespace n2 found; " +
				"[case5-multi-obj-temp-pod-22] in namespace n3 found " +
				"as specified, therefore this Object template is compliant"
			thirdMsg := "Pod [case5-multi-obj-temp-pod-33] in namespace n1 found; " +
				"[case5-multi-obj-temp-pod-33] in namespace n2 found; " +
				"[case5-multi-obj-temp-pod-33] in namespace n3 found " +
				"as specified, therefore this Object template is compliant"
			utils.Kubectl("apply", "-f", case5MultiObjTmpYaml)
			utils.DoConfigPolicyMessageTest(clientManagedDynamic, gvrConfigPolicy, testNamespace,
				case5MultiObjNSConfigPolicyName, 0, defaultTimeoutSeconds, firstMsg)
			utils.DoConfigPolicyMessageTest(clientManagedDynamic, gvrConfigPolicy, testNamespace,
				case5MultiObjNSConfigPolicyName, 1, defaultTimeoutSeconds, secondMsg)
			utils.DoConfigPolicyMessageTest(clientManagedDynamic, gvrConfigPolicy, testNamespace,
				case5MultiObjNSConfigPolicyName, 2, defaultTimeoutSeconds, thirdMsg)
		})
		cleanup := func() {
			policies := []string{
				case5MultiNSConfigPolicyName,
				case5MultiNSInformConfigPolicyName,
				case5MultiObjNSConfigPolicyName,
			}

			deleteConfigPolicies(policies)
			nss := []string{
				case5MultiNamespace1,
				case5MultiNamespace2,
				case5MultiNamespace3,
			}

			for _, ns := range nss {
				utils.Kubectl("delete", "ns", ns, "--ignore-not-found")
			}
		}
		AfterAll(cleanup)
	})
})
