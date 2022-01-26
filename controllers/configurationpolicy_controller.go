// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	gocmp "github.com/google/go-cmp/cmp"
	templates "github.com/stolostron/go-template-utils/v2/pkg/templates"
	extpoliciesv1 "github.com/stolostron/governance-policy-propagator/api/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	policyv1 "github.com/stolostron/config-policy-controller/api/v1"
	common "github.com/stolostron/config-policy-controller/pkg/common"
)

const ControllerName string = "configuration-policy-controller"

var log = ctrl.Log.WithName(ControllerName)

// availablePolicies is a cach all all available polices
var availablePolicies common.SyncedPolicyMap

// PlcChan a channel used to pass policies ready for update
var PlcChan chan *policyv1.ConfigurationPolicy

var clientSet *kubernetes.Clientset

var (
	eventNormal  = "Normal"
	eventWarning = "Warning"
	eventFmtStr  = "policy: %s/%s"
	plcFmtStr    = "policy: %s"
)

var (
	reasonWantFoundExists    = "Resource found as expected"
	reasonWantFoundNoMatch   = "Resource found but does not match"
	reasonWantFoundDNE       = "Resource not found but should exist"
	reasonWantNotFoundExists = "Resource found but should not exist"
	reasonWantNotFoundDNE    = "Resource not found as expected"
)

var config *rest.Config

// Mx for making the map thread safe
var Mx sync.RWMutex

// MxUpdateMap for making the map thread safe
var MxUpdateMap sync.RWMutex

// NamespaceWatched defines which namespace we can watch for the GRC policies and ignore others
var NamespaceWatched string

// EventOnParent specifies if we also want to send events to the parent policy. Available options are yes/no/ifpresent
var EventOnParent string

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigurationPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(ControllerName).
		For(&policyv1.ConfigurationPolicy{}).
		Complete(r)
}

// Initialize to initialize some controller variables
func Initialize(kubeconfig *rest.Config, clientset *kubernetes.Clientset, namespace, eventParent string) {
	config = kubeconfig
	clientSet = clientset
	NamespaceWatched = namespace
	EventOnParent = strings.ToLower(eventParent)
}

// blank assignment to verify that ConfigurationPolicyReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &ConfigurationPolicyReconciler{}

type cachedEncryptionKey struct {
	key         []byte
	previousKey []byte
}

// ConfigurationPolicyReconciler reconciles a ConfigurationPolicy object
type ConfigurationPolicyReconciler struct {
	cachedEncryptionKey *cachedEncryptionKey
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
	DecryptionConcurrency uint8
	Scheme                *runtime.Scheme
	Recorder              record.EventRecorder
}

//+kubebuilder:rbac:groups=*,resources=*,verbs=*

// Reconcile reads that state of the cluster for a ConfigurationPolicy object and makes changes based
// on the state read and what is in the ConfigurationPolicy.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ConfigurationPolicyReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling ConfigurationPolicy")

	// Fetch the ConfigurationPolicy instance
	instance := &policyv1.ConfigurationPolicy{}

	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("Configuration policy was deleted, removing it...")
			handleRemovingPolicy(request.NamespacedName.Name)

			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to retrieve configuration policy")

		return reconcile.Result{}, err
	}

	reqLogger.V(1).Info("Configuration policy was found, adding it...")

	err = handleAddingPolicy(instance)
	if err != nil {
		reqLogger.Error(err, "Failed to handleAddingPolicy", "instance", instance)

		return reconcile.Result{}, err
	}

	reqLogger.Info("Policy successfully added, reconcile complete")

	return reconcile.Result{}, nil
}

// PeriodicallyExecConfigPolicies loops through all configurationpolicies in the target namespace and triggers
// template handling for each one. This function drives all the work the configuration policy controller does.
func (r *ConfigurationPolicyReconciler) PeriodicallyExecConfigPolicies(freq uint, test bool) {
	cachedAPIResourceList := []*metav1.APIResourceList{}
	cachedAPIGroupsList := []*restmapper.APIGroupResources{}

	for {
		start := time.Now()
		flattenedPolicyList := map[string]*policyv1.ConfigurationPolicy{}

		log.V(2).Info(sprintMap(availablePolicies.PolicyMap))

		for _, policy := range availablePolicies.PolicyMap {
			key := fmt.Sprintf("%s/%s", policy.GetName(), policy.GetResourceVersion())
			if _, ok := flattenedPolicyList[key]; ok {
				continue
			} else {
				flattenedPolicyList[key] = policy
			}
		}

		// get resources once per cycle to avoid hanging
		dd := clientSet.Discovery()
		//nolint:staticcheck,nolintlint
		apiresourcelist, apiresourcelistErr := dd.ServerResources()

		if len(apiresourcelist) > 0 {
			cachedAPIResourceList = append([]*metav1.APIResourceList{}, apiresourcelist...)
		}

		skipLoop := false

		if apiresourcelistErr != nil && len(cachedAPIResourceList) > 0 {
			apiresourcelist = cachedAPIResourceList

			log.Error(apiresourcelistErr, "Could not get API resource list, using cached list")
		} else if apiresourcelistErr != nil {
			skipLoop = true
			log.Error(apiresourcelistErr, "Could not get API resource list, skipping loop because cached list is empty")
		}

		apigroups, apigroupsErr := restmapper.GetAPIGroupResources(dd)

		if len(apigroups) > 0 {
			cachedAPIGroupsList = append([]*restmapper.APIGroupResources{}, apigroups...)
		}

		if apigroupsErr != nil && len(cachedAPIGroupsList) > 0 {
			apigroups = cachedAPIGroupsList

			log.Error(apigroupsErr, "Could not get API groups list, using cached list")
		} else if !skipLoop && apigroupsErr != nil {
			skipLoop = true
			log.Error(apigroupsErr, "Could not get API groups list, skipping loop because cached list is empty")
		}

		if !skipLoop {
			log.Info("Processing the policies", "count", len(flattenedPolicyList))
			// flattenedpolicylist only contains 1 of each policy instance
			for _, policy := range flattenedPolicyList {
				Mx.Lock()
				// Deep copy the policy since even though handleObjectTemplates accepts a copy
				// (i.e. not a pointer) of the policy, policy.Spec.ObjectTemplates is a slice of
				// pointers, so any modifications to the objects in that slice will be reflected in
				// the PolicyMap cache, which can have unintended side effects.
				policy = (*policy).DeepCopy()
				// handle each template in each policy
				r.handleObjectTemplates(*policy, apiresourcelist, apigroups)
				Mx.Unlock()
			}
		}

		// making sure that if processing is > freq we don't sleep
		// if freq > processing we sleep for the remaining duration
		elapsed := time.Since(start) / 1000000000 // convert to seconds
		if float64(freq) > float64(elapsed) {
			remainingSleep := float64(freq) - float64(elapsed)
			sleepTime := time.Duration(remainingSleep) * time.Second
			log.V(2).Info("Sleeping before reprocessing the configuration polices", "seconds", sleepTime)
			time.Sleep(sleepTime)
		}

		if test {
			return
		}
	}
}

// getTemplateConfigErrorMsg converts a configuration error from `NewResolver` or `SetEncryptionConfig` to a message
// to be used as a policy noncompliant message.
func getTemplateConfigErrorMsg(err error) string {
	if errors.Is(err, templates.ErrInvalidAESKey) || errors.Is(err, templates.ErrAESKeyNotSet) {
		return `The "policy-encryption-key" Secret contains an invalid AES key`
	} else if errors.Is(err, templates.ErrInvalidIV) {
		return fmt.Sprintf(`The "%s" annotation value is not a valid initialization vector`, IVAnnotation)
	}

	// This should never happen unless go-template-utils is updated and this doesn't account for a new error
	// type that can happen with the input configuration.
	return fmt.Sprintf("An unexpected error occurred when configuring the template resolver: %v", err)
}

