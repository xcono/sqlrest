package compare

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Response represents a normalized API response
type Response struct {
	Data       interface{}
	StatusCode int
	Headers    map[string]string
}

// CompareResponses semantically compares two responses
func CompareResponses(postgrest, sqlrest Response) error {
	// Compare status codes
	if postgrest.StatusCode != sqlrest.StatusCode {
		return fmt.Errorf("status codes differ: postgrest=%d, sqlrest=%d",
			postgrest.StatusCode, sqlrest.StatusCode)
	}

	// Normalize and compare data
	pgData := normalizeData(postgrest.Data)
	srData := normalizeData(sqlrest.Data)

	if !deepEqual(pgData, srData) {
		return fmt.Errorf("data differs:\npostgrest: %+v\nsqlrest: %+v",
			pgData, srData)
	}

	return nil
}

// normalizeData converts data to a comparable format
func normalizeData(data interface{}) interface{} {
	// Convert to JSON and back to normalize types
	jsonBytes, _ := json.Marshal(data)
	var normalized interface{}
	json.Unmarshal(jsonBytes, &normalized)

	// Sort arrays for order-independent comparison
	if arr, ok := normalized.([]interface{}); ok {
		// Only sort if the array is not already in a consistent order
		if !isConsistentlyOrdered(arr) {
			sortArray(arr)
		}
	}

	return normalized
}

// sortArray sorts an array of maps by a deterministic key
func sortArray(arr []interface{}) {
	if len(arr) <= 1 {
		return
	}

	// Check if the array is already sorted by checking if it's in ascending order by ID
	if isAlreadySorted(arr) {
		return
	}

	sort.Slice(arr, func(i, j int) bool {
		mi, ok1 := arr[i].(map[string]interface{})
		mj, ok2 := arr[j].(map[string]interface{})
		if !ok1 || !ok2 {
			return false
		}

		// Try to find a common key to sort by
		// Prefer keys that look like IDs (end with '_id' or are 'id')
		for key := range mi {
			if _, exists := mj[key]; exists {
				// Prefer ID-like keys for deterministic sorting
				if key == "id" || strings.HasSuffix(key, "_id") {
					return fmt.Sprintf("%v", mi[key]) < fmt.Sprintf("%v", mj[key])
				}
			}
		}

		// Fallback: use the first common key alphabetically
		for key := range mi {
			if _, exists := mj[key]; exists {
				return fmt.Sprintf("%v", mi[key]) < fmt.Sprintf("%v", mj[key])
			}
		}

		return false
	})
}

// isAlreadySorted checks if the array is already sorted by ID
func isAlreadySorted(arr []interface{}) bool {
	if len(arr) <= 1 {
		return true
	}

	// Check if sorted by ID-like keys
	for i := 1; i < len(arr); i++ {
		prev, ok1 := arr[i-1].(map[string]interface{})
		curr, ok2 := arr[i].(map[string]interface{})
		if !ok1 || !ok2 {
			continue
		}

		// Check if sorted by ID-like keys
		for key := range prev {
			if _, exists := curr[key]; exists {
				if key == "id" || strings.HasSuffix(key, "_id") {
					prevVal := fmt.Sprintf("%v", prev[key])
					currVal := fmt.Sprintf("%v", curr[key])
					if prevVal > currVal {
						return false
					}
					break
				}
			}
		}
	}

	return true
}

// isConsistentlyOrdered checks if the array is consistently ordered
func isConsistentlyOrdered(arr []interface{}) bool {
	if len(arr) <= 1 {
		return true
	}

	// Check if the array is consistently ordered by checking if consecutive elements
	// follow a consistent pattern (either ascending or descending by ID)
	ascending := true
	descending := true

	for i := 1; i < len(arr); i++ {
		prev, ok1 := arr[i-1].(map[string]interface{})
		curr, ok2 := arr[i].(map[string]interface{})
		if !ok1 || !ok2 {
			continue
		}

		// Check if sorted by ID-like keys
		for key := range prev {
			if _, exists := curr[key]; exists {
				if key == "id" || strings.HasSuffix(key, "_id") {
					prevVal := fmt.Sprintf("%v", prev[key])
					currVal := fmt.Sprintf("%v", curr[key])

					if prevVal > currVal {
						ascending = false
					}
					if prevVal < currVal {
						descending = false
					}
					break
				}
			}
		}
	}

	// Return true if the array is consistently ordered (either ascending or descending)
	return ascending || descending
}

// deepEqual compares with type flexibility
func deepEqual(a, b interface{}) bool {
	return reflect.DeepEqual(a, b)
}
