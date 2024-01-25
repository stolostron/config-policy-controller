package controllers

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policyv1 "open-cluster-management.io/config-policy-controller/api/v1"
	policyv1beta1 "open-cluster-management.io/config-policy-controller/api/v1beta1"
)

// updateStatus takes one condition to update, and related objects for that condition. The related
// objects given will replace all existing relatedObjects with the same gvk. If a condition is
// changed, the compliance will be recalculated and a compliance event will be emitted. The
// condition and related objects can match what is already in the status - in that case, no API
// calls are made. The `lastTransitionTime` on a condition is not considered when checking if the
// condition has changed. If not provided, the `lastTransitionTime` will use "now". It also handles
// preserving the `CreatedByPolicy` property on relatedObjects.
//
// This function requires that all given related objects are of the same kind.
//
// Note that only changing the related objects will not emit a new compliance event, but will update
// the status.
func (r *OperatorPolicyReconciler) updateStatus(
	ctx context.Context,
	policy *policyv1beta1.OperatorPolicy,
	updatedCondition metav1.Condition,
	updatedRelatedObjs ...policyv1.RelatedObject,
) error {
	condChanged := false

	if updatedCondition.LastTransitionTime.IsZero() {
		updatedCondition.LastTransitionTime = metav1.Now()
	}

	condIdx, existingCondition := policy.Status.GetCondition(updatedCondition.Type)
	if condIdx == -1 {
		condChanged = true

		// Just append, the conditions will be sorted later.
		policy.Status.Conditions = append(policy.Status.Conditions, updatedCondition)
	} else if conditionChanged(updatedCondition, existingCondition) {
		condChanged = true

		policy.Status.Conditions[condIdx] = updatedCondition
	}

	if condChanged {
		updatedComplianceCondition := calculateComplianceCondition(policy)

		compCondIdx, _ := policy.Status.GetCondition(updatedComplianceCondition.Type)
		if compCondIdx == -1 {
			policy.Status.Conditions = append(policy.Status.Conditions, updatedComplianceCondition)
		} else {
			policy.Status.Conditions[compCondIdx] = updatedComplianceCondition
		}

		// Sort the conditions based on their type.
		sort.SliceStable(policy.Status.Conditions, func(i, j int) bool {
			return policy.Status.Conditions[i].Type < policy.Status.Conditions[j].Type
		})

		if updatedComplianceCondition.Status == metav1.ConditionTrue {
			policy.Status.ComplianceState = policyv1.Compliant
		} else {
			policy.Status.ComplianceState = policyv1.NonCompliant
		}

		err := r.emitComplianceEvent(ctx, policy, updatedComplianceCondition)
		if err != nil {
			return err
		}
	}

	relObjsChanged := false

	prevRelObjs := make(map[int]policyv1.RelatedObject)
	if len(updatedRelatedObjs) != 0 {
		prevRelObjs = policy.Status.RelatedObjsOfKind(updatedRelatedObjs[0].Object.Kind)
	}

	for _, prevObj := range prevRelObjs {
		nameFound := false

		for i, updatedObj := range updatedRelatedObjs {
			if prevObj.Object.Metadata.Name != updatedObj.Object.Metadata.Name {
				continue
			}

			nameFound = true

			if updatedObj.Properties != nil && prevObj.Properties != nil {
				if updatedObj.Properties.UID != prevObj.Properties.UID {
					relObjsChanged = true
				} else if prevObj.Properties.CreatedByPolicy != nil {
					// There is an assumption here that it will never need to transition to false.
					updatedRelatedObjs[i].Properties.CreatedByPolicy = prevObj.Properties.CreatedByPolicy
				}
			}

			if prevObj.Compliant != updatedObj.Compliant || prevObj.Reason != updatedObj.Reason {
				relObjsChanged = true
			}
		}

		if !nameFound {
			relObjsChanged = true
		}
	}

	// Catch the case where there is a new object in updatedRelatedObjs
	if len(prevRelObjs) != len(updatedRelatedObjs) {
		relObjsChanged = true
	}

	if relObjsChanged {
		// start with the related objects which do not match the currently considered kind
		newRelObjs := make([]policyv1.RelatedObject, 0)

		for idx, relObj := range policy.Status.RelatedObjects {
			if _, matchedIdx := prevRelObjs[idx]; !matchedIdx {
				newRelObjs = append(newRelObjs, relObj)
			}
		}

		// add the new related objects
		newRelObjs = append(newRelObjs, updatedRelatedObjs...)

		// sort the related objects by kind and name
		sort.SliceStable(newRelObjs, func(i, j int) bool {
			if newRelObjs[i].Object.Kind != newRelObjs[j].Object.Kind {
				return newRelObjs[i].Object.Kind < newRelObjs[j].Object.Kind
			}

			return newRelObjs[i].Object.Metadata.Name < newRelObjs[j].Object.Metadata.Name
		})

		policy.Status.RelatedObjects = newRelObjs
	}

	if condChanged || relObjsChanged {
		return r.Status().Update(ctx, policy)
	}

	return nil
}