// handleObjectTemplates iterates through all policy templates in a given policy and processes them
func (r *ConfigurationPolicyReconciler) handleObjectTemplates(
	plc policyv1.ConfigurationPolicy,
	apiresourcelist []*metav1.APIResourceList,
	apigroups []*restmapper.APIGroupResources,
) {
	log := log.WithValues("policy", plc.GetName())
	log.V(1).Info("Processing object templates")

	// error if no remediationAction is specified
	plcNamespaces := getPolicyNamespaces(plc)
	log.V(2).Info("Determined the applicable namespaces", "count", len(plcNamespaces))

	if plc.Spec.RemediationAction == "" {
		message := "Policy does not have a RemediationAction specified"
		log.Info(message)
		update := addConditionToStatus(&plc, 0, false, "No RemediationAction", message)

		if update {
			r.Recorder.Event(&plc, eventWarning,
				fmt.Sprintf(plcFmtStr, plc.GetName()), convertPolicyStatusToString(&plc))
			r.addForUpdate(&plc)
		}

		return
	}

	// initialize the RelatedObjects for this Configuration Policy
	oldRelated := append([]policyv1.RelatedObject{}, plc.Status.RelatedObjects...)

	relatedObjects := []policyv1.RelatedObject{}
	parentUpdate := false

	addTemplateErrorViolation := func(reason, msg string) {
		log.Info("Setting the policy to noncompliant due to a templating error", "error", msg)

		if reason == "" {
			reason = "Error processing template"
		}

		update := addConditionToStatus(&plc, 0, false, reason, msg)
		if update {
			r.Recorder.Event(
				&plc,
				eventWarning,
				fmt.Sprintf(plcFmtStr, plc.GetName()),
				convertPolicyStatusToString(&plc),
			)

			r.checkRelatedAndUpdate(update, plc, relatedObjects, oldRelated)
		}
	}

	// initialize apiresources for template processing before starting objectTemplate processing
	// this is optional but since apiresourcelist is already available,
	// use this rather than re-discovering the list for generic-lookup
	tmplResolverCfg := templates.Config{KubeAPIResourceList: apiresourcelist}
	usedKeyCache := false

	if usesEncryption(plc) {
		var encryptionConfig templates.EncryptionConfig
		var err error

		encryptionConfig, usedKeyCache, err = r.getEncryptionConfig(plc, false)

		if err != nil {
			addTemplateErrorViolation("", err.Error())

			return
		}

		tmplResolverCfg.EncryptionConfig = encryptionConfig
	}

	kubeclient := kubernetes.Interface(clientSet)

	annotations := plc.GetAnnotations()
	disableTemplates := false

	if disableAnnotation, ok := annotations["policy.open-cluster-management.io/disable-templates"]; ok {
		log.V(2).Info("Found disable-templates annotation", "value", disableAnnotation)

		parsedDisable, err := strconv.ParseBool(disableAnnotation)
		if err != nil {
			log.Error(err, "Could not parse value for disable-templates annotation", "value", disableAnnotation)
		} else {
			disableTemplates = parsedDisable
		}
	}

	tmplResolver, err := templates.NewResolver(&kubeclient, config, tmplResolverCfg)
	if err != nil {
		// If the encryption key is invalid, clear the cache.
		if errors.Is(err, templates.ErrInvalidAESKey) || errors.Is(err, templates.ErrAESKeyNotSet) {
			r.cachedEncryptionKey = &cachedEncryptionKey{}
		}

		msg := getTemplateConfigErrorMsg(err)
		addTemplateErrorViolation("", msg)

		return
	}

	log.V(2).Info("Processing the object templates", "count", len(plc.Spec.ObjectTemplates))

	for indx, objectT := range plc.Spec.ObjectTemplates {
		nonCompliantObjects := map[string]map[string]interface{}{}
		compliantObjects := map[string]map[string]interface{}{}
		enforce := strings.EqualFold(string(plc.Spec.RemediationAction), string(policyv1.Enforce))
		// This will be overridden if a namespace is present in the object template
		relevantNamespaces := plcNamespaces
		kind := ""
		desiredName := ""
		objShouldExist := !strings.EqualFold(string(objectT.ComplianceType), string(policyv1.MustNotHave))

		var unstruct unstructured.Unstructured
		unstruct.Object = make(map[string]interface{})

		// Here appears to be a  good place to hook in template processing
		// This is at the head of objectemplate processing
		// ( just before the perNamespace handling of objectDefinitions)

		// check here to determine if the object definition has a template
		// and execute  template-processing only if  there is a template pattern "{{" in it
		// to avoid unnecessary parsing when there is no template in the definition.

		if !disableTemplates {
			// first check to make sure there are no hub-templates with delimiter - {{hub
			// if one exists, it means the template resolution on the hub did not succeed.
			if templates.HasTemplate(objectT.ObjectDefinition.Raw, "{{hub", false) {
				// check to see there is an annotation set to the hub error msg,
				// if not ,set a generic msg
				hubTemplatesErrMsg, ok := annotations["policy.open-cluster-management.io/hub-templates-error"]
				if !ok || hubTemplatesErrMsg == "" {
					// set a generic msg
					hubTemplatesErrMsg = "Error occurred while processing hub-templates, " +
						"check the policy events for more details."
				}

				log.Info(
					"An error occurred while processing hub-templates on the Hub cluster. Cannot process the policy.",
					"message", hubTemplatesErrMsg,
				)

				addTemplateErrorViolation("Error processing hub templates", hubTemplatesErrMsg)

				return
			}

			if templates.HasTemplate(objectT.ObjectDefinition.Raw, "", true) {
				log.V(1).Info("Processing policy templates")

				resolvedTemplate, tplErr := tmplResolver.ResolveTemplate(objectT.ObjectDefinition.Raw, nil)

				// If the error is because the padding is invalid, this either means the encrypted value was not
				// generated by the "protect" template function or the AES key is incorrect. Control for a stale
				// cached key.
				if usedKeyCache && errors.Is(tplErr, templates.ErrInvalidPKCS7Padding) {
					log.V(2).Info(
						"The template decryption failed likely due to an invalid encryption key, will refresh " +
							"the encryption key cache and try the decryption again",
					)
					var encryptionConfig templates.EncryptionConfig
					encryptionConfig, usedKeyCache, err = r.getEncryptionConfig(plc, true)

					if err != nil {
						addTemplateErrorViolation("", err.Error())

						return
					}

					tmplResolverCfg.EncryptionConfig = encryptionConfig

					err := tmplResolver.SetEncryptionConfig(encryptionConfig)
					if err != nil {
						// If the encryption key is invalid, clear the cache.
						if errors.Is(err, templates.ErrInvalidAESKey) || errors.Is(err, templates.ErrAESKeyNotSet) {
							r.cachedEncryptionKey = &cachedEncryptionKey{}
						}

						msg := getTemplateConfigErrorMsg(err)
						addTemplateErrorViolation("", msg)

						return
					}

					resolvedTemplate, tplErr = tmplResolver.ResolveTemplate(objectT.ObjectDefinition.Raw, nil)
				}

				if tplErr != nil {
					addTemplateErrorViolation("", tplErr.Error())

					return
				}

				// Set the resolved data for use in further processing
				objectT.ObjectDefinition.Raw = resolvedTemplate
			}
		}

		var blob interface{}
		if jsonErr := json.Unmarshal(objectT.ObjectDefinition.Raw, &blob); jsonErr != nil {
			log.Error(jsonErr, "Could not unmarshal data from JSON")

			return
		}

		// pull metadata out of the object template
		//nolint:forcetypeassert
		unstruct.Object = blob.(map[string]interface{})
		if md, ok := unstruct.Object["metadata"]; ok {
			//nolint:forcetypeassert
			metadata := md.(map[string]interface{})

			if objectns, ok := metadata["namespace"]; ok {
				relevantNamespaces = []string{objectns.(string)}
				log.V(2).Info(
					"The applicable namespace is limited to what is defined in the object template",
					"namespace", relevantNamespaces[0],
					"index", indx,
				)
			} else {
				log.V(2).Info("metadata.namespace is not set in the object template", "index", indx)
			}

			if objectname, ok := metadata["name"]; ok {
				//nolint:forcetypeassert
				desiredName = objectname.(string)
			} else {
				log.V(2).Info("metadata.name is not set in the object template", "index", indx)
			}
		}

		numCompliant := 0
		numNonCompliant := 0
		handled := false
		objNamespaced := false

		// iterate through all namespaces the configurationpolicy is set on
		for _, ns := range relevantNamespaces {
			log.Info(
				"Handling the object template for the relevant namespace",
				"namespace", ns,
				"desiredName", desiredName,
				"index", indx,
			)

			names, compliant, reason, objKind, related, update, namespaced := r.handleObjects(
				objectT, ns, indx, &plc, apiresourcelist, apigroups)
			if update {
				parentUpdate = true
			}

			if objKind != "" {
				kind = objKind
			}

			if namespaced {
				objNamespaced = true
			}

			if names == nil {
				// object template enforced, already handled in handleObjects
				handled = true
			} else {
				enforce = false
				if !compliant {
					if len(names) == 0 {
						numNonCompliant++
					} else {
						numNonCompliant += len(names)
					}
					nonCompliantObjects[ns] = map[string]interface{}{
						"names":  names,
						"reason": reason,
					}
				} else {
					numCompliant += len(names)
					compliantObjects[ns] = map[string]interface{}{
						"names":  names,
						"reason": reason,
					}
				}
			}

			for _, object := range related {
				relatedObjects = updateRelatedObjectsStatus(relatedObjects, object)
			}
		}
		// violations for enforce configurationpolicies are already handled in handleObjects,
		// so we only need to generate a violation if the remediationAction is set to inform
		if !handled && !enforce {
			objData := map[string]interface{}{
				"indx":        indx,
				"kind":        kind,
				"desiredName": desiredName,
				"namespaced":  objNamespaced,
			}

			statusUpdate := createInformStatus(objShouldExist, numCompliant, numNonCompliant,
				compliantObjects, nonCompliantObjects, &plc, objData)
			if statusUpdate {
				parentUpdate = true
			}
		}
	}

	r.checkRelatedAndUpdate(parentUpdate, plc, relatedObjects, oldRelated)
}

