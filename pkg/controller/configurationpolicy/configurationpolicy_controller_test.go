// Copyright (c) 2020 Red Hat, Inc.

package configurationpolicy

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	policiesv1alpha1 "github.com/stolostron/config-policy-controller/pkg/apis/policy/v1"
	"github.com/stolostron/config-policy-controller/pkg/common"
	"github.com/stretchr/testify/assert"
	coretypes "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var mgr manager.Manager
var err error

func TestReconcile(t *testing.T) {
	var (
		name      = "foo"
		namespace = "default"
	)
	instance := &policiesv1alpha1.ConfigurationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
		Spec: policiesv1alpha1.ConfigurationPolicySpec{
			Severity: "low",
			NamespaceSelector: policiesv1alpha1.Target{
				Include: []string{"default", "kube-*"},
				Exclude: []string{"kube-system"},
			},
			RemediationAction: "inform",
			ObjectTemplates: []*policiesv1alpha1.ObjectTemplate{
				&policiesv1alpha1.ObjectTemplate{
					ComplianceType:   "musthave",
					ObjectDefinition: runtime.RawExtension{},
				},
			},
		},
	}

	// Objects to track in the fake client.
	objs := []runtime.Object{instance}
	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	s.AddKnownTypes(policiesv1alpha1.SchemeGroupVersion, instance)

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClient(objs...)
	// Create a ReconcileConfigurationPolicy object with the scheme and fake client
	r := &ReconcileConfigurationPolicy{client: cl, scheme: s, recorder: nil}

	// Mock request to simulate Reconcile() being called on an event for a
	// watched resource .
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
	var simpleClient kubernetes.Interface = testclient.NewSimpleClientset()
	common.Initialize(&simpleClient, nil)
	res, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	t.Log(res)
}

// func TestHandleObjectTemplates(t *testing.T) {
// 	var typeMeta = metav1.TypeMeta{
// 		Kind: "namespace",
// 	}
// 	var objMeta = metav1.ObjectMeta{
// 		Name: "default",
// 	}
// 	var ns = coretypes.Namespace{
// 		TypeMeta:   typeMeta,
// 		ObjectMeta: objMeta,
// 	}
// 	defJSON := []byte(`{
// 		"apiVersion": "v1",
// 		"kind": "Pod"
// 	}`)

// 	re := runtime.RawExtension{}
// 	re.Raw = append(re.Raw[0:0], defJSON...)

// 	instance := &policiesv1alpha1.ConfigurationPolicy{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      "foo",
// 			Namespace: "default",
// 		},
// 		Spec: policiesv1alpha1.ConfigurationPolicySpec{
// 			Severity: "low",
// 			NamespaceSelector: policiesv1alpha1.Target{
// 				Include: []string{"default", "kube-*"},
// 				Exclude: []string{"kube-system"},
// 			},
// 			RemediationAction: "inform",
// 			ObjectTemplates: []*policiesv1alpha1.ObjectTemplate{
// 				&policiesv1alpha1.ObjectTemplate{
// 					ComplianceType:   "musthave",
// 					ObjectDefinition: re,
// 				},
// 			},
// 		},
// 	}
// 	// Register operator types with the runtime scheme.
// 	s := scheme.Scheme
// 	s.AddKnownTypes(policiesv1alpha1.SchemeGroupVersion, instance)

// 	var simpleClient kubernetes.Interface = testclient.NewSimpleClientset()
// 	simpleClient.CoreV1().Namespaces().Create(&ns)
// 	common.Initialize(&simpleClient, nil)

// 	handleObjectTemplates(*instance)
// }

// func TestPeriodicallyExecConfigPolicies(t *testing.T) {
// 	var (
// 		name      = "foo"
// 		namespace = "default"
// 	)
// 	var typeMeta = metav1.TypeMeta{
// 		Kind: "namespace",
// 	}
// 	var objMeta = metav1.ObjectMeta{
// 		Name: "default",
// 	}
// 	var ns = coretypes.Namespace{
// 		TypeMeta:   typeMeta,
// 		ObjectMeta: objMeta,
// 	}
// 	defJSON := []byte(`{
// 		"apiVersion": "v1",
// 		"kind": "Pod"
// 	}`)

// 	re := runtime.RawExtension{}
// 	re.Raw = append(re.Raw[0:0], defJSON...)

