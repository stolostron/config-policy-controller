package templates

import (
  metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  "github.com/golang/glog"
  base64 "encoding/base64"
)


// retrieve value for the given namespace , secretname and key
func fromSecret(namespace string, rscname string,  key string) (string , error) {

  secretsClient := (*kubeClient).CoreV1().Secrets(namespace)
  secret, getErr := secretsClient.Get(rscname, metav1.GetOptions{})

  if getErr != nil {
      glog.Errorf("Error Getting secret:  %v", getErr)
    return "" , getErr
  }

  keyVal := secret.Data[key]
  // when using corev1 secret , the data is returned decoded ,
  // to be able to use in the referencing secret
  sEnc := base64.StdEncoding.EncodeToString(keyVal)
  return sEnc, nil
}

// retrieve value for the given namespace , configmap and key
func fromConfigmap(namespace string, rscname string,  key string) (string , error) {
  configmapsClient := (*kubeClient).CoreV1().ConfigMaps(namespace)
  configmap, getErr := configmapsClient.Get(rscname, metav1.GetOptions{})

  if getErr != nil {
      glog.Errorf("Error getting configmap:  %v", getErr)
    return "" , getErr
  }
  //glog.Errorf("Configmap is %v", configmap)

  keyVal := configmap.Data[key]
  return keyVal, nil
}

func note(name string) string{
  return "hello"
}

func base64encode(v string) string {
	return base64.StdEncoding.EncodeToString([]byte(v))
}

func base64decode(v string) string {
	data, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return err.Error()
	}
	return string(data)
}