// checks related objects field and triggers an update on the configurationpolicy if it has changed
func (r *ConfigurationPolicyReconciler) checkRelatedAndUpdate(
	update bool, plc policyv1.ConfigurationPolicy, related, oldRelated []policyv1.RelatedObject,
) {
	sortUpdate := sortRelatedObjectsAndUpdate(&plc, related, oldRelated)
	if update || sortUpdate {
		r.addForUpdate(&plc)
	}
}

// helper function to check whether related objects has changed
func sortRelatedObjectsAndUpdate(
	plc *policyv1.ConfigurationPolicy, related, oldRelated []policyv1.RelatedObject,
) (updateNeeded bool) {
	sort.SliceStable(related, func(i, j int) bool {
		if related[i].Object.Kind != related[j].Object.Kind {
			return related[i].Object.Kind < related[j].Object.Kind
		}
		if related[i].Object.Metadata.Namespace != related[j].Object.Metadata.Namespace {
			return related[i].Object.Metadata.Namespace < related[j].Object.Metadata.Namespace
		}

		return related[i].Object.Metadata.Name < related[j].Object.Metadata.Name
	})

	update := false

	if len(oldRelated) == len(related) {
		for i, entry := range oldRelated {
			if !gocmp.Equal(entry, related[i]) {
				update = true
			}
		}
	} else {
		update = true
	}

	if update {
		plc.Status.RelatedObjects = related
	}

	return update
}