// 	// Mock request to simulate Reconcile() being called on an event for a
// 	// watched resource .
// 	req := reconcile.Request{
// 		NamespacedName: types.NamespacedName{
// 			Name:      name,
// 			Namespace: namespace,
// 		},
// 	}
// 	instance := &policiesv1alpha1.ConfigurationPolicy{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      "foo",
// 			Namespace: "default",
// 		},
// 		Spec: policiesv1alpha1.ConfigurationPolicySpec{
// 			Severity: "low",
// 			NamespaceSelector: policiesv1alpha1.Target{
// 				Include: []string{"default", "kube-*"},
// 				Exclude: []string{"kube-system"},
// 			},
// 			RemediationAction: "inform",
// 			ObjectTemplates:   []*policiesv1alpha1.ObjectTemplate{},
// 		},
// 	}

// 	// Objects to track in the fake client.
// 	objs := []runtime.Object{instance}
// 	// Register operator types with the runtime scheme.
// 	s := scheme.Scheme
// 	s.AddKnownTypes(policiesv1alpha1.SchemeGroupVersion, instance)

// 	// Create a fake client to mock API calls.
// 	cl := fake.NewFakeClient(objs...)

// 	// Create a ReconcileConfigurationPolicy object with the scheme and fake client.
// 	r := &ReconcileConfigurationPolicy{client: cl, scheme: s, recorder: nil}
// 	var simpleClient kubernetes.Interface = testclient.NewSimpleClientset()
// 	simpleClient.CoreV1().Namespaces().Create(&ns)
// 	common.Initialize(&simpleClient, nil)
// 	InitializeClient(&simpleClient)
// 	res, err := r.Reconcile(req)
// 	if err != nil {
// 		t.Fatalf("reconcile: (%v)", err)
// 	}
// 	t.Log(res)
// 	var target = []string{"default"}
// 	samplePolicy.Spec.NamespaceSelector.Include = target
// 	err = handleAddingPolicy(&samplePolicy)
// 	assert.Nil(t, err)
// 	PeriodicallyExecConfigPolicies(1, true)
// }

func TestCompareSpecs(t *testing.T) {
	var spec1 = map[string]interface{}{
		"containers": map[string]string{
			"image": "nginx1.7.9",
			"name":  "nginx",
		},
	}
	var spec2 = map[string]interface{}{
		"containers": map[string]string{
			"image": "nginx1.7.9",
			"test":  "test",
		},
	}
	merged, err := compareSpecs(spec1, spec2, "mustonlyhave")
	if err != nil {
		t.Fatalf("compareSpecs: (%v)", err)
	}
	var mergedExpected = map[string]interface{}{
		"containers": map[string]string{
			"image": "nginx1.7.9",
			"name":  "nginx",
		},
	}
	assert.Equal(t, reflect.DeepEqual(merged, mergedExpected), true)
	spec1 = map[string]interface{}{
		"containers": map[string]string{
			"image": "nginx1.7.9",
			"test":  "1111",
		},
	}
	spec2 = map[string]interface{}{
		"containers": map[string]string{
			"image": "nginx1.7.9",
			"name":  "nginx",
		},
	}
	merged, err = compareSpecs(spec1, spec2, "musthave")
	if err != nil {
		t.Fatalf("compareSpecs: (%v)", err)
	}
	mergedExpected = map[string]interface{}{
		"containers": map[string]string{
			"image": "nginx1.7.9",
			"name":  "nginx",
			"test":  "1111",
		},
	}
	assert.Equal(t, reflect.DeepEqual(fmt.Sprint(merged), fmt.Sprint(mergedExpected)), true)
}

func TestCompareLists(t *testing.T) {
	var rules1 = []interface{}{
		map[string]interface{}{
			"apiGroups": []string{
				"extensions", "apps",
			},
			"resources": []string{
				"deployments",
			},
			"verbs": []string{
				"get", "list", "watch", "create", "delete",
			},
		},
	}
	var rules2 = []interface{}{
		map[string]interface{}{
			"apiGroups": []string{
				"extensions", "apps",
			},
			"resources": []string{
				"deployments",
			},
			"verbs": []string{
				"get", "list",
			},
		},
	}
	merged, err := compareLists(rules2, rules1, "musthave")
	if err != nil {
		t.Fatalf("compareSpecs: (%v)", err)
	}
	mergedExpected := []interface{}{
		map[string]interface{}{
			"apiGroups": []string{
				"extensions", "apps",
			},
			"resources": []string{
				"deployments",
			},
			"verbs": []string{
				"get", "list",
			},
		},
		map[string]interface{}{
			"apiGroups": []string{
				"extensions", "apps",
			},
			"resources": []string{
				"deployments",
			},
			"verbs": []string{
				"get", "list", "watch", "create", "delete",
			},
		},
	}
	assert.Equal(t, reflect.DeepEqual(fmt.Sprint(merged), fmt.Sprint(mergedExpected)), true)
	merged, err = compareLists(rules2, rules1, "mustonlyhave")
	if err != nil {
		t.Fatalf("compareSpecs: (%v)", err)
	}
	mergedExpected = []interface{}{
		map[string]interface{}{
			"apiGroups": []string{
				"extensions", "apps",
			},
			"resources": []string{
				"deployments",
			},
			"verbs": []string{
				"get", "list",
			},
		},
	}
	assert.Equal(t, reflect.DeepEqual(fmt.Sprint(merged), fmt.Sprint(mergedExpected)), true)
}