func conditionChanged(updatedCondition, existingCondition metav1.Condition) bool {
	if updatedCondition.Message != existingCondition.Message {
		return true
	}

	if updatedCondition.Reason != existingCondition.Reason {
		return true
	}

	if updatedCondition.Status != existingCondition.Status {
		return true
	}

	return false
}

// The Compliance condition is calculated by going through the known conditions in a consistent
// order, checking if there are any reasons the policy should be NonCompliant, and accumulating
// the reasons into one string to reflect the whole status.
func calculateComplianceCondition(policy *policyv1beta1.OperatorPolicy) metav1.Condition {
	foundNonCompliant := false
	messages := make([]string, 0)

	idx, cond := policy.Status.GetCondition(opGroupConditionType)
	if idx == -1 {
		messages = append(messages, "the status of the OperatorGroup is unknown")
		foundNonCompliant = true
	} else {
		messages = append(messages, cond.Message)

		if cond.Status != metav1.ConditionTrue {
			foundNonCompliant = true
		}
	}

	idx, cond = policy.Status.GetCondition(subConditionType)
	if idx == -1 {
		messages = append(messages, "the status of the Subscription is unknown")
		foundNonCompliant = true
	} else {
		messages = append(messages, cond.Message)

		if cond.Status != metav1.ConditionTrue {
			foundNonCompliant = true
		}
	}

	// FUTURE: check additional conditions

	if foundNonCompliant {
		return metav1.Condition{
			Type:               compliantConditionType,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             "NonCompliant",
			Message:            "NonCompliant; " + strings.Join(messages, ", "),
		}
	}

	return metav1.Condition{
		Type:               compliantConditionType,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "Compliant",
		Message:            "Compliant; " + strings.Join(messages, ", "),
	}
}

func (r *OperatorPolicyReconciler) emitComplianceEvent(
	ctx context.Context,
	policy *policyv1beta1.OperatorPolicy,
	complianceCondition metav1.Condition,
) error {
	if len(policy.OwnerReferences) == 0 {
		return nil // there is nothing to do, since no owner is set
	}

	ownerRef := policy.OwnerReferences[0]
	now := time.Now()
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			// This event name matches the convention of recorders from client-go
			Name:      fmt.Sprintf("%v.%x", ownerRef.Name, now.UnixNano()),
			Namespace: policy.Namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:       ownerRef.Kind,
			Namespace:  policy.Namespace, // k8s ensures owners are always in the same namespace
			Name:       ownerRef.Name,
			UID:        ownerRef.UID,
			APIVersion: ownerRef.APIVersion,
		},
		Reason:  fmt.Sprintf(eventFmtStr, policy.Namespace, policy.Name),
		Message: complianceCondition.Message,
		Source: corev1.EventSource{
			Component: ControllerName,
			Host:      r.InstanceName,
		},
		FirstTimestamp: metav1.NewTime(now),
		LastTimestamp:  metav1.NewTime(now),
		Count:          1,
		Type:           "Normal",
		Action:         "ComplianceStateUpdate",
		Related: &corev1.ObjectReference{
			Kind:       policy.Kind,
			Namespace:  policy.Namespace,
			Name:       policy.Name,
			UID:        policy.UID,
			APIVersion: policy.APIVersion,
		},
		ReportingController: ControllerName,
		ReportingInstance:   r.InstanceName,
	}

	if policy.Status.ComplianceState != policyv1.Compliant {
		event.Type = "Warning"
	}

	return r.Create(ctx, event)
}

const (
	compliantConditionType = "Compliant"
	opGroupConditionType   = "OperatorGroupCompliant"
	subConditionType       = "SubscriptionCompliant"
)

func condType(kind string) string {
	switch kind {
	case "OperatorGroup":
		return opGroupConditionType
	case "Subscription":
		return subConditionType
	default:
		panic("Unknown condition type for kind " + kind)
	}
}

// missingWantedCond returns a NonCompliant condition, with a Reason like '____Missing'
// and a Message like 'the ____ required by the policy was not found'
func missingWantedCond(kind string) metav1.Condition {
	return metav1.Condition{
		Type:    condType(kind),
		Status:  metav1.ConditionFalse,
		Reason:  kind + "Missing",
		Message: "the " + kind + " required by the policy was not found",
	}
}

// createdCond returns a Compliant condition, with a Reason like'____Created',
// and a Message like 'the ____ required by the policy was created'
func createdCond(kind string) metav1.Condition {
	return metav1.Condition{
		Type:    condType(kind),
		Status:  metav1.ConditionTrue,
		Reason:  kind + "Created",
		Message: "the " + kind + " required by the policy was created",
	}
}

// matchesCond returns a Compliant condition, with a Reason like'____Matches',
// and a Message like 'the ____ matches what is required by the policy'
func matchesCond(kind string) metav1.Condition {
	return metav1.Condition{
		Type:    condType(kind),
		Status:  metav1.ConditionTrue,
		Reason:  kind + "Matches",
		Message: "the " + kind + " matches what is required by the policy",
	}
}

