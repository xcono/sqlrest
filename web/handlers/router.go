package handlers

import (
	"net/http"
	"strings"

	"github.com/xcono/sqlrest/web/database"
	"github.com/xcono/sqlrest/web/query"
	"github.com/xcono/sqlrest/web/response"
)

// Router handles request routing and delegates to appropriate handlers
type Router struct {
	selectHandler *SelectHandler
	insertHandler *InsertHandler
	updateHandler *UpdateHandler
	upsertHandler *UpsertHandler
	// deleteHandler *DeleteHandler  // TODO: Implement in next phase
}

// NewRouter creates a new request router
func NewRouter(db *database.Executor) *Router {
	queryExecutor := query.NewExecutor(db)

	return &Router{
		selectHandler: NewSelectHandler(queryExecutor),
		insertHandler: NewInsertHandler(db),
		updateHandler: NewUpdateHandler(db),
		upsertHandler: NewUpsertHandler(db),
		// deleteHandler: NewDeleteHandler(db),  // TODO: Implement in next phase
	}
}

// HandleTable handles requests to a specific table
func (r *Router) HandleTable(w http.ResponseWriter, req *http.Request) {
	// Extract table name from URL path
	pathParts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
	if len(pathParts) == 0 {
		response.WriteBadRequest(w, "Table name required", "No table specified in URL path")
		return
	}

	tableName := pathParts[0]

	// Route based on HTTP method
	switch req.Method {
	case http.MethodGet:
		r.selectHandler.Handle(w, req, tableName)
	case http.MethodPost:
		// Check if this is an UPSERT request (has Prefer: resolution=merge-duplicates header)
		preferHeader := req.Header.Get("Prefer")
		if preferHeader == "resolution=merge-duplicates" {
			r.upsertHandler.Handle(w, req, tableName)
		} else {
			r.insertHandler.Handle(w, req, tableName)
		}
	case http.MethodPatch:
		r.updateHandler.Handle(w, req, tableName)
	case http.MethodPut:
		// PUT method not used for UPSERT in PostgREST
		response.WriteMethodNotAllowed(w, req.Method)
	case http.MethodDelete:
		// TODO: Implement DELETE handler
		response.WriteMethodNotAllowed(w, req.Method)
	default:
		response.WriteMethodNotAllowed(w, req.Method)
	}
}