func TestCreateParentPolicy(t *testing.T) {
	var ownerReference = metav1.OwnerReference{
		Name: "foo",
	}
	var ownerReferences = []metav1.OwnerReference{}
	ownerReferences = append(ownerReferences, ownerReference)
	samplePolicy.OwnerReferences = ownerReferences

	policy := createParentPolicy(&samplePolicy)
	assert.NotNil(t, policy)
	createParentPolicyEvent(&samplePolicy)
}

func TestConvertPolicyStatusToString(t *testing.T) {
	var compliantDetail = policiesv1alpha1.TemplateStatus{
		ComplianceState: policiesv1alpha1.NonCompliant,
		Conditions:      []policiesv1alpha1.Condition{},
	}
	var compliantDetails = []policiesv1alpha1.TemplateStatus{}

	for i := 0; i < 3; i++ {
		compliantDetails = append(compliantDetails, compliantDetail)
	}

	samplePolicyStatus := policiesv1alpha1.ConfigurationPolicyStatus{
		ComplianceState:   "Compliant",
		CompliancyDetails: compliantDetails,
	}
	samplePolicy.Status = samplePolicyStatus
	var policyInString = convertPolicyStatusToString(&samplePolicy)
	assert.NotNil(t, policyInString)
}

func TestHandleAddingPolicy(t *testing.T) {
	var simpleClient kubernetes.Interface = testclient.NewSimpleClientset()
	var typeMeta = metav1.TypeMeta{
		Kind: "namespace",
	}
	var objMeta = metav1.ObjectMeta{
		Name: "default",
	}
	var ns = coretypes.Namespace{
		TypeMeta:   typeMeta,
		ObjectMeta: objMeta,
	}
	simpleClient.CoreV1().Namespaces().Create(&ns)
	common.Initialize(&simpleClient, nil)
	err := handleAddingPolicy(&samplePolicy)
	assert.Nil(t, err)
	handleRemovingPolicy(samplePolicy.GetName())
}

func TestMerge(t *testing.T) {
	oldList := []interface{}{
		map[string]interface{}{
			"a": "apple",
			"b": "boy",
		},
		map[string]interface{}{
			"c": "candy",
			"d": "dog",
		},
	}
	newList := []interface{}{
		map[string]interface{}{
			"a": "apple",
			"b": "boy",
		},
	}
	merged1 := mergeArrays(newList, oldList, "musthave")
	assert.Equal(t, checkListsMatch(oldList, merged1), true)
	merged2 := mergeArrays(newList, oldList, "mustonlyhave")
	assert.Equal(t, checkListsMatch(newList, merged2), true)
	newList2 := []interface{}{
		map[string]interface{}{
			"b": "boy",
		},
	}
	oldList2 := []interface{}{
		map[string]interface{}{
			"a": "apple",
			"b": "boy",
		},
		map[string]interface{}{
			"c": "candy",
			"d": "dog",
		},
	}
	checkList2 := []interface{}{
		map[string]interface{}{
			"a": "apple",
			"b": "boy",
		},
		map[string]interface{}{
			"c": "candy",
			"d": "dog",
		},
	}
	merged3 := mergeArrays(newList2, oldList2, "musthave")
	assert.Equal(t, checkListsMatch(checkList2, merged3), true)
	newList3 := []interface{}{
		map[string]interface{}{
			"a": "apple",
		},
		map[string]interface{}{
			"c": "candy",
		},
	}
	merged4 := mergeArrays(newList3, oldList2, "musthave")
	assert.Equal(t, checkListsMatch(checkList2, merged4), true)
}

