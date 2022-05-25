package configurationpolicy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatTemplateAnnotation(t *testing.T) {
	t.Parallel()
	policyTemplate := map[string]interface{}{
		"annotations": map[string]interface{}{
			"annotation1": "one!",
			"annotation2": "two!",
		},
		"labels": map[string]string{
			"label1": "yes",
			"label2": "no",
		},
	}

	policyTemplateFormatted := formatMetadata(policyTemplate)
	assert.Equal(t, policyTemplateFormatted["annotations"], policyTemplate["annotations"])
}

func TestFormatTemplateNullAnnotation(t *testing.T) {
	t.Parallel()
	policyTemplate := map[string]interface{}{
		"annotations": nil,
		"labels": map[string]string{
			"label1": "yes",
			"label2": "no",
		},
	}

	policyTemplateFormatted := formatMetadata(policyTemplate)
	assert.Nil(t, policyTemplateFormatted["annotations"])
}

func TestFormatTemplateStringAnnotation(t *testing.T) {
	t.Parallel()
	policyTemplate := map[string]interface{}{
		"annotations": "not-an-annotation",
		"labels": map[string]string{
			"label1": "yes",
			"label2": "no",
		},
	}

	policyTemplateFormatted := formatMetadata(policyTemplate)
	assert.Equal(t, policyTemplateFormatted["annotations"], "not-an-annotation")
}

func TestCheckFieldsWithSort(t *testing.T) {
	t.Parallel()

	oldObj := map[string]interface{}{
		"nonResourceURLs": []string{"/version", "/healthz"},
		"verbs":           []string{"get"},
	}
	mergedObj := map[string]interface{}{
		"nonResourceURLs": []string{"/version", "/healthz"},
		"verbs":           []string{"get"},
		"apiGroups":       []interface{}{},
		"resources":       []interface{}{},
	}

	assert.True(t, checkFieldsWithSort(mergedObj, oldObj))
}
