package response

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Response represents a standardized API response
type Response struct {
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
	Code    string      `json:"code,omitempty"`
	Details string      `json:"details,omitempty"`
	Hint    interface{} `json:"hint,omitempty"`
	Count   int         `json:"count,omitempty"`
}

// WriteSuccess writes a successful response
func WriteSuccess(w http.ResponseWriter, data interface{}, count int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Total-Count", fmt.Sprintf("%d", count))
	w.WriteHeader(http.StatusOK)

	response := Response{
		Data:  data,
		Count: count,
	}

	json.NewEncoder(w).Encode(response)
}

// WriteCreated writes a 201 Created response
func WriteCreated(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	response := Response{
		Data: data,
	}

	json.NewEncoder(w).Encode(response)
}

// WriteNoContent writes a 204 No Content response
func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// WriteSingle writes a single object response (for single/maybeSingle)
func WriteSingle(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(data)
}

// WriteNull writes a null response (for maybeSingle with no results)
func WriteNull(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(nil)
}

// WritePostgRESTSuccess writes a PostgREST-compatible success response (direct array)
func WritePostgRESTSuccess(w http.ResponseWriter, data interface{}, count int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Total-Count", fmt.Sprintf("%d", count))
	w.WriteHeader(http.StatusOK)

	// PostgREST returns data directly as array, not wrapped in object
	json.NewEncoder(w).Encode(data)
}