func TestAddRelatedObject(t *testing.T) {

	policy := &policiesv1alpha1.ConfigurationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
		Spec: policiesv1alpha1.ConfigurationPolicySpec{
			Severity: "low",
			NamespaceSelector: policiesv1alpha1.Target{
				Include: []string{"default", "kube-*"},
				Exclude: []string{"kube-system"},
			},
			RemediationAction: "inform",
			ObjectTemplates: []*policiesv1alpha1.ObjectTemplate{
				&policiesv1alpha1.ObjectTemplate{
					ComplianceType:   "musthave",
					ObjectDefinition: runtime.RawExtension{},
				},
			},
		},
	}
	compliant := true
	rsrc := policiesv1alpha1.SchemeBuilder.GroupVersion.WithResource("ConfigurationPolicy")
	namespace := "default"
	namespaced := true
	name := "foo"
	reason := "reason"
	relatedList := addRelatedObjects(policy, compliant, rsrc, namespace, namespaced, []string{name}, reason)
	related := relatedList[0]
	// get the related object and validate what we added is in the status
	assert.True(t, related.Compliant == string(policiesv1alpha1.Compliant))
	assert.True(t, related.Reason == "reason")
	assert.True(t, related.Object.APIVersion == rsrc.GroupVersion().String())
	assert.True(t, related.Object.Kind == rsrc.Resource)
	assert.True(t, related.Object.Metadata.Name == name)
	assert.True(t, related.Object.Metadata.Namespace == namespace)

	// add the same object and make sure the existing one is overwritten
	reason = "new"
	compliant = false
	relatedList = addRelatedObjects(policy, compliant, rsrc, namespace, namespaced, []string{name}, reason)
	related = relatedList[0]
	assert.True(t, len(relatedList) == 1)
	assert.True(t, related.Compliant == string(policiesv1alpha1.NonCompliant))
	assert.True(t, related.Reason == "new")

	// add a new related object and make sure the entry is appended
	name = "bar"
	relatedList = append(relatedList, addRelatedObjects(policy, compliant, rsrc, namespace, namespaced, []string{name}, reason)...)
	assert.True(t, len(relatedList) == 2)
	related = relatedList[1]
	assert.True(t, related.Object.Metadata.Name == name)
}

func TestSortRelatedObjectsAndUpdate(t *testing.T) {
	policy := &policiesv1alpha1.ConfigurationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
		Spec: policiesv1alpha1.ConfigurationPolicySpec{
			Severity: "low",
			NamespaceSelector: policiesv1alpha1.Target{
				Include: []string{"default", "kube-*"},
				Exclude: []string{"kube-system"},
			},
			RemediationAction: "inform",
			ObjectTemplates: []*policiesv1alpha1.ObjectTemplate{
				&policiesv1alpha1.ObjectTemplate{
					ComplianceType:   "musthave",
					ObjectDefinition: runtime.RawExtension{},
				},
			},
		},
	}
	rsrc := policiesv1alpha1.SchemeBuilder.GroupVersion.WithResource("ConfigurationPolicy")
	name := "foo"
	relatedList := addRelatedObjects(policy, true, rsrc, "default", true, []string{name}, "reason")

	// add the same object but after sorting it should be first
	name = "bar"
	relatedList = append(relatedList, addRelatedObjects(policy, true, rsrc, "default", true, []string{name}, "reason")...)

	empty := []policiesv1alpha1.RelatedObject{}
	sortRelatedObjectsAndUpdate(policy, relatedList, empty)
	assert.True(t, relatedList[0].Object.Metadata.Name == "bar")

	// append another object named bar but also with namespace bar
	relatedList = append(relatedList, addRelatedObjects(policy, true, rsrc, "bar", true, []string{name}, "reason")...)
	sortRelatedObjectsAndUpdate(policy, relatedList, empty)
	assert.True(t, relatedList[0].Object.Metadata.Namespace == "bar")

	// clear related objects and test sorting with no namespace
	relatedList = []policiesv1alpha1.RelatedObject{}
	name = "foo"
	relatedList = addRelatedObjects(policy, true, rsrc, "", false, []string{name}, "reason")
	name = "bar"
	relatedList = append(relatedList, addRelatedObjects(policy, true, rsrc, "", false, []string{name}, "reason")...)
	sortRelatedObjectsAndUpdate(policy, relatedList, empty)
	assert.True(t, relatedList[0].Object.Metadata.Name == "bar")
}

func newRule(verbs, apiGroups, resources, nonResourceURLs string) rbacv1.PolicyRule {
	return rbacv1.PolicyRule{
		Verbs:           strings.Split(verbs, ","),
		APIGroups:       strings.Split(apiGroups, ","),
		Resources:       strings.Split(resources, ","),
		NonResourceURLs: strings.Split(nonResourceURLs, ","),
	}
}

