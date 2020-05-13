// Copyright (c) 2020 Red Hat, Inc.

package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/config-policy-controller/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

//const case1PolicyName string = "default.case1-create-policy"
const case1ConfigPolicyName string = "policy-pod-create"
const case1PodName string = "nginx-pod-e2e"
const case1PolicyYaml string = "../resources/case1_pod_handling/case1_pod_create.yaml"
const case1PolicyCheckMNHYaml string = "../resources/case1_pod_handling/case1_pod_check-mnh.yaml"

var _ = Describe("Test pod obj template handling", func() {
	Describe("Create a policy on managed cluster in ns:"+testNamespace, func() {
		It("should be created properly on the managed cluster", func() {
			By("Creating " + case1PolicyYaml + " on managed")
			klog.Infof("using test namespace %s", testNamespace)
			utils.Kubectl("apply", "-f", case1PolicyYaml, "-n", testNamespace)
			plc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case1ConfigPolicyName, testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case1ConfigPolicyName, testNamespace, true, defaultTimeoutSeconds)
				return managedPlc.Object["status"].(map[string]interface{})["compliant"]
			}, defaultTimeoutSeconds, 1).Should(Equal("NonCompliant"))
		})
		It("should create pod on managed cluster", func() {
			By("Patching " + case1PolicyYaml + " on hub with spec.remediationAction = enforce")
			managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case1ConfigPolicyName, testNamespace, true, defaultTimeoutSeconds)
			Expect(managedPlc).NotTo(BeNil())
			Expect(managedPlc.Object["spec"].(map[string]interface{})["remediationAction"]).To(Equal("inform"))
			managedPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "enforce"
			managedPlc, err := clientManagedDynamic.Resource(gvrPolicy).Namespace(testNamespace).Update(managedPlc, metav1.UpdateOptions{})
			Expect(err).To(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case1ConfigPolicyName, testNamespace, true, defaultTimeoutSeconds)
				return managedPlc.Object["status"].(map[string]interface{})["compliant"]
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
			pod := utils.GetWithTimeout(clientManagedDynamic, gvrPod, case1PodName, testNamespace, true, defaultTimeoutSeconds)
			Expect(pod).NotTo(BeNil())
		})
		It("should create violations properly", func() {
			By("Creating " + case1PolicyYaml + " on managed")
			utils.Kubectl("apply", "-f", case1PolicyCheckMNHYaml, "-n", testNamespace)
			plc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, "policy-pod-check-mnh", testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, "policy-pod-check-mnh", testNamespace, true, defaultTimeoutSeconds)
				return managedPlc.Object["status"].(map[string]interface{})["compliant"]
			}, defaultTimeoutSeconds, 1).Should(Equal("NonCompliant"))
			utils.Kubectl("apply", "-f", case1PolicyCheckMNHYaml, "-n", testNamespace)
			plc = utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, "policy-pod-check-moh", testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, "policy-pod-check-moh", testNamespace, true, defaultTimeoutSeconds)
				return managedPlc.Object["status"].(map[string]interface{})["compliant"]
			}, defaultTimeoutSeconds, 1).Should(Equal("NonCompliant"))
		})
	})
})
