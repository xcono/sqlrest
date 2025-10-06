package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/huandu/go-sqlbuilder"
	"github.com/xcono/legs/web/database"
	"github.com/xcono/legs/web/response"
)

// InsertHandler handles POST requests for data insertion
type InsertHandler struct {
	db *database.Executor
}

// NewInsertHandler creates a new INSERT handler
func NewInsertHandler(db *database.Executor) *InsertHandler {
	return &InsertHandler{db: db}
}

// Handle handles INSERT requests
func (h *InsertHandler) Handle(w http.ResponseWriter, r *http.Request, tableName string) {
	// Parse request body
	var data interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		response.WriteBadRequest(w, "Invalid JSON in request body", err.Error())
		return
	}

	// Handle different data formats
	values, err := h.parseInsertData(data)
	if err != nil {
		response.WriteBadRequest(w, "Invalid data format", err.Error())
		return
	}

	if len(values) == 0 {
		response.WriteBadRequest(w, "No data provided", "Request body must contain data to insert")
		return
	}

	// Build and execute INSERT query
	result, err := h.executeInsert(tableName, values)
	if err != nil {
		response.WriteDatabaseError(w, "insert", err)
		return
	}

	// Get affected rows count
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		response.WriteInternalServerError(w, "Failed to get insert result", err.Error())
		return
	}

	// Check if we should return the inserted data
	returning := r.URL.Query().Get("returning")
	if returning != "minimal" {
		// Return the inserted data
		response.WriteCreated(w, values)
	} else {
		// Return just the count
		response.WriteSuccess(w, map[string]interface{}{
			"rows_affected": rowsAffected,
		}, int(rowsAffected))
	}
}

// parseInsertData parses the request body into insertable data
func (h *InsertHandler) parseInsertData(data interface{}) ([]map[string]interface{}, error) {
	var values []map[string]interface{}

	switch v := data.(type) {
	case map[string]interface{}:
		// Single object
		values = []map[string]interface{}{v}
	case []interface{}:
		// Array of objects
		for _, item := range v {
			if obj, ok := item.(map[string]interface{}); ok {
				values = append(values, obj)
			} else {
				return nil, &InsertError{Message: "Array elements must be objects"}
			}
		}
	default:
		return nil, &InsertError{Message: "Expected object or array of objects"}
	}

	return values, nil
}

// executeInsert builds and executes the INSERT query
func (h *InsertHandler) executeInsert(tableName string, values []map[string]interface{}) (sql.Result, error) {
	// Get column names from first row
	var columns []string
	for col := range values[0] {
		columns = append(columns, col)
	}

	// Build INSERT query
	ib := sqlbuilder.NewInsertBuilder()
	ib.InsertInto(tableName)
	ib.Cols(columns...)

	// Add values
	for _, row := range values {
		var rowValues []interface{}
		for _, col := range columns {
			rowValues = append(rowValues, row[col])
		}
		ib.Values(rowValues...)
	}

	// Execute query
	sql, args := ib.BuildWithFlavor(sqlbuilder.MySQL)
	return h.db.Exec(sql, args...)
}

// InsertError represents an insert-specific error
type InsertError struct {
	Message string
}

func (e *InsertError) Error() string {
	return e.Message
}
