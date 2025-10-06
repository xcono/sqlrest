package handlers

import (
	"net/http"

	"github.com/xcono/legs/web/query"
	"github.com/xcono/legs/web/response"
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
		response.WriteDatabaseError(w, "query", err)
		return
	}

	// Handle single row requests
	h.executor.HandleSingleRow(w, results, queryParams)
}
