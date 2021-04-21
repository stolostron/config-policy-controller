
package templates

import (
	"text/template"
	"strings"
	"github.com/golang/glog"
  "sigs.k8s.io/yaml"
	"k8s.io/client-go/kubernetes"
)

var kubeClient *kubernetes.Interface
func InitializeKubeClient(kClient *kubernetes.Interface) {
	kubeClient = kClient
}

func initFuncMap() template.FuncMap{
	fmap := template.FuncMap{
			"fromSecret": fromSecret,
			"fromConfigmap": fromConfigmap,
			"fromClusterClaim": fromClusterClaim,
			"lookup" : lookup,
			"base64enc" : base64encode,
			"base64dec" : base64decode,
			"note" : note,
	}
	return fmap
}

// fromYAML converts a YAML document into a map[string]interface{}.
func fromYAML(str string) map[string]interface{} {
	m := map[string]interface{}{}

	if err := yaml.Unmarshal([]byte(str), &m); err != nil {
		glog.Errorf("error parsing fromYAML %v", err)
		m["Error"] = err.Error()
	}
	return m
}

func toYAML(v interface{}) string {
	data, err := yaml.Marshal(v)

	if err != nil {
		// Swallow errors inside of a template.
		glog.Errorf("error parsing toYAML %v", err)
		return ""
	}

	return strings.TrimSuffix(string(data), "\n")
}

func HasTemplate(templateStr string) bool{
	glog.Errorf("hasTemplate template str  %v", templateStr)
	if strings.Contains(templateStr, "{{") {
		return true
	}
	return false
}

func ResolveTemplate(tmplMap interface{}) (interface{}, error) {
	var buf strings.Builder

	templateStr := toYAML(tmplMap)
	glog.Errorf("template String  %v", templateStr)


	tmpl := template.New("tmpl").Funcs(initFuncMap())

	tmpl, err := tmpl.Parse(templateStr)
	if err != nil {
		glog.Errorf("error parsing the template %v", err)
		return "", err
	}

	err = tmpl.Execute(&buf, "")
	if err != nil {
		glog.Errorf("error executing the template %v", err)
	  return "", err
	}

	glog.Errorf("resolved template: %v ", buf.String())

	return fromYAML(buf.String()), nil
}
