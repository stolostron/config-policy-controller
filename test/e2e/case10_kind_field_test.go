// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"open-cluster-management.io/config-policy-controller/test/utils"
)

const (
	case10ConfigPolicyNamePod   string = "policy-pod-c10-create"
	case10ConfigPolicyNameCheck string = "policy-kind-labels"
	case10ConfigPolicyNameFail  string = "policy-kind-labels-fail"
	case10PolicyYamlPod         string = "../resources/case10_kind_field/case10_pod_create.yaml"
	case10PolicyYamlCheck       string = "../resources/case10_kind_field/case10_kind_check.yaml"
	case10PolicyYamlFail        string = "../resources/case10_kind_field/case10_kind_fail.yaml"
)

var _ = Describe("Test pod obj template handling", func() {
	Describe("Create a pod policy on managed cluster in ns:"+testNamespace, Ordered, func() {
		It("should create a policy properly on the managed cluster", func() {
			By("Creating " + case10ConfigPolicyNamePod + " on managed")
			utils.Kubectl("apply", "-f", case10PolicyYamlPod, "-n", testNamespace)
			plc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy,
				case10ConfigPolicyNamePod, testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func(g Gomega) {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy,
					case10ConfigPolicyNamePod, testNamespace, true, defaultTimeoutSeconds)

				utils.CheckComplianceStatus(g, managedPlc, "Compliant")
			}, defaultTimeoutSeconds, 1).Should(Succeed())
		})
		It("should check annotations of all pods", func() {
			By("Creating " + case10ConfigPolicyNameCheck + " on managed")
			utils.Kubectl("apply", "-f", case10PolicyYamlCheck, "-n", testNamespace)
			plc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy,
				case10ConfigPolicyNameCheck, testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func(g Gomega) {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy,
					case10ConfigPolicyNameCheck, testNamespace, true, defaultTimeoutSeconds)

				utils.CheckComplianceStatus(g, managedPlc, "Compliant")
			}, defaultTimeoutSeconds, 1).Should(Succeed())
		})
		It("should return noncompliant if no pods match annotations", func() {
			By("Creating " + case10ConfigPolicyNameFail + " on managed")
			utils.Kubectl("apply", "-f", case10PolicyYamlFail, "-n", testNamespace)
			plc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy,
				case10ConfigPolicyNameFail, testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func(g Gomega) {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy,
					case10ConfigPolicyNameFail, testNamespace, true, defaultTimeoutSeconds)

				utils.CheckComplianceStatus(g, managedPlc, "NonCompliant")
			}, defaultTimeoutSeconds, 1).Should(Succeed())
		})
		AfterAll(func() {
			policies := []string{
				case10ConfigPolicyNamePod,
				case10ConfigPolicyNameCheck,
				case10ConfigPolicyNameFail,
			}

			deleteConfigPolicies(policies)
			utils.KubectlDelete("pod", "nginx-pod-e2e-10", "-n", "managed")
		})
	})
})