// mismatchCond returns a NonCompliant condition with a Reason like '____Mismatch',
// and a Message like 'the ____ found on the cluster does not match the policy'
func mismatchCond(kind string) metav1.Condition {
	return metav1.Condition{
		Type:    condType(kind),
		Status:  metav1.ConditionFalse,
		Reason:  kind + "Mismatch",
		Message: "the " + kind + " found on the cluster does not match the policy",
	}
}

// mismatchCondUnfixable returns a NonCompliant condition with a Reason like '____Mismatch',
// and a Message like 'the ____ found on the cluster does not match the policy and can't be enforced'
func mismatchCondUnfixable(kind string) metav1.Condition {
	return metav1.Condition{
		Type:    condType(kind),
		Status:  metav1.ConditionFalse,
		Reason:  kind + "Mismatch",
		Message: "the " + kind + " found on the cluster does not match the policy and can't be enforced",
	}
}

// updatedCond returns a Compliant condition, with a Reason like'____Updated',
// and a Message like 'the ____ was updated to match the policy'
func updatedCond(kind string) metav1.Condition {
	return metav1.Condition{
		Type:    condType(kind),
		Status:  metav1.ConditionTrue,
		Reason:  kind + "Updated",
		Message: "the " + kind + " was updated to match the policy",
	}
}

var opGroupPreexistingCond = metav1.Condition{
	Type:   opGroupConditionType,
	Status: metav1.ConditionTrue,
	Reason: "PreexistingOperatorGroupFound",
	Message: "the policy does not specify an OperatorGroup but one already exists in the namespace - " +
		"assuming that OperatorGroup is correct",
}

// opGroupTooManyCond is a NonCompliant condition with a Reason like 'TooManyOperatorGroups',
// and a Message like 'there is more than one OperatorGroup in the namespace'
var opGroupTooManyCond = metav1.Condition{
	Type:    opGroupConditionType,
	Status:  metav1.ConditionFalse,
	Reason:  "TooManyOperatorGroups",
	Message: "there is more than one OperatorGroup in the namespace",
}

// missingWantedObj returns a NonCompliant RelatedObject with reason = 'Resource not found but should exist'
func missingWantedObj(obj client.Object) policyv1.RelatedObject {
	return policyv1.RelatedObject{
		Object:    policyv1.ObjectResourceFromObj(obj),
		Compliant: string(policyv1.NonCompliant),
		Reason:    reasonWantFoundDNE,
	}
}

// createdObj returns a Compliant RelatedObject with reason = 'K8s creation success'
func createdObj(obj client.Object) policyv1.RelatedObject {
	created := true

	return policyv1.RelatedObject{
		Object:    policyv1.ObjectResourceFromObj(obj),
		Compliant: string(policyv1.Compliant),
		Reason:    reasonWantFoundCreated,
		Properties: &policyv1.ObjectProperties{
			CreatedByPolicy: &created,
			UID:             string(obj.GetUID()),
		},
	}
}

// matchedObj returns a Compliant RelatedObject with reason = 'Resource found as expected'
func matchedObj(obj client.Object) policyv1.RelatedObject {
	return policyv1.RelatedObject{
		Object:    policyv1.ObjectResourceFromObj(obj),
		Compliant: string(policyv1.Compliant),
		Reason:    reasonWantFoundExists,
		Properties: &policyv1.ObjectProperties{
			UID: string(obj.GetUID()),
		},
	}
}

// mismatchedObj returns a NonCompliant RelatedObject with reason = 'Resource found but does not match'
func mismatchedObj(obj client.Object) policyv1.RelatedObject {
	return policyv1.RelatedObject{
		Object:    policyv1.ObjectResourceFromObj(obj),
		Compliant: string(policyv1.NonCompliant),
		Reason:    reasonWantFoundNoMatch,
		Properties: &policyv1.ObjectProperties{
			UID: string(obj.GetUID()),
		},
	}
}

// updatedObj returns a Compliant RelatedObject with reason = 'K8s update success'
func updatedObj(obj client.Object) policyv1.RelatedObject {
	return policyv1.RelatedObject{
		Object:    policyv1.ObjectResourceFromObj(obj),
		Compliant: string(policyv1.Compliant),
		Reason:    reasonUpdateSuccess,
		Properties: &policyv1.ObjectProperties{
			UID: string(obj.GetUID()),
		},
	}
}

// opGroupTooManyObjs returns a list of NonCompliant RelatedObjects, each with
// reason = 'There is more than one OperatorGroup in this namespace'
func opGroupTooManyObjs(opGroups []unstructured.Unstructured) []policyv1.RelatedObject {
	objs := make([]policyv1.RelatedObject, len(opGroups))

	for i, opGroup := range opGroups {
		objs[i] = policyv1.RelatedObject{
			Object:    policyv1.ObjectResourceFromObj(&opGroup),
			Compliant: string(policyv1.NonCompliant),
			Reason:    "There is more than one OperatorGroup in this namespace",
			Properties: &policyv1.ObjectProperties{
				UID: string(opGroup.GetUID()),
			},
		}
	}

	return objs
}
