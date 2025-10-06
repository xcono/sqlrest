package handlers

import (
	"net/http"
	"strings"

	"github.com/xcono/sqlrest/web/query"
	"github.com/xcono/sqlrest/web/response"
)

// SelectHandler handles GET requests for data retrieval
type SelectHandler struct {
	executor *query.Executor
}

// NewSelectHandler creates a new SELECT handler
func NewSelectHandler(executor *query.Executor) *SelectHandler {
	return &SelectHandler{executor: executor}
}

// Handle handles SELECT requests
func (h *SelectHandler) Handle(w http.ResponseWriter, r *http.Request, tableName string) {
	// Parse URL parameters
	queryParams := r.URL.Query()

	// Execute SELECT query
	results, err := h.executor.ExecuteSelect(tableName, queryParams)
	if err != nil {
		// Check if this is a validation error (400) vs database error (500)
		if strings.Contains(err.Error(), "failed to parse filter") {
			response.WriteParseError(w, err.Error(), "")
		} else {
			response.WriteDatabaseError(w, "query", err)
		}
		return
	}

	// Handle single row requests
	h.executor.HandleSingleRow(w, results, queryParams)
}
