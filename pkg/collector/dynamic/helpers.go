package dynamic

import (
	"sort"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// extractFieldString extracts a string field from an unstructured object using a JSONPath
func extractFieldString(obj *unstructured.Unstructured, path string) string {
	if path == "" {
		return ""
	}

	parts := strings.Split(path, ".")
	value, _, _ := unstructured.NestedString(obj.Object, parts...)

	return value
}

// extractFieldFloat extracts a float field from an unstructured object
func extractFieldFloat(obj *unstructured.Unstructured, path string) float64 {
	if path == "" {
		return 0
	}

	parts := strings.Split(path, ".")

	value, found, err := unstructured.NestedFieldNoCopy(obj.Object, parts...)
	if err != nil || !found {
		return 0
	}

	return toFloat64(value)
}

// extractFieldMap extracts a map field from an unstructured object
func extractFieldMap(obj *unstructured.Unstructured, path string) map[string]any {
	if path == "" {
		return nil
	}

	parts := strings.Split(path, ".")

	value, found, err := unstructured.NestedMap(obj.Object, parts...)
	if err != nil || !found {
		return nil
	}

	return value
}

// extractFieldSlice extracts a slice field from an unstructured object
func extractFieldSlice(obj *unstructured.Unstructured, path string) []any {
	if path == "" {
		return nil
	}

	parts := strings.Split(path, ".")

	value, found, err := unstructured.NestedSlice(obj.Object, parts...)
	if err != nil || !found {
		return nil
	}

	return value
}

// toFloat64 converts various types to float64
func toFloat64(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case uint:
		return float64(v)
	case uint32:
		return float64(v)
	case uint64:
		return float64(v)
	case bool:
		if v {
			return 1.0
		}
		return 0.0
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	default:
		return 0
	}
}

// sanitizeName sanitizes a name for use in Prometheus metrics
func sanitizeName(name string) string {
	// Replace invalid characters with underscores
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, name)

	// Convert to lowercase
	return strings.ToLower(result)
}

// getSortedKeys returns sorted keys from a map
func getSortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}

// getSortedValues returns values in sorted key order from a map
func getSortedValues(m map[string]string) []string {
	keys := getSortedKeys(m)

	values := make([]string, len(keys))
	for i, k := range keys {
		values[i] = m[k]
	}

	return values
}
