package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/huandu/go-sqlbuilder"
	"github.com/xcono/sqlrest/builder"
	"github.com/xcono/sqlrest/web/database"
	"github.com/xcono/sqlrest/web/query"
	"github.com/xcono/sqlrest/web/response"
)

// UpdateHandler handles PATCH requests for data updates
type UpdateHandler struct {
	db      *database.Executor
	builder *builder.PostgRESTBuilder
}

// NewUpdateHandler creates a new UPDATE handler
func NewUpdateHandler(db *database.Executor) *UpdateHandler {
	return &UpdateHandler{
		db:      db,
		builder: builder.NewPostgRESTBuilder(),
	}
}

// Handle handles PATCH requests
func (h *UpdateHandler) Handle(w http.ResponseWriter, r *http.Request, tableName string) {
	// Parse request body
	var data interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		response.WriteBadRequest(w, "Invalid JSON in request body", err.Error())
		return
	}

	// Parse update data (must be a single object, not array)
	updates, err := h.parseUpdateData(data)
	if err != nil {
		response.WriteBadRequest(w, "Invalid data format", err.Error())
		return
	}

	if len(updates) == 0 {
		response.WriteBadRequest(w, "No data provided", "Request body must contain data to update")
		return
	}

	// Parse URL parameters for filters
	queryParams := r.URL.Query()
	query, err := h.builder.ParseURLParams(tableName, queryParams)
	if err != nil {
		response.WriteParseError(w, err.Error(), "")
		return
	}

	// Safety check: require at least one filter to prevent accidental full-table updates
	if len(query.Filters) == 0 {
		response.WriteBadRequest(w, "Filters required", "PATCH requests must include at least one filter to prevent accidental full-table updates")
		return
	}

	// Build and execute UPDATE query
	result, err := h.executeUpdate(tableName, updates, query.Filters)
	if err != nil {
		response.WriteDatabaseError(w, "update", err)
		return
	}

	// Get affected rows count
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		response.WriteInternalServerError(w, "Failed to get update result", err.Error())
		return
	}

	// Check if no rows were affected (PostgREST returns 404 in this case)
	if rowsAffected == 0 {
		response.WriteNotFound(w, "No rows matched the filter criteria", "No records were updated")
		return
	}

	// Check returning parameter
	returning := queryParams.Get("returning")
	if returning == "minimal" {
		// Return 204 No Content with count header
		w.Header().Set("X-Total-Count", fmt.Sprintf("%d", rowsAffected))
		response.WriteNoContent(w)
	} else if returning == "representation" {
		// Return the updated data (representation mode)
		updatedData, err := h.getUpdatedData(tableName, query.Filters, queryParams)
		if err != nil {
			response.WriteDatabaseError(w, "select updated data", err)
			return
		}

		// PostgREST returns data directly as array, not wrapped in object
		response.WritePostgRESTSuccess(w, updatedData, len(updatedData))
	} else {
		// Default behavior: return 204 No Content (PostgREST default)
		w.Header().Set("X-Total-Count", fmt.Sprintf("%d", rowsAffected))
		response.WriteNoContent(w)
	}
}

// parseUpdateData parses the request body into updateable data
func (h *UpdateHandler) parseUpdateData(data interface{}) (map[string]interface{}, error) {
	switch v := data.(type) {
	case map[string]interface{}:
		// Single object - this is what we want for PATCH
		return v, nil
	case []interface{}:
		// Array - reject this for PATCH (safety feature)
		return nil, &UpdateError{Message: "PATCH requests must contain a single object, not an array"}
	default:
		return nil, &UpdateError{Message: "Expected object for PATCH request"}
	}
}

// executeUpdate builds and executes the UPDATE query
func (h *UpdateHandler) executeUpdate(tableName string, updates map[string]interface{}, filters []interface{}) (sql.Result, error) {
	// Build UPDATE query
	ub := sqlbuilder.NewUpdateBuilder()
	ub.Update(tableName)

	// Set columns and values
	for col, val := range updates {
		ub.Set(ub.Assign(col, val))
	}

	// Apply filters to WHERE clause
	if err := h.builder.ApplyFiltersToUpdateBuilder(ub, filters); err != nil {
		return nil, err
	}

	// Execute query
	sql, args := ub.BuildWithFlavor(sqlbuilder.MySQL)
	return h.db.Exec(sql, args...)
}

// getUpdatedData retrieves the updated rows for representation mode
func (h *UpdateHandler) getUpdatedData(tableName string, filters []interface{}, queryParams map[string][]string) ([]map[string]interface{}, error) {
	// Create a query executor to reuse SELECT logic
	queryExecutor := query.NewExecutor(h.db)

	// Execute SELECT query with the same filters to get updated data
	return queryExecutor.ExecuteSelect(tableName, queryParams)
}

// UpdateError represents an update-specific error
type UpdateError struct {
	Message string
}

func (e *UpdateError) Error() string {
	return e.Message
}
