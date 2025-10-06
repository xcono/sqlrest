package response

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// WriteError writes a standardized error response
func WriteError(w http.ResponseWriter, statusCode int, message, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := Response{
		Error:   message,
		Code:    fmt.Sprintf("PGRST%d", statusCode),
		Details: details,
	}

	json.NewEncoder(w).Encode(errorResponse)
}

// WriteBadRequest writes a 400 Bad Request error
func WriteBadRequest(w http.ResponseWriter, message, details string) {
	WriteError(w, http.StatusBadRequest, message, details)
}

// WriteNotFound writes a 404 Not Found error
func WriteNotFound(w http.ResponseWriter, message, details string) {
	WriteError(w, http.StatusNotFound, message, details)
}

// WriteMethodNotAllowed writes a 405 Method Not Allowed error
func WriteMethodNotAllowed(w http.ResponseWriter, method string) {
	WriteError(w, http.StatusMethodNotAllowed, "Method not allowed",
		fmt.Sprintf("Method %s not supported", method))
}

// WriteInternalServerError writes a 500 Internal Server Error
func WriteInternalServerError(w http.ResponseWriter, message, details string) {
	WriteError(w, http.StatusInternalServerError, message, details)
}

// WriteDatabaseError writes a database-related error
func WriteDatabaseError(w http.ResponseWriter, operation string, err error) {
	WriteInternalServerError(w, fmt.Sprintf("Database %s failed", operation), err.Error())
}

// WriteValidationError writes a validation error
func WriteValidationError(w http.ResponseWriter, field, reason string) {
	WriteBadRequest(w, "Validation failed", fmt.Sprintf("Field '%s': %s", field, reason))
}
