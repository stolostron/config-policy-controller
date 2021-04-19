module github.com/open-cluster-management/config-policy-controller

go 1.14

require (
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/google/go-cmp v0.5.2
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/operator-framework/operator-sdk v0.19.4
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/api v0.20.5
	k8s.io/apiextensions-apiserver v0.20.0 // indirect
	k8s.io/apimachinery v0.20.5
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.8.0 // indirect
	sigs.k8s.io/controller-runtime v0.6.2
)

replace (
	github.com/go-logr/zapr => github.com/go-logr/zapr v0.4.0
	github.com/open-cluster-management/config-policy-controller/test => ./test
	k8s.io/client-go => k8s.io/client-go v0.20.5
)