// helper function that appends a condition (violation or compliant) to the status of a configurationpolicy
func addConditionToStatus(
	plc *policyv1.ConfigurationPolicy, index int, compliant bool, reason string, message string,
) (updateNeeded bool) {
	cond := &policyv1.Condition{
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	var complianceState policyv1.ComplianceState

	if compliant {
		complianceState = policyv1.Compliant
		cond.Type = "notification"
	} else {
		complianceState = policyv1.NonCompliant
		cond.Type = "violation"
	}

	var update bool

	if len(plc.Status.CompliancyDetails) <= index {
		plc.Status.CompliancyDetails = append(plc.Status.CompliancyDetails, policyv1.TemplateStatus{
			ComplianceState: complianceState,
			Conditions:      []policyv1.Condition{},
		})
	}

	if plc.Status.CompliancyDetails[index].ComplianceState != complianceState {
		update = true
	}

	plc.Status.CompliancyDetails[index].ComplianceState = complianceState

	// do not add condition unless it does not already appear in the status
	if !checkMessageSimilarity(plc.Status.CompliancyDetails[index].Conditions, cond) {
		conditions := AppendCondition(plc.Status.CompliancyDetails[index].Conditions, cond, "", false)
		plc.Status.CompliancyDetails[index].Conditions = conditions
		update = true
	}

	if update {
		log.Info("Will update the policy status", "policy", plc.GetName(), "complianceState", complianceState)
	}

	return update
}

// createInformStatus updates the status field for a configurationpolicy with remediationAction=inform
// based on how many compliant/noncompliant objects are found when processing the templates in the configurationpolicy
func createInformStatus(
	objShouldExist bool,
	numCompliant,
	numNonCompliant int,
	compliantObjects,
	nonCompliantObjects map[string]map[string]interface{},
	plc *policyv1.ConfigurationPolicy,
	objData map[string]interface{},
) (updateNeeded bool) {
	//nolint:forcetypeassert
	desiredName := objData["desiredName"].(string)
	//nolint:forcetypeassert
	indx := objData["indx"].(int)
	//nolint:forcetypeassert
	kind := objData["kind"].(string)
	//nolint:forcetypeassert
	namespaced := objData["namespaced"].(bool)

	if kind == "" {
		return false
	}

	var compObjs map[string]map[string]interface{}
	var compliant bool

	if numNonCompliant > 0 {
		compliant = false
		compObjs = nonCompliantObjects
	} else if objShouldExist && numCompliant == 0 {
		// Special case: No resources found is NonCompliant
		compliant = false
		compObjs = nonCompliantObjects
	} else {
		compliant = true
		compObjs = compliantObjects
	}

	return createStatus(desiredName, kind, compObjs, namespaced, plc, indx, compliant, objShouldExist)
}

// handleObjects controls the processing of each individual object template within a configurationpolicy
func (r *ConfigurationPolicyReconciler) handleObjects(
	objectT *policyv1.ObjectTemplate,
	namespace string,
	index int,
	policy *policyv1.ConfigurationPolicy,
	apiresourcelist []*metav1.APIResourceList,
	apigroups []*restmapper.APIGroupResources,
) (
	objNameList []string,
	compliant bool,
	reason string,
	rsrcKind string,
	relatedObjects []policyv1.RelatedObject,
	pUpdate bool,
	isNamespaced bool,
) {
	log := log.WithValues("policy", policy.GetName(), "index", index, "objectNamespace", namespace)

	if namespace != "" {
		log.V(2).Info("Handling object template")
	} else {
		log.V(2).Info("Handling object template, no namespace specified")
	}

	namespaced := true
	ext := objectT.ObjectDefinition

	// map raw object to a resource, generate a violation if resource cannot be found
	mapping, mappingUpdate := r.getMapping(apigroups, ext, policy, index)
	if mapping == nil {
		return nil, false, "", "", nil, mappingUpdate, namespaced
	}

	var unstruct unstructured.Unstructured
	unstruct.Object = make(map[string]interface{})

	var blob interface{}
	if err := json.Unmarshal(ext.Raw, &blob); err != nil {
		log.Error(err, "Could not unmarshal data from JSON")
		os.Exit(1)
	}

	//nolint:forcetypeassert
	unstruct.Object = blob.(map[string]interface{}) // set object to the content of the blob after Unmarshalling
	exists := true
	objNames := []string{}
	remediation := policy.Spec.RemediationAction

	name, kind, metaNamespace := getDetails(unstruct)
	if metaNamespace != "" {
		namespace = metaNamespace
		log = log.WithValues("namespace", metaNamespace)

		log.V(2).Info("Overridding the namespace with what is defined in the object template")
	}

	statusUpdateNeeded := false

	dclient, rsrc, namespaced := getResourceAndDynamicClient(mapping, apiresourcelist)
	if namespaced && namespace == "" {
		log.Info("The object template is namespaced but no namespace is specified. Cannot process.")
		// namespaced but none specified, generate violation
		updateStatus := addConditionToStatus(policy, index, false, "K8s missing namespace",
			"namespaced object has no namespace specified")
		if updateStatus {
			eventType := eventNormal
			if index < len(policy.Status.CompliancyDetails) &&
				policy.Status.CompliancyDetails[index].ComplianceState == policyv1.NonCompliant {
				eventType = eventWarning
			}

			r.Recorder.Event(policy, eventType, fmt.Sprintf(eventFmtStr, policy.GetName(), name),
				convertPolicyStatusToString(policy))

			statusUpdateNeeded = true
		}

		return nil, false, "", "", nil, statusUpdateNeeded, namespaced
	}

	if name != "" { // named object, so checking just for the existence of the specific object
		exists = objectExists(namespaced, namespace, name, rsrc, dclient)
		objNames = append(objNames, name)
	} else if kind != "" { // no name, so we are checking for the existence of any object of this kind
		log.V(1).Info(
			"The object template does not specify a name. Will search for matching objects in the namespace.",
		)
		objNames = append(objNames, getNamesOfKind(unstruct, rsrc, namespaced,
			namespace, dclient, strings.ToLower(string(objectT.ComplianceType)))...)

		// we do not support enforce on unnamed templates
		if !strings.EqualFold(string(remediation), "inform") {
			log.Info(
				"The object template does not specify a name. Setting the remediation action to inform.",
				"oldRemediationAction", remediation,
			)
		}
		remediation = "inform"

		if len(objNames) == 0 {
			exists = false
		}
	}

	objShouldExist := !strings.EqualFold(string(objectT.ComplianceType), string(policyv1.MustNotHave))
	rsrcKind = rsrc.Resource

	if len(objNames) == 1 {
		name = objNames[0]
		singObj := singleObject{
			policy:      policy,
			gvr:         rsrc,
			name:        name,
			namespace:   namespace,
			namespaced:  namespaced,
			shouldExist: objShouldExist,
			index:       index,
			unstruct:    unstruct,
		}

		log.V(2).Info("Handling a single object template")

		objNames, compliant, rsrcKind, statusUpdateNeeded = r.handleSingleObj(
			singObj, remediation, exists, dclient, objectT,
		)
		// The message string for single objects is different than for multiple
		reason = generateSingleObjReason(objShouldExist, compliant, exists)
		// Enforce could clear the objNames array so use name instead
		relatedObjects = addRelatedObjects(compliant, rsrc, namespace, namespaced, []string{name}, reason)

		if !statusUpdateNeeded {
			log.V(2).Info("The status did not change for this object template")
		}
	} else { // This case only occurs when the desired object is not named
		if objShouldExist {
			if exists {
				compliant = true
				reason = reasonWantFoundExists
			} else {
				compliant = false
				reason = reasonWantFoundDNE
			}
		} else {
			if exists {
				compliant = false
				reason = reasonWantNotFoundExists
			} else {
				compliant = true
				reason = reasonWantNotFoundDNE
			}
		}

		relatedObjects = addRelatedObjects(compliant, rsrc, namespace, namespaced, objNames, reason)
	}

	return objNames, compliant, reason, rsrcKind, relatedObjects, statusUpdateNeeded, namespaced
}

// generateSingleObjReason is a helper function to create a compliant/noncompliant message for a named object
func generateSingleObjReason(objShouldExist bool, compliant bool, exists bool) (rsn string) {
	reason := ""

	if objShouldExist && compliant {
		reason = reasonWantFoundExists
	} else if objShouldExist && !compliant && exists {
		reason = reasonWantFoundNoMatch
	} else if objShouldExist && !compliant {
		reason = reasonWantFoundDNE
	} else if !objShouldExist && compliant {
		reason = reasonWantNotFoundDNE
	} else if !objShouldExist && !compliant {
		reason = reasonWantNotFoundExists
	}

	return reason
}

type singleObject struct {
	policy      *policyv1.ConfigurationPolicy
	gvr         schema.GroupVersionResource
	name        string
	namespace   string
	namespaced  bool
	shouldExist bool
	index       int
	unstruct    unstructured.Unstructured
}

// handleSingleObj takes in an object template (for a named object) and its data and determines whether
// the object on the cluster is compliant or not
func (r *ConfigurationPolicyReconciler) handleSingleObj(
	obj singleObject,
	remediation policyv1.RemediationAction,
	exists bool,
	dclient dynamic.Interface,
	objectT *policyv1.ObjectTemplate,
) (objNameList []string, compliance bool, rsrcKind string, statusUpdateNeeded bool) {
	objLog := log.WithValues("object", obj.name, "policy", obj.policy.Name, "index", obj.index)

	var err error
	var compliant bool

	compliantObject := map[string]map[string]interface{}{
		obj.namespace: {
			"names": []string{obj.name},
		},
	}

	if !exists && obj.shouldExist {
		// it is a musthave and it does not exist, so it must be created
		if strings.EqualFold(string(remediation), string(policyv1.Enforce)) {
			statusUpdateNeeded, err = enforceByCreatingOrDeleting(obj, dclient)
			if err != nil {
				// violation created for handling error
				objLog.Error(err, "Could not handle missing musthave object")
			}
		} else { // inform
			compliant = false
		}
	}

	if exists && !obj.shouldExist {
		// it is a mustnothave but it exist, so it must be deleted
		if strings.EqualFold(string(remediation), string(policyv1.Enforce)) {
			statusUpdateNeeded, err = enforceByCreatingOrDeleting(obj, dclient)
			if err != nil {
				objLog.Error(err, "Could not handle existing mustnothave object")
			}
		} else { // inform
			compliant = false
		}
	}

	if !exists && !obj.shouldExist {
		log.V(1).Info("The object does not exist and is compliant with the mustnothave compliance type")
		// it is a must not have and it does not exist, so it is compliant
		compliant = true

		if strings.EqualFold(string(remediation), string(policyv1.Enforce)) {
			log.V(2).Info("Entering `does not exist` and `must not have`")

			statusUpdateNeeded = createStatus("", obj.gvr.Resource, compliantObject, obj.namespaced, obj.policy,
				obj.index, compliant, false)
		}
	}

	processingErr := false
	specViolation := false

	// object exists and the template requires it, so we need to check specific fields to see if we have a match
	if exists {
		log.V(2).Info("The object already exists. Verifying the object fields match what is desired.")

		compType := strings.ToLower(string(objectT.ComplianceType))
		mdCompType := strings.ToLower(string(objectT.MetadataComplianceType))
		throwSpecViolation, msg, pErr := checkAndUpdateResource(obj, compType, mdCompType, remediation, dclient)

		if throwSpecViolation {
			specViolation = throwSpecViolation
			compliant = false
		} else if msg != "" {
			statusUpdateNeeded = addConditionToStatus(obj.policy, obj.index, false, "K8s update template error", msg)
		} else if obj.shouldExist {
			// it is a must have and it does exist, so it is compliant
			compliant = true
			if strings.EqualFold(string(remediation), string(policyv1.Enforce)) {
				statusUpdateNeeded = createStatus("", obj.gvr.Resource, compliantObject, obj.namespaced, obj.policy,
					obj.index, compliant, true)
			}
		}

		processingErr = pErr
	}

	if statusUpdateNeeded {
		eventType := eventNormal
		if obj.index < len(obj.policy.Status.CompliancyDetails) &&
			obj.policy.Status.CompliancyDetails[obj.index].ComplianceState == policyv1.NonCompliant {
			eventType = eventWarning
			compliant = false
		}

		statusStr := convertPolicyStatusToString(obj.policy)
		log.V(1).Info("Sending an update policy status event", "policy", obj.policy.Name, "status", statusStr)
		r.Recorder.Event(obj.policy, eventType, fmt.Sprintf(eventFmtStr, obj.policy.GetName(), obj.name), statusStr)

		return nil, compliant, "", statusUpdateNeeded
	}

	if processingErr {
		return nil, false, "", false
	}

	if strings.EqualFold(string(remediation), string(policyv1.Inform)) || specViolation {
		return []string{obj.name}, compliant, obj.gvr.Resource, false
	}

	return nil, compliant, "", false
}

// getResourceAndDynamicClient creates a dynamic client to query resources and pulls the groupVersionResource
// for an object from its mapping, as well as checking whether the resource is namespaced or cluster-level
func getResourceAndDynamicClient(
	mapping *meta.RESTMapping,
	apiresourcelist []*metav1.APIResourceList,
) (dclient dynamic.Interface, rsrc schema.GroupVersionResource, namespaced bool) {
	namespaced = false
	restconfig := config
	restconfig.GroupVersion = &schema.GroupVersion{
		Group:   mapping.GroupVersionKind.Group,
		Version: mapping.GroupVersionKind.Version,
	}

	dclient, err := dynamic.NewForConfig(restconfig)
	if err != nil {
		log.Error(err, "Could not get dynamic client from config", "config", restconfig)
		os.Exit(1)
	}

	// check all resources in the list of resources on the cluster to get a match for the mapping
	rsrc = mapping.Resource

	for _, apiresourcegroup := range apiresourcelist {
		if apiresourcegroup.GroupVersion == buildGV(mapping.GroupVersionKind.Group, mapping.GroupVersionKind.Version) {
			for _, apiresource := range apiresourcegroup.APIResources {
				if apiresource.Name == mapping.Resource.Resource && apiresource.Kind == mapping.GroupVersionKind.Kind {
					namespaced = apiresource.Namespaced
					log.V(2).Info("Found resource in apiresourcelist", "namespaced", namespaced, "resource", rsrc)
				}
			}
		}
	}

	return dclient, rsrc, namespaced
}

// getMapping takes in a raw object, decodes it, and maps it to an existing group/kind
func (r *ConfigurationPolicyReconciler) getMapping(
	apigroups []*restmapper.APIGroupResources,
	ext runtime.RawExtension,
	policy *policyv1.ConfigurationPolicy,
	index int,
) (mapping *meta.RESTMapping, update bool) {
	log := log.WithValues("policy", policy.GetName(), "index", index)
	updateNeeded := false
	restmapper := restmapper.NewDiscoveryRESTMapper(apigroups)

	_, gvk, err := unstructured.UnstructuredJSONScheme.Decode(ext.Raw, nil, nil)
	if err != nil {
		// generate violation if object cannot be decoded and update the configpolicy
		decodeErr := fmt.Sprintf("Decoding error, please check your policy file!"+
			" Aborting handling the object template at index [%v] in policy `%v` with error = `%v`",
			index, policy.Name, err)

		log.Error(err, "Could not decode object")

		if len(policy.Status.CompliancyDetails) <= index {
			policy.Status.CompliancyDetails = append(policy.Status.CompliancyDetails, policyv1.TemplateStatus{
				ComplianceState: policyv1.NonCompliant,
				Conditions:      []policyv1.Condition{},
			})
		}

		policy.Status.CompliancyDetails[index].ComplianceState = policyv1.NonCompliant
		policy.Status.CompliancyDetails[index].Conditions = []policyv1.Condition{
			{
				Type:               "violation",
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Reason:             "K8s decode object definition error",
				Message:            decodeErr,
			},
		}

		return nil, true
	}

	// initializes a mapping between Kind and APIVersion to a resource name
	mapping, err = restmapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	mappingErrMsg := ""

	if err != nil {
		// if the restmapper fails to find a mapping to a resource, generate a violation and update the configpolicy
		prefix := "no matches for kind \""
		startIdx := strings.Index(err.Error(), prefix)

		if startIdx == -1 {
			log.Error(err, "Could not identify mapping error from raw object", "gvk", gvk)
		} else {
			afterPrefix := err.Error()[(startIdx + len(prefix)):len(err.Error())]
			kind := afterPrefix[0:(strings.Index(afterPrefix, "\" "))]
			mappingErrMsg = "couldn't find mapping resource with kind " + kind +
				", please check if you have CRD deployed"
			log.Error(err, "Could not map resource, do you have the CRD deployed?", "kind", kind)
		}

		errMsg := err.Error()
		if mappingErrMsg != "" {
			errMsg = mappingErrMsg
			cond := &policyv1.Condition{
				Type:               "violation",
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.Now(),
				Reason:             "K8s creation error",
				Message:            mappingErrMsg,
			}

			if len(policy.Status.CompliancyDetails) <= index {
				policy.Status.CompliancyDetails = append(policy.Status.CompliancyDetails, policyv1.TemplateStatus{
					ComplianceState: policyv1.NonCompliant,
					Conditions:      []policyv1.Condition{},
				})
			}

			if policy.Status.CompliancyDetails[index].ComplianceState != policyv1.NonCompliant {
				updateNeeded = true
			}

			policy.Status.CompliancyDetails[index].ComplianceState = policyv1.NonCompliant

			if !checkMessageSimilarity(policy.Status.CompliancyDetails[index].Conditions, cond) {
				conditions := AppendCondition(policy.Status.CompliancyDetails[index].Conditions,
					cond, gvk.GroupKind().Kind, false)
				policy.Status.CompliancyDetails[index].Conditions = conditions
				updateNeeded = true
			}
		}

		if updateNeeded {
			// generate an event on the configurationpolicy if a violation is created
			r.Recorder.Event(policy, eventWarning, fmt.Sprintf(plcFmtStr, policy.GetName()), errMsg)
		}

		return nil, updateNeeded
	}

	log.V(2).Info(
		"Found the API mapping for the object template",
		"group", gvk.Group,
		"version", gvk.Version,
		"kind", gvk.Kind,
	)

	return mapping, updateNeeded
}

func getDetails(unstruct unstructured.Unstructured) (name string, kind string, namespace string) {
	name = ""
	kind = ""
	namespace = ""

	if md, ok := unstruct.Object["metadata"]; ok {
		//nolint:forcetypeassert
		metadata := md.(map[string]interface{})
		if objectName, ok := metadata["name"]; ok {
			name = strings.TrimSpace(objectName.(string))
		}
		// override the namespace if specified in objectTemplates
		if objectns, ok := metadata["namespace"]; ok {
			namespace = strings.TrimSpace(objectns.(string))

			log.V(2).Info("Overrode namespace since it is specified in objectTemplates",
				"name", name, "namespace", namespace)
		}
	}

	if objKind, ok := unstruct.Object["kind"]; ok {
		kind = strings.TrimSpace(objKind.(string))
	}

	return name, kind, namespace
}

// buildNameList is a helper function to pull names of resources that match an objectTemplate from a list of resources
func buildNameList(
	unstruct unstructured.Unstructured, complianceType string, resList *unstructured.UnstructuredList,
) (kindNameList []string) {
	for i := range resList.Items {
		uObj := resList.Items[i]
		match := true

		for key := range unstruct.Object {
			// if any key in the object generates a mismatch, the object does not match the template and we
			// do not add its name to the list
			errorMsg, updateNeeded, _, skipped := handleSingleKey(key, unstruct, &uObj, complianceType)
			if !skipped {
				if errorMsg != "" || updateNeeded {
					match = false
				}
			}
		}

		if match {
			kindNameList = append(kindNameList, uObj.Object["metadata"].(map[string]interface{})["name"].(string))
		}
	}

	return kindNameList
}

// getNamesOfKind returns an array with names of all of the resources found
// matching the GVK specified.
func getNamesOfKind(
	unstruct unstructured.Unstructured,
	rsrc schema.GroupVersionResource,
	namespaced bool,
	ns string,
	dclient dynamic.Interface,
	complianceType string,
) (kindNameList []string) {
	if namespaced {
		res := dclient.Resource(rsrc).Namespace(ns)

		resList, err := res.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Error(err, "Could not list resources", "rsrc", rsrc, "namespaced", namespaced)

			return kindNameList
		}

		return buildNameList(unstruct, complianceType, resList)
	}

	res := dclient.Resource(rsrc)

	resList, err := res.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error(err, "Could not list resources", "rsrc", rsrc, "namespaced", namespaced)

		return kindNameList
	}

	return buildNameList(unstruct, complianceType, resList)
}

