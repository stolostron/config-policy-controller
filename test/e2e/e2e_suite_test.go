// Copyright (c) 2020 Red Hat, Inc.

package e2e

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

var (
	testNamespace         string
	clientHub             kubernetes.Interface
	clientHubDynamic      dynamic.Interface
	gvrPolicy             schema.GroupVersionResource
	gvrPlacementBinding   schema.GroupVersionResource
	gvrPlacementRule      schema.GroupVersionResource
	optionsFile           string
	baseDomain            string
	kubeadminUser         string
	kubeadminCredential   string
	kubeconfig            string
	defaultTimeoutSeconds int
	kubeconfigManaged     string
	clientManaged         kubernetes.Interface
	clientManagedDynamic  dynamic.Interface
	gvrConfigPolicy       schema.GroupVersionResource
	gvrPod                schema.GroupVersionResource
	gvrRole               schema.GroupVersionResource
	gvrNS                 schema.GroupVersionResource
	gvrSCC                schema.GroupVersionResource

	defaultImageRegistry       string
	defaultImagePullSecretName string
)

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config policy controller e2e Suite")
}

func init() {
	klog.SetOutput(GinkgoWriter)
	klog.InitFlags(nil)
	//flag.StringVar(&kubeconfigHub, "kubeconfig_hub", "../../kubeconfig_hub", "Location of the kubeconfig to use; defaults to KUBECONFIG if not set")
	flag.StringVar(&kubeconfigManaged, "kubeconfig_managed", "../../kubeconfig_managed", "Location of the kubeconfig to use; defaults to KUBECONFIG if not set")
}

var _ = BeforeSuite(func() {
	By("Setup Hub client")
	gvrPod = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	gvrNS = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	gvrRole = schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"}
	gvrConfigPolicy = schema.GroupVersionResource{Group: "policy.open-cluster-management.io", Version: "v1", Resource: "configurationpolicies"}
	gvrSCC = schema.GroupVersionResource{Group: "security.openshift.io", Version: "v1", Resource: "securitycontextconstraints"}
	clientManaged = NewKubeClient("", kubeconfigManaged, "")
	clientManagedDynamic = NewKubeClientDynamic("", kubeconfigManaged, "")
	defaultImageRegistry = "quay.io/stolostron"
	defaultImagePullSecretName = "multiclusterhub-operator-pull-secret"
	testNamespace = "managed"
	defaultTimeoutSeconds = 60
	By("Create Namespace if needed")
	namespaces := clientManaged.CoreV1().Namespaces()
	if _, err := namespaces.Get(testNamespace, metav1.GetOptions{}); err != nil && errors.IsNotFound(err) {
		Expect(namespaces.Create(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		})).NotTo(BeNil())
	}
	Expect(namespaces.Get(testNamespace, metav1.GetOptions{})).NotTo(BeNil())
})

func NewKubeClient(url, kubeconfig, context string) kubernetes.Interface {
	klog.V(5).Infof("Create kubeclient for url %s using kubeconfig path %s\n", url, kubeconfig)
	config, err := LoadConfig(url, kubeconfig, context)
	if err != nil {
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	return clientset
}

func NewKubeClientDynamic(url, kubeconfig, context string) dynamic.Interface {
	klog.V(5).Infof("Create kubeclient dynamic for url %s using kubeconfig path %s\n", url, kubeconfig)
	config, err := LoadConfig(url, kubeconfig, context)
	if err != nil {
		panic(err)
	}

	clientset, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	return clientset
}

func LoadConfig(url, kubeconfig, context string) (*rest.Config, error) {
	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}
	klog.V(5).Infof("Kubeconfig path %s\n", kubeconfig)
	// If we have an explicit indication of where the kubernetes config lives, read that.
	if kubeconfig != "" {
		if context == "" {
			return clientcmd.BuildConfigFromFlags(url, kubeconfig)
		}
		return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
			&clientcmd.ConfigOverrides{
				CurrentContext: context,
			}).ClientConfig()
	}
	// If not, try the in-cluster config.
	if c, err := rest.InClusterConfig(); err == nil {
		return c, nil
	}
	// If no in-cluster config, try the default location in the user's home directory.
	if usr, err := user.Current(); err == nil {
		klog.V(5).Infof("clientcmd.BuildConfigFromFlags for url %s using %s\n", url, filepath.Join(usr.HomeDir, ".kube", "config"))
		if c, err := clientcmd.BuildConfigFromFlags("", filepath.Join(usr.HomeDir, ".kube", "config")); err == nil {
			return c, nil
		}
	}

	return nil, fmt.Errorf("could not create a valid kubeconfig")

}
