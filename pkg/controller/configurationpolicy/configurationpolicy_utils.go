// Copyright (c) 2021 Red Hat, Inc.

package configurationpolicy

import (
	"fmt"
	"sort"
	"strings"

	policyv1 "github.com/open-cluster-management/config-policy-controller/pkg/apis/policy/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var statusNamespaceString = " in namespace "

// addRelatedObjects builds the list of kubernetes resources related to the policy.  The list contains
// details on whether the object is compliant or not compliant with the policy.  The results are updated in the
// policy's Status information.
func addRelatedObjects(policy *policyv1.ConfigurationPolicy, compliant bool, rsrc schema.GroupVersionResource,
	namespace string, namespaced bool, objNames []string,
	nameLinkMap map[string]string, reason string) (relatedObjects []policyv1.RelatedObject) {

	for _, name := range objNames {
		// Initialize the related object from the object handling
		var relatedObject policyv1.RelatedObject
		if compliant {
			relatedObject.Compliant = string(policyv1.Compliant)
		} else {
			relatedObject.Compliant = string(policyv1.NonCompliant)
		}

		relatedObject.Reason = reason

		var metadata policyv1.ObjectMetadata
		metadata.Name = name
		if namespaced {
			metadata.Namespace = namespace
		} else {
			metadata.Namespace = ""
		}
		selfLink, ok := nameLinkMap[name]
		if ok {
			metadata.SelfLink = selfLink
		} else {
			metadata.SelfLink = ""
		}
		relatedObject.Object.APIVersion = rsrc.GroupVersion().String()
		relatedObject.Object.Kind = rsrc.Resource
		relatedObject.Object.Metadata = metadata
		relatedObjects = updateRelatedObjectsStatus(relatedObjects, relatedObject)
	}
	return relatedObjects
}

// updateRelatedObjectsStatus adds or updates the RelatedObject in the policy status.
func updateRelatedObjectsStatus(list []policyv1.RelatedObject,
	relatedObject policyv1.RelatedObject) (result []policyv1.RelatedObject) {
	present := false
	for index, currentObject := range list {
		if currentObject.Object.APIVersion ==
			relatedObject.Object.APIVersion && currentObject.Object.Kind == relatedObject.Object.Kind {
			if currentObject.Object.Metadata.Name ==
				relatedObject.Object.Metadata.Name && currentObject.Object.Metadata.Namespace ==
				relatedObject.Object.Metadata.Namespace {
				present = true
				if currentObject.Compliant != relatedObject.Compliant {
					list[index] = relatedObject
				}
			}
		}
	}
	if !present {
		list = append(list, relatedObject)
	}
	return list
}

func checkFieldsWithSort(mergedObj map[string]interface{}, oldObj map[string]interface{}) (matches bool) {
	//needed to compare lists, since merge messes up the order
	match := true
	for i, mVal := range mergedObj {
		switch mVal := mVal.(type) {
		case ([]interface{}):
			oVal, ok := oldObj[i].([]interface{})
			if !ok {
				match = false
				break
			}
			sort.Slice(oVal, func(i, j int) bool {
				return fmt.Sprintf("%v", oVal[i]) < fmt.Sprintf("%v", oVal[j])
			})
			sort.Slice(mVal, func(x, y int) bool {
				return fmt.Sprintf("%v", mVal[x]) < fmt.Sprintf("%v", mVal[y])
			})
			if len(mVal) != len(oVal) {
				match = false
			} else {
				if !checkListsMatch(oVal, mVal) {
					match = false
				}
			}
		default:
			oVal := oldObj[i]
			if fmt.Sprint(oVal) != fmt.Sprint(mVal) {
				match = false
			}
		}
	}
	return match
}

func checkListsMatch(oVal []interface{}, mVal []interface{}) (m bool) {
	match := true
	for idx, oNestedVal := range oVal {
		if fmt.Sprint(oNestedVal) != fmt.Sprint(mVal[idx]) {
			match = false
		}
	}
	return match
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func isDenylisted(key string) (result bool) {
	denylist := []string{"apiVersion", "kind"}
	for _, val := range denylist {
		if key == val {
			return true
		}
	}
	return false
}

func isAutogenerated(key string) (result bool) {
	denylist := []string{"kubectl.kubernetes.io/last-applied-configuration"}
	for _, val := range denylist {
		if key == val {
			return true
		}
	}
	return false
}

func formatTemplate(unstruct unstructured.Unstructured, key string) (obj interface{}) {
	if key == "metadata" {
		metadata := unstruct.Object[key].(map[string]interface{})
		return formatMetadata(metadata)
	}
	return unstruct.Object[key]
}

func formatMetadata(metadata map[string]interface{}) (formatted map[string]interface{}) {
	md := map[string]interface{}{}
	if labels, ok := metadata["labels"]; ok {
		md["labels"] = labels
	}
	if annos, ok := metadata["annotations"]; ok {
		noAutogenerated := map[string]interface{}{}
		for key := range annos.(map[string]interface{}) {
			if !isAutogenerated(key) {
				noAutogenerated[key] = annos.(map[string]interface{})[key]
			}
		}
		if len(noAutogenerated) > 0 {
			md["annotations"] = noAutogenerated
		}
	}
	return md
}

//createCompliantMustHaveStatus generates a status for a musthave/mustonlyhave policy that is compliant
func createCompliantMustHaveStatus(kind string, compliantObjects map[string]map[string]interface{},
	namespaced bool, plc *policyv1.ConfigurationPolicy, indx int) (update bool) {
	nameList := []string{}
	sortedNamespaces := []string{}
	for n := range compliantObjects {
		sortedNamespaces = append(sortedNamespaces, n)
	}
	sort.Strings(sortedNamespaces)
	for i := range sortedNamespaces {
		nameStr := ""
		ns := sortedNamespaces[i]
		names := compliantObjects[ns]["names"].([]string)
		sort.Strings(names)
		nameStr += "["
		for i, name := range names {
			nameStr += name
			if i != len(names)-1 {
				nameStr += ", "
			}
		}
		nameStr += "]"
		if namespaced {
			nameStr += statusNamespaceString + ns
		}
		if len(names) == 1 {
			nameStr += " exists"
		} else {
			nameStr += " exist"
		}
		if !stringInSlice(nameStr, nameList) {
			nameList = append(nameList, nameStr)
		}
	}
	names := strings.Join(nameList, "; ")
	message := fmt.Sprintf("%v %v as specified, therefore this Object template is compliant", kind, names)
	return createNotification(plc, indx, "K8s `must have` object already exists", message)
}

//createNonCompliantMustHaveStatus generates a status for a musthave/mustonlyhave policy that is noncompliant
func createNonCompliantMustHaveStatus(desiredName string, kind string,
	nonCompliantObjects map[string]map[string]interface{}, namespaced bool, plc *policyv1.ConfigurationPolicy,
	indx int) (update bool) {
	message := ""
	if desiredName == "" {
		message = fmt.Sprintf("No instances of `%v` exist as specified", kind)
	} else {
		nameList := []string{}
		sortedNamespaces := []string{}
		for n := range nonCompliantObjects {
			sortedNamespaces = append(sortedNamespaces, n)
		}
		sort.Strings(sortedNamespaces)
		for i := range sortedNamespaces {
			nameStr := ""
			ns := sortedNamespaces[i]
			names := nonCompliantObjects[ns]["names"].([]string)
			reason := nonCompliantObjects[ns]["reason"].(string)
			sort.Strings(names)
			nameStr += "["
			for i, name := range names {
				nameStr += name
				if i != len(names)-1 {
					nameStr += ", "
				}
			}
			nameStr += "]"
			if namespaced {
				nameStr += statusNamespaceString + ns
			}
			single := false
			if len(names) == 1 {
				single = true
			}
			if reason == reasonWantFoundNoMatch && single {
				nameStr += " exists but doesn't match"
			} else if reason == reasonWantFoundNoMatch && !single {
				nameStr += " exist but don't match"
			} else if reason != reasonWantFoundNoMatch && single {
				nameStr += " does not exist"
			} else {
				nameStr += " do not exist"
			}
			if !stringInSlice(nameStr, nameList) {
				nameList = append(nameList, nameStr)
			}
		}
		names := strings.Join(nameList, "; ")
		message = fmt.Sprintf("%v not found: %v", kind, names)
	}
	return createViolation(plc, indx, "K8s has a must `not` have object", message)
}

//createCompliantMustNotHaveStatus generates a status for a mustnothave policy that is compliant
func createCompliantMustNotHaveStatus(kind string, compliantObjects map[string]map[string]interface{},
	namespaced bool, plc *policyv1.ConfigurationPolicy, indx int) (update bool) {
	nameList := []string{}
	sortedNamespaces := []string{}
	for n := range compliantObjects {
		sortedNamespaces = append(sortedNamespaces, n)
	}
	sort.Strings(sortedNamespaces)
	for i := range sortedNamespaces {
		nameStr := ""
		ns := sortedNamespaces[i]
		names := compliantObjects[ns]["names"].([]string)
		sort.Strings(names)
		nameStr += "["
		for i, name := range names {
			nameStr += name
			if i != len(names)-1 {
				nameStr += ", "
			}
		}
		nameStr += "]"
		if namespaced {
			nameStr += statusNamespaceString + ns
		}
		if len(names) == 1 {
			nameStr += " is"
		} else {
			nameStr += " are"
		}
		if !stringInSlice(nameStr, nameList) {
			nameList = append(nameList, nameStr)
		}
	}
	names := strings.Join(nameList, "; ")
	message := fmt.Sprintf("%v %v missing as expected, therefore this Object template is compliant", kind, names)
	return createNotification(plc, indx, "K8s `must have` object already exists", message)
}

//createNonCompliantMustNotHaveStatus generates a status for a mustnothave policy that is noncompliant
func createNonCompliantMustNotHaveStatus(kind string, nonCompliantObjects map[string]map[string]interface{},
	namespaced bool, plc *policyv1.ConfigurationPolicy, indx int) (update bool) {
	nameList := []string{}
	sortedNamespaces := []string{}
	for n := range nonCompliantObjects {
		sortedNamespaces = append(sortedNamespaces, n)
	}
	sort.Strings(sortedNamespaces)
	for i := range sortedNamespaces {
		nameStr := ""
		ns := sortedNamespaces[i]
		names := nonCompliantObjects[ns]["names"].([]string)
		sort.Strings(names)
		nameStr += "["
		for i, name := range names {
			nameStr += name
			if i != len(names)-1 {
				nameStr += ", "
			}
		}
		nameStr += "]"
		if namespaced {
			nameStr += statusNamespaceString + ns
		}
		if !stringInSlice(nameStr, nameList) {
			nameList = append(nameList, nameStr)
		}
	}
	names := strings.Join(nameList, "; ")
	message := fmt.Sprintf("%v exist: %v", kind, names)
	return createViolation(plc, indx, "K8s has a must `not` have object", message)
}