func newRole(name, namespace string, rules ...rbacv1.PolicyRule) *rbacv1.Role {
	return &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}, Rules: rules}
}

func newRuleTemplate(verbs, apiGroups, resources, nonResourceURLs string, complianceT policiesv1alpha1.ComplianceType) policiesv1alpha1.PolicyRuleTemplate {
	return policiesv1alpha1.PolicyRuleTemplate{
		ComplianceType: complianceT,
		PolicyRule: rbacv1.PolicyRule{
			Verbs:           strings.Split(verbs, ","),
			APIGroups:       strings.Split(apiGroups, ","),
			Resources:       strings.Split(resources, ","),
			NonResourceURLs: strings.Split(nonResourceURLs, ","),
		},
	}
}

func TestCreateInformStatus(t *testing.T) {
	policy := &policiesv1alpha1.ConfigurationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
		Spec: policiesv1alpha1.ConfigurationPolicySpec{
			Severity: "low",
			NamespaceSelector: policiesv1alpha1.Target{
				Include: []string{"test1", "test2"},
			},
			RemediationAction: "inform",
			ObjectTemplates: []*policiesv1alpha1.ObjectTemplate{
				&policiesv1alpha1.ObjectTemplate{
					ComplianceType:   "musthave",
					ObjectDefinition: runtime.RawExtension{},
				},
			},
		},
	}
	objNamespaced := true
	objData := map[string]interface{}{
		"indx":        0,
		"kind":        "Secret",
		"desiredName": "myobject",
		"namespaced":  objNamespaced,
	}
	mustNotHave := false
	numCompliant := 0
	numNonCompliant := 1
	var nonCompliantObjects map[string]map[string]interface{} = make(map[string]map[string]interface{})
	var compliantObjects map[string]map[string]interface{} = make(map[string]map[string]interface{})
	nonCompliantObjects["test1"] = map[string]interface{}{
		"names":  []string{"myobject"},
		"reason": "my reason",
	}

	// Test 1 NonCompliant resource
	createInformStatus(mustNotHave, numCompliant, numNonCompliant,
		compliantObjects, nonCompliantObjects, policy, objData)
	assert.True(t, policy.Status.CompliancyDetails[0].ComplianceState == policiesv1alpha1.NonCompliant)

	nonCompliantObjects["test2"] = map[string]interface{}{
		"names":  []string{"myobject"},
		"reason": "my reason",
	}
	numNonCompliant = 2

	// Test 2 NonCompliant resources
	createInformStatus(mustNotHave, numCompliant, numNonCompliant,
		compliantObjects, nonCompliantObjects, policy, objData)
	assert.True(t, policy.Status.CompliancyDetails[0].ComplianceState == policiesv1alpha1.NonCompliant)

	delete(nonCompliantObjects, "test1")
	delete(nonCompliantObjects, "test2")

	// Test 0 resources
	numNonCompliant = 0
	createInformStatus(mustNotHave, numCompliant, numNonCompliant,
		compliantObjects, nonCompliantObjects, policy, objData)
	assert.True(t, policy.Status.CompliancyDetails[0].ComplianceState == policiesv1alpha1.NonCompliant)

	compliantObjects["test1"] = map[string]interface{}{
		"names":  []string{"myobject"},
		"reason": "my reason",
	}
	numCompliant = 1
	nonCompliantObjects["test2"] = map[string]interface{}{
		"names":  []string{"myobject"},
		"reason": "my reason",
	}
	numNonCompliant = 1

	// Test 1 compliant and 1 noncompliant resource  NOTE: This use case is the new behavior change!
	createInformStatus(mustNotHave, numCompliant, numNonCompliant,
		compliantObjects, nonCompliantObjects, policy, objData)
	assert.True(t, policy.Status.CompliancyDetails[0].ComplianceState == policiesv1alpha1.NonCompliant)

	compliantObjects["test2"] = map[string]interface{}{
		"names":  []string{"myobject"},
		"reason": "my reason",
	}
	numCompliant = 2
	numNonCompliant = 0
	delete(nonCompliantObjects, "test2")

	// Test 2 compliant resources
	createInformStatus(mustNotHave, numCompliant, numNonCompliant,
		compliantObjects, nonCompliantObjects, policy, objData)
	assert.True(t, policy.Status.CompliancyDetails[0].ComplianceState == policiesv1alpha1.Compliant)
}