// enforceByCreatingOrDeleting can handle the situation where a musthave or mustonlyhave object is
// completely missing (as opposed to existing, but not matching the desired state), or where a
// mustnothave object does exist. Eg, it does not handle the case where a targeted update would need
// to be made to an object.
func enforceByCreatingOrDeleting(obj singleObject, dclient dynamic.Interface) (result bool, erro error) {
	log := log.WithValues(
		"object", obj.name,
		"policy", obj.policy.Name,
		"objectNamespace", obj.namespace,
		"objectTemplateIndex", obj.index,
	)
	idStr := identifierStr([]string{obj.name}, obj.namespace, obj.namespaced)

	var res dynamic.ResourceInterface
	if obj.namespaced {
		res = dclient.Resource(obj.gvr).Namespace(obj.namespace)
	} else {
		res = dclient.Resource(obj.gvr)
	}

	var completed bool
	var reason, msg string
	var err error

	if obj.shouldExist {
		log.Info("Enforcing the policy by creating the object")

		if completed, err = createObject(res, obj.unstruct); !completed {
			reason = "K8s creation error"
			msg = fmt.Sprintf("%v %v is missing, and cannot be created, reason: `%v`", obj.gvr.Resource, idStr, err)
		} else {
			log.V(2).Info("Created missing must have object", "resource", obj.gvr.Resource, "name", obj.name)
			reason = "K8s creation success"
			msg = fmt.Sprintf("%v %v was missing, and was created successfully", obj.gvr.Resource, idStr)
		}
	} else {
		log.Info("Enforcing the policy by deleting the object")

		if completed, err = deleteObject(res, obj.name, obj.namespace); completed {
			reason = "K8s deletion error"
			msg = fmt.Sprintf("%v %v exists, and cannot be deleted, reason: `%v`", obj.gvr.Resource, idStr, err)
		} else {
			reason = "K8s deletion success"
			msg = fmt.Sprintf("%v %v existed, and was deleted successfully", obj.gvr.Resource, idStr)
		}
	}

	return addConditionToStatus(obj.policy, obj.index, completed, reason, msg), err
}

