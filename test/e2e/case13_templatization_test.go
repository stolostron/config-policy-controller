// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/config-policy-controller/test/utils"
)

const case13Secret string = "e2esecret"
const case13SecretCopy string = "e2esecret2"
const case13SecretYaml string = "../resources/case13_templatization/case13_secret.yaml"
const case13CfgPolCreateSecret string = "tmplt-policy-secret-duplicate"
const case13CfgPolCheckSecret string = "tmplt-policy-secret-duplicate-check"
const case13CfgPolCreateSecretYaml string = "../resources/case13_templatization/case13_copysecret.yaml"
const case13CfgPolCheckSecretYaml string = "../resources/case13_templatization/case13_verifysecret.yaml"

var _ = Describe("Test templatization", func() {
	Describe("Create a secret and pull data from it into a configurationPolicy", func() {
		It("should be created properly on the managed cluster", func() {
			By("Creating " + case13CfgPolCreateSecret + " and " + case13CfgPolCheckSecret + " on managed")
			//create secret
			utils.Kubectl("apply", "-f", case13SecretYaml, "-n", "default")
			secret := utils.GetWithTimeout(clientManagedDynamic, gvrSecret, case13Secret, "default", true, defaultTimeoutSeconds)
			Expect(secret).NotTo(BeNil())
			//create copy with password from original secret using a templatized policy
			utils.Kubectl("apply", "-f", case13CfgPolCreateSecretYaml, "-n", testNamespace)
			plc = utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case13CfgPolCreateSecret, testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case13CfgPolCreateSecret, testNamespace, true, defaultTimeoutSeconds)
				return utils.GetComplianceState(managedPlc)
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
			Eventually(func() interface{} {
				copiedSecret := utils.GetWithTimeout(clientManagedDynamic, gvrSecret, case13Secret, "default", true, defaultTimeoutSeconds)
				return utils.GetFieldFromSecret(copiedSecret, "PASSWORD")
			}, defaultTimeoutSeconds, 1).Should(Equal("MWYyZDFlMmU2N2Rm"))
			//check copied secret with a templatized inform policy
			utils.Kubectl("apply", "-f", case13CfgPolCheckSecretYaml, "-n", testNamespace)
			plc = utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case13CfgPolCheckSecret, testNamespace, true, defaultTimeoutSeconds)
			Expect(plc).NotTo(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigPolicy, case13CfgPolCheckSecret, testNamespace, true, defaultTimeoutSeconds)
				return utils.GetComplianceState(managedPlc)
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
		})
	})
})
