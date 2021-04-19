package templates

import (
)


// retrieve value for the given namespace , secretname and key
func fromSecret(namespace string, rscname string,  key string) (string , error) {
    return "secrettest", nil
}

// retrieve value for the given namespace , configmap and key
func fromConfigmap(namespace string, rscname string,  key string) (string , error) {
    return "configmaptest", nil
}

func note(name string) string{
  return "hello"
}