func getPolicyNamespaces(policy policyv1.ConfigurationPolicy) []string {
	// get all namespaces
	allNamespaces := getAllNamespaces()
	// then get the list of included
	includedNamespaces := []string{}
	included := policy.Spec.NamespaceSelector.Include

	for _, value := range included {
		found := common.FindPattern(string(value), allNamespaces)
		if found != nil {
			includedNamespaces = append(includedNamespaces, found...)
		}
	}

	// then get the list of excluded
	excludedNamespaces := []string{}
	excluded := policy.Spec.NamespaceSelector.Exclude

	for _, value := range excluded {
		found := common.FindPattern(string(value), allNamespaces)
		if found != nil {
			excludedNamespaces = append(excludedNamespaces, found...)
		}
	}

	// then get the list of deduplicated
	finalList := common.DeduplicateItems(includedNamespaces, excludedNamespaces)
	if len(finalList) == 0 {
		finalList = append(finalList, "")
	}

	return finalList
}

func getAllNamespaces() (list []string) {
	listOpt := &metav1.ListOptions{}

	nsList, err := clientSet.CoreV1().Namespaces().List(context.TODO(), *listOpt)
	if err != nil {
		log.Error(err, "Could not list namespaces from the API server")
	}

	namespacesNames := []string{}

	for _, n := range nsList.Items {
		namespacesNames = append(namespacesNames, n.Name)
	}

	return namespacesNames
}

// checkMessageSimilarity decides whether to append a new condition to a configurationPolicy status
// based on whether it is too similar to the previous one
func checkMessageSimilarity(conditions []policyv1.Condition, cond *policyv1.Condition) bool {
	same := true
	lastIndex := len(conditions)

	if lastIndex > 0 {
		oldCond := conditions[lastIndex-1]
		if !IsSimilarToLastCondition(oldCond, *cond) {
			same = false
		}
	} else {
		same = false
	}

	return same
}

// objectExists gets object with dynamic client, returns true if the object is found
func objectExists(
	namespaced bool,
	namespace string,
	name string,
	rsrc schema.GroupVersionResource,
	dclient dynamic.Interface,
) (result bool) {
	objLog := log.WithValues("name", name, "namespaced", namespaced, "namespace", namespace)
	objLog.V(2).Info("Checking if the object exists")

	var res dynamic.ResourceInterface
	if namespaced {
		res = dclient.Resource(rsrc).Namespace(namespace)
	} else {
		res = dclient.Resource(rsrc)
	}

	_, err := res.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			objLog.V(2).Info("Got 'Not Found' response for object from the API server")
		} else {
			objLog.Error(err, "Could not retrieve object from the API server")
		}

		return false
	}

	objLog.V(2).Info("Retrieved object from the API server")

	return true
}

func createObject(res dynamic.ResourceInterface, unstruct unstructured.Unstructured) (created bool, err error) {
	objLog := log.WithValues("name", unstruct.GetName(), "namespace", unstruct.GetNamespace())
	objLog.V(2).Info("Entered createObject", "unstruct", unstruct)

	_, err = res.Create(context.TODO(), &unstruct, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			objLog.V(2).Info("Got 'Already Exists' response for object")

			return true, err
		}

		objLog.Error(err, "Could not create object", "reason", k8serrors.ReasonForError(err))

		return false, err
	}

	objLog.V(2).Info("Resource created")

	return true, nil
}

func deleteObject(res dynamic.ResourceInterface, name, namespace string) (deleted bool, err error) {
	objLog := log.WithValues("name", name, "namespace", namespace)
	objLog.V(2).Info("Entered deleteObject")

	err = res.Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			objLog.V(2).Info("Got 'Not Found' response while deleting object")

			return true, err
		}

		objLog.Error(err, "Could not delete object")

		return false, err
	}

	objLog.V(2).Info("Deleted object")

	return true, nil
}

// mergeSpecs is a wrapper for the recursive function to merge 2 maps. It marshals the objects into JSON
// to make sure they are valid objects before calling the merge function
func mergeSpecs(templateVal, existingVal interface{}, ctype string) (interface{}, error) {
	data1, err := json.Marshal(templateVal)
	if err != nil {
		return nil, err
	}

	data2, err := json.Marshal(existingVal)
	if err != nil {
		return nil, err
	}

	var j1, j2 interface{}

	err = json.Unmarshal(data1, &j1)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data2, &j2)
	if err != nil {
		return nil, err
	}

	return mergeSpecsHelper(j1, j2, ctype), nil
}

// mergeSpecsHelper is a helper function that takes an object from the existing object and merges in
// all the data that is different in the template. This way, comparing the merged object to the one
// that exists on the cluster will tell you whether the existing object is compliant with the template.
// This function uses recursion to check mismatches in nested objects and is the basis for most
// comparisons the controller makes.
func mergeSpecsHelper(templateVal, existingVal interface{}, ctype string) interface{} {
	switch templateVal := templateVal.(type) {
	case map[string]interface{}:
		existingVal, ok := existingVal.(map[string]interface{})
		if !ok {
			// if one field is a map and the other isn't, don't bother merging -
			// just returning the template value will still generate noncompliant
			return templateVal
		}
		// otherwise, iterate through all fields in the template object and
		// merge in missing values from the existing object
		for k, v2 := range existingVal {
			if v1, ok := templateVal[k]; ok {
				templateVal[k] = mergeSpecsHelper(v1, v2, ctype)
			} else {
				templateVal[k] = v2
			}
		}
	case []interface{}: // list nested in map
		if !isSorted(templateVal) {
			// arbitrary sort on template value for easier comparison
			sort.Slice(templateVal, func(i, j int) bool {
				return fmt.Sprintf("%v", templateVal[i]) < fmt.Sprintf("%v", templateVal[j])
			})
		}

		existingVal, ok := existingVal.([]interface{})
		if !ok {
			// if one field is a list and the other isn't, don't bother merging
			return templateVal
		}

		if len(existingVal) > 0 {
			// if both values are non-empty lists, we need to merge in the extra data in the existing
			// object to do a proper compare
			return mergeArrays(templateVal, existingVal, ctype)
		}
	case nil:
		// if template value is nil, pull data from existing, since the template does not care about it
		existingVal, ok := existingVal.(map[string]interface{})
		if ok {
			return existingVal
		}
	}

	_, ok := templateVal.(string)
	if !ok {
		return templateVal
	}

	return templateVal.(string)
}

// isSorted is a helper function that checks whether an array is sorted
func isSorted(arr []interface{}) (result bool) {
	arrCopy := append([]interface{}{}, arr...)
	sort.Slice(arr, func(i, j int) bool {
		return fmt.Sprintf("%v", arr[i]) < fmt.Sprintf("%v", arr[j])
	})

	return fmt.Sprint(arrCopy) == fmt.Sprint(arr)
}

// mergeArrays is a helper function that takes a list from the existing object and merges in all the data that is
// different in the template. This way, comparing the merged object to the one that exists on the cluster will tell
// you whether the existing object is compliant with the template
func mergeArrays(newArr []interface{}, old []interface{}, ctype string) (result []interface{}) {
	if ctype == "mustonlyhave" {
		return newArr
	}

	newArrCopy := append([]interface{}{}, newArr...)
	idxWritten := map[int]bool{}

	for i := range newArrCopy {
		idxWritten[i] = false
	}

	// create a set with a key for each unique item in the list
	oldItemSet := map[string]map[string]interface{}{}
	for _, val2 := range old {
		if entry, ok := oldItemSet[fmt.Sprint(val2)]; ok {
			oldItemSet[fmt.Sprint(val2)]["count"] = entry["count"].(int) + 1
		} else {
			oldItemSet[fmt.Sprint(val2)] = map[string]interface{}{
				"count": 1,
				"value": val2,
			}
		}
	}

	for _, data := range oldItemSet {
		count := 0
		reqCount := data["count"]
		val2 := data["value"]
		// for each list item in the existing array, iterate through the template array and try to find a match
		for newArrIdx, val1 := range newArrCopy {
			if idxWritten[newArrIdx] {
				continue
			}

			var mergedObj interface{}

			switch val2 := val2.(type) {
			case map[string]interface{}:
				// use map compare helper function to check equality on lists of maps
				mergedObj, _ = compareSpecs(val1.(map[string]interface{}), val2, ctype)
			default:
				mergedObj = val1
			}
			// if a match is found, this field is already in the template, so we can skip it in future checks
			if equalObjWithSort(mergedObj, val2) {
				count++

				newArr[newArrIdx] = mergedObj
				idxWritten[newArrIdx] = true
			}
		}
		// if an item in the existing object cannot be found in the template, we add it to the template array
		// to produce the merged array
		if count < reqCount.(int) {
			for i := 0; i < (reqCount.(int) - count); i++ {
				newArr = append(newArr, val2)
			}
		}
	}

	return newArr
}

// compareLists is a wrapper function that creates a merged list for musthave
// and returns the template list for mustonlyhave
func compareLists(newList []interface{}, oldList []interface{}, ctype string) (updatedList []interface{}, err error) {
	if ctype != "mustonlyhave" {
		return mergeArrays(newList, oldList, ctype), nil
	}

	// mustonlyhave scenario: go through existing list and merge everything in order
	// then add all other template items in order afterward
	mergedList := []interface{}{}

	for idx, item := range newList {
		if idx < len(oldList) {
			newItem, err := mergeSpecs(item, oldList[idx], ctype)
			if err != nil {
				return nil, err
			}

			mergedList = append(mergedList, newItem)
		} else {
			mergedList = append(mergedList, item)
		}
	}

	return mergedList, nil
}

// compareSpecs is a wrapper function that creates a merged map for mustHave
// and returns the template map for mustonlyhave
func compareSpecs(
	newSpec, oldSpec map[string]interface{}, ctype string,
) (updatedSpec map[string]interface{}, err error) {
	if ctype == "mustonlyhave" {
		return newSpec, nil
	}
	// if compliance type is musthave, create merged object to compare on
	merged, err := mergeSpecs(newSpec, oldSpec, ctype)
	if err != nil {
		return merged.(map[string]interface{}), err
	}

	return merged.(map[string]interface{}), nil
}

// handleSingleKey checks whether a key/value pair in an object template matches with that in the existing
// resource on the cluster
func handleSingleKey(
	key string, desiredObj unstructured.Unstructured, existingObj *unstructured.Unstructured, complianceType string,
) (errormsg string, update bool, merged interface{}, skip bool) {
	log := log.WithValues("name", existingObj.GetName(), "namespace", existingObj.GetNamespace())
	var err error

	updateNeeded := false

	if isDenylisted(key) {
		log.V(2).Info("Ignoring the key since it is deny listed", "key", key)

		return "", false, nil, true
	}

	desiredValue := formatTemplate(desiredObj, key)
	existingValue := existingObj.UnstructuredContent()[key]
	typeErr := ""

	// We will compare the existing field to a "merged" field which has the fields in the template
	// merged into the existing object to avoid erroring on fields that are not in the template
	// but have been automatically added to the object.
	// For the mustOnlyHave complianceType, this object is identical to the field in the template.
	var mergedValue interface{}

	switch desiredValue := desiredValue.(type) {
	case []interface{}:
		switch existingValue := existingValue.(type) {
		case []interface{}:
			mergedValue, err = compareLists(desiredValue, existingValue, complianceType)
		case nil:
			mergedValue = desiredValue
		default:
			typeErr = fmt.Sprintf(
				"Error merging changes into key \"%s\": object type of template and existing do not match",
				key)
		}
	case map[string]interface{}:
		switch existingValue := existingValue.(type) {
		case map[string]interface{}:
			mergedValue, err = compareSpecs(desiredValue, existingValue, complianceType)
		case nil:
			mergedValue = desiredValue
		default:
			typeErr = fmt.Sprintf(
				"Error merging changes into key \"%s\": object type of template and existing do not match",
				key)
		}
	default: // If the field is not an object or slice, just do a basic compare
		mergedValue = desiredValue
	}

	if typeErr != "" {
		return typeErr, false, mergedValue, false
	}

	if err != nil {
		message := fmt.Sprintf("Error merging changes into %s: %s", key, err)

		return message, false, mergedValue, false
	}

	if key == "metadata" {
		// filter out autogenerated annotations that have caused compare issues in the past
		mergedValue, existingValue = fmtMetadataForCompare(
			mergedValue.(map[string]interface{}), existingValue.(map[string]interface{}))
	}

	// sort objects before checking equality to ensure they're in the same order
	if !equalObjWithSort(mergedValue, existingValue) {
		updateNeeded = true
	}

	return "", updateNeeded, mergedValue, false
}

// handleKeys is a helper function that calls handleSingleKey to check if each field in the template
// matches the object. If it finds a mismatch and the remediationAction is enforce, it will update
// the object with the data from the template
func handleKeys(
	unstruct unstructured.Unstructured,
	existingObj *unstructured.Unstructured,
	remediation policyv1.RemediationAction,
	complianceType string,
	mdComplianceType string,
	name string,
	res dynamic.ResourceInterface,
) (throwSpecViolation bool, message string, processingErr bool) {
	log := log.WithValues(
		"name", existingObj.GetName(), "namespace", existingObj.GetNamespace(), "kind", existingObj.GetKind(),
	)
	var err error
	var updateNeeded bool

	for key := range unstruct.Object {
		isStatus := key == "status"

		// use metadatacompliancetype to evaluate metadata if it is set
		keyComplianceType := complianceType
		if key == "metadata" && mdComplianceType != "" {
			keyComplianceType = mdComplianceType
		}

		// check key for mismatch
		errorMsg, keyUpdateNeeded, mergedObj, skipped := handleSingleKey(key, unstruct, existingObj, keyComplianceType)
		if errorMsg != "" {
			log.Info(errorMsg)

			return false, errorMsg, true
		}

		if mergedObj == nil && skipped {
			continue
		}

		mapMtx := sync.RWMutex{}
		mapMtx.Lock()

		// only look at labels and annotations for metadata - configurationPolicies do not update other metadata fields
		if key == "metadata" {
			mergedAnnotations := mergedObj.(map[string]interface{})["annotations"]
			mergedLabels := mergedObj.(map[string]interface{})["labels"]
			existingObj.UnstructuredContent()["metadata"].(map[string]interface{})["annotations"] = mergedAnnotations
			existingObj.UnstructuredContent()["metadata"].(map[string]interface{})["labels"] = mergedLabels
		} else {
			existingObj.UnstructuredContent()[key] = mergedObj
		}
		mapMtx.Unlock()

		if keyUpdateNeeded {
			updateNeeded = keyUpdateNeeded

			if strings.EqualFold(string(remediation), string(policyv1.Inform)) || isStatus {
				return true, "", false
			}

			log.Info("Queuing an update for the object due to a value mismatch", "key", key)
		}
	}

	if updateNeeded {
		log.V(2).Info("Updating the object based on the template definition")

		_, err = res.Update(context.TODO(), existingObj, metav1.UpdateOptions{})
		if k8serrors.IsNotFound(err) {
			message := fmt.Sprintf("`%v` is not present and must be created", existingObj.GetKind())

			return false, message, true
		}

		if err != nil {
			message := fmt.Sprintf("Error updating the object `%v`, the error is `%v`", name, err)

			return false, message, true
		}

		log.Info("Updated the object based on the template definition")
	}

	return false, "", false
}

// checkAndUpdateResource checks each individual key of a resource and passes it to handleKeys to see if it
// matches the template and update it if the remediationAction is enforce
func checkAndUpdateResource(
	obj singleObject,
	complianceType string,
	mdComplianceType string,
	remediation policyv1.RemediationAction,
	dclient dynamic.Interface,
) (throwSpecViolation bool, message string, processingErr bool) {
	log := log.WithValues(
		"policy", obj.policy.Name, "name", obj.name, "namespace", obj.namespace, "resource", obj.gvr.Resource,
	)

	var res dynamic.ResourceInterface
	if obj.namespaced {
		res = dclient.Resource(obj.gvr).Namespace(obj.namespace)
	} else {
		res = dclient.Resource(obj.gvr)
	}

	existingObj, err := res.Get(context.TODO(), obj.name, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "Could not retrieve object from the API server")
	} else {
		return handleKeys(obj.unstruct, existingObj, remediation, complianceType, mdComplianceType, obj.name, res)
	}

	return false, "", false
}

// AppendCondition check and appends conditions to the policy status
func AppendCondition(
	conditions []policyv1.Condition, newCond *policyv1.Condition, resourceType string, resolved ...bool,
) (conditionsRes []policyv1.Condition) {
	defer recoverFlow()

	lastIndex := len(conditions)
	if lastIndex > 0 {
		oldCond := conditions[lastIndex-1]
		if IsSimilarToLastCondition(oldCond, *newCond) {
			conditions[lastIndex-1] = *newCond

			return conditions
		}
	} else {
		// first condition => trigger event
		conditions = append(conditions, *newCond)

		return conditions
	}

	conditions[lastIndex-1] = *newCond

	return conditions
}

// IsSimilarToLastCondition checks the diff, so that we don't keep updating with the same info
func IsSimilarToLastCondition(oldCond policyv1.Condition, newCond policyv1.Condition) bool {
	return reflect.DeepEqual(oldCond.Status, newCond.Status) &&
		reflect.DeepEqual(oldCond.Reason, newCond.Reason) &&
		reflect.DeepEqual(oldCond.Message, newCond.Message) &&
		reflect.DeepEqual(oldCond.Type, newCond.Type)
}

// addForUpdate calculates the compliance status of a configurationPolicy and updates its status field if needed
func (r *ConfigurationPolicyReconciler) addForUpdate(policy *policyv1.ConfigurationPolicy) {
	compliant := true

	for index := range policy.Spec.ObjectTemplates {
		if index < len(policy.Status.CompliancyDetails) {
			if policy.Status.CompliancyDetails[index].ComplianceState == policyv1.NonCompliant {
				compliant = false
			}
		}
	}

	if len(policy.Status.CompliancyDetails) == 0 {
		policy.Status.ComplianceState = "Undetermined"
	} else if compliant {
		policy.Status.ComplianceState = policyv1.Compliant
	} else {
		policy.Status.ComplianceState = policyv1.NonCompliant
	}

	_, err := r.updatePolicyStatus(map[string]*policyv1.ConfigurationPolicy{
		(*policy).GetName(): policy,
	})
	policyLog := log.WithValues("name", policy.Name, "namespace", policy.Namespace)
	modifiedErr := "the object has been modified; please apply your changes to the latest version and try again"

	if err != nil && strings.Contains(err.Error(), modifiedErr) {
		policyLog.Error(err, "Tried to re-update status before previous update could be applied, retrying next loop")
	}

	if err != nil && !strings.Contains(err.Error(), modifiedErr) {
		policyLog.Error(err, "Could not update status")
	}
}

// updatePolicyStatus updates the status of the configurationPolicy if new conditions are added and generates an event
// on the parent policy with the compliance decision
func (r *ConfigurationPolicyReconciler) updatePolicyStatus(
	policies map[string]*policyv1.ConfigurationPolicy,
) (*policyv1.ConfigurationPolicy, error) {
	for plcName, instance := range policies { // policies is a map where: key = plc.Name, value = pointer to plc
		log.V(2).Info(
			"Updating configurationPolicy status", "status", instance.Status.ComplianceState, "policy", plcName,
		)

		err := r.Status().Update(context.TODO(), instance)
		if err != nil {
			return instance, err
		}

		if EventOnParent != "no" && instance.Status.ComplianceState != "Undetermined" {
			r.createParentPolicyEvent(instance)
		}

		r.Recorder.Event(instance, "Normal", "Policy updated",
			fmt.Sprintf("Policy status is: %v", instance.Status.ComplianceState))
	}

	return nil, nil
}

// handleRemovingPolicy removes a configurationPolicy from the list of configurationPolicies that the controller is
// processing
func handleRemovingPolicy(name string) {
	for k, v := range availablePolicies.PolicyMap {
		if v.Name == name {
			availablePolicies.RemoveObject(k)
		}
	}
}

// handleAddingPolicy adds a configurationPolicy to the list of configurationPolicies that the controller is processing
func handleAddingPolicy(plc *policyv1.ConfigurationPolicy) error {
	allNamespaces, err := common.GetAllNamespaces()
	if err != nil {
		return err
	}

	// clean up that policy from the existing namepsaces, in case the modification is in the namespace selector
	for _, ns := range allNamespaces {
		key := fmt.Sprintf("%s/%s", ns, plc.Name)
		if policy, found := availablePolicies.GetObject(key); found {
			if policy.Name == plc.Name {
				availablePolicies.RemoveObject(key)
			}
		}
	}

	// build namespace lists
	exclude := []string{}
	for _, ns := range plc.Spec.NamespaceSelector.Exclude {
		exclude = append(exclude, string(ns))
	}

	include := []string{}
	for _, ns := range plc.Spec.NamespaceSelector.Include {
		include = append(include, string(ns))
	}

	selectedNamespaces := common.GetSelectedNamespaces(include, exclude, allNamespaces)
	for _, ns := range selectedNamespaces {
		key := fmt.Sprintf("%s/%s", ns, plc.Name)
		availablePolicies.AddObject(key, plc)
	}

	if len(selectedNamespaces) == 0 {
		key := fmt.Sprintf("%s/%s", "NA", plc.Name)
		availablePolicies.AddObject(key, plc)
	}

	return err
}

// Builds the GroupVersion string from the inputs, by combining them with a "/" in the middle.
// If the group is empty, just returns the version string.
func buildGV(group, version string) string {
	if group == "" {
		return version
	}

	return group + "/" + version
}

// Helper functions that pretty prints a map to a string
func sprintMap(myMap map[string]*policyv1.ConfigurationPolicy) string {
	if len(myMap) == 0 {
		return "<Waiting for policies to be available for processing>"
	}

	var out strings.Builder

	out.WriteString("Available policies in namespaces:\n")

	mapToPrint := map[string][]string{}
	for k, v := range myMap {
		mapToPrint[v.Name] = append(mapToPrint[v.Name], strings.Split(k, "/")[0])
	}

	for k, v := range mapToPrint {
		nsString := "["

		for idx, ns := range v {
			nsString += ns
			if idx != len(v)-1 {
				nsString += ", "
			}
		}

		nsString += "]"

		fmt.Fprintf(&out, "\tconfigpolicy %s in namespace(s) %s", k, nsString)
	}

	return out.String()
}

func (r *ConfigurationPolicyReconciler) createParentPolicyEvent(instance *policyv1.ConfigurationPolicy) {
	if len(instance.OwnerReferences) == 0 {
		return // there is nothing to do, since no owner is set
	}

	// we are making an assumption that the GRC policy has a single owner, or we chose the first owner in the list
	if string(instance.OwnerReferences[0].UID) == "" {
		return // there is nothing to do, since no owner UID is set
	}

	parentPlc := createParentPolicy(instance)
	eventType := "Normal"

	if instance.Status.ComplianceState == policyv1.NonCompliant {
		eventType = "Warning"
	}

	eventMsg := convertPolicyStatusToString(instance)

	log.V(2).Info("Creating parent policy event", "eventMsg", eventMsg)

	r.Recorder.Event(&parentPlc,
		eventType,
		fmt.Sprintf(eventFmtStr, instance.Namespace, instance.Name),
		eventMsg)
}

func createParentPolicy(instance *policyv1.ConfigurationPolicy) extpoliciesv1.Policy {
	ns := common.ExtractNamespaceLabel(instance)
	if ns == "" {
		ns = NamespaceWatched
	}

	return extpoliciesv1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.OwnerReferences[0].Name,
			Namespace: ns, // we assume that the parent policy is in the watched-namespace passed as flag
			UID:       instance.OwnerReferences[0].UID,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Policy",
			APIVersion: "policy.open-cluster-management.io/v1",
		},
	}
}

// convertPolicyStatusToString to be able to pass the status as event
func convertPolicyStatusToString(plc *policyv1.ConfigurationPolicy) (results string) {
	if plc.Status.ComplianceState == "" {
		return "ComplianceState is still undetermined"
	}

	result := string(plc.Status.ComplianceState)

	if plc.Status.CompliancyDetails == nil || len(plc.Status.CompliancyDetails) == 0 {
		return result
	}

	for _, v := range plc.Status.CompliancyDetails {
		result += "; "
		for idx, cond := range v.Conditions {
			result += cond.Type + " - " + cond.Message
			if idx != len(v.Conditions)-1 {
				result += ", "
			}
		}
	}

	return result
}

func recoverFlow() {
	if r := recover(); r != nil {
		// V(-2) is the error level
		log.V(-2).Info("ALERT!!!! -> recovered from ", "recover", r)
	}
}
