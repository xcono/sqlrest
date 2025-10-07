package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/huandu/go-sqlbuilder"
	"github.com/xcono/sqlrest/builder"
	"github.com/xcono/sqlrest/web/database"
	"github.com/xcono/sqlrest/web/query"
	"github.com/xcono/sqlrest/web/response"
)

// UpsertHandler handles PUT requests for data upsertion
type UpsertHandler struct {
	db      *database.Executor
	builder *builder.PostgRESTBuilder
}

// NewUpsertHandler creates a new UPSERT handler
func NewUpsertHandler(db *database.Executor) *UpsertHandler {
	return &UpsertHandler{
		db:      db,
		builder: builder.NewPostgRESTBuilder(),
	}
}

// Handle handles PUT requests
func (h *UpsertHandler) Handle(w http.ResponseWriter, r *http.Request, tableName string) {
	// Parse request body
	var data interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		response.WriteBadRequest(w, "Invalid JSON in request body", err.Error())
		return
	}

	// Parse upsert data (accept both single object and arrays)
	values, err := h.parseUpsertData(data)
	if err != nil {
		response.WriteBadRequest(w, "Invalid data format", err.Error())
		return
	}

	if len(values) == 0 {
		response.WriteBadRequest(w, "No data provided", "Request body must contain data to upsert")
		return
	}

	// Check if any of the objects have fields
	hasFields := false
	for _, value := range values {
		if len(value) > 0 {
			hasFields = true
			break
		}
	}
	if !hasFields {
		response.WriteBadRequest(w, "No data provided", "Request body must contain data to upsert")
		return
	}

	// Build and execute UPSERT query
	result, err := h.executeUpsert(tableName, values)
	if err != nil {
		response.WriteDatabaseError(w, "upsert", err)
		return
	}

	// Get affected rows count
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		response.WriteInternalServerError(w, "Failed to get upsert result", err.Error())
		return
	}

	// Check returning parameter
	returning := r.URL.Query().Get("returning")
	if returning == "minimal" {
		// Return 201 Created with count header
		w.Header().Set("X-Total-Count", fmt.Sprintf("%d", rowsAffected))
		response.WriteCreated(w, nil)
	} else if returning == "representation" {
		// MySQL compatibility issue: returning=representation requires complex SELECT logic
		// that may not work reliably with MySQL's INSERT ... ON DUPLICATE KEY UPDATE
		response.WriteBadRequest(w, "Unsupported returning parameter", "returning=representation is not supported with MySQL UPSERT operations")
		return
	} else {
		// Default behavior: return 201 Created with input data
		response.WriteCreated(w, values)
	}
}

// parseUpsertData parses the request body into upsertable data
func (h *UpsertHandler) parseUpsertData(data interface{}) ([]map[string]interface{}, error) {
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
				return nil, &UpsertError{Message: "Array elements must be objects"}
			}
		}
	default:
		return nil, &UpsertError{Message: "Expected object or array of objects"}
	}

	return values, nil
}

// executeUpsert builds and executes the UPSERT query
func (h *UpsertHandler) executeUpsert(tableName string, values []map[string]interface{}) (sql.Result, error) {
	// Get column names from first row and sort them for consistent ordering
	var columns []string
	for col := range values[0] {
		columns = append(columns, col)
	}

	// Check if we have any columns
	if len(columns) == 0 {
		return nil, fmt.Errorf("no columns to upsert")
	}

	// Sort columns for consistent SQL generation
	sort.Strings(columns)

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

	// Build SQL and manually append ON DUPLICATE KEY UPDATE
	sql, args := ib.BuildWithFlavor(sqlbuilder.MySQL)

	// Create update clauses for ON DUPLICATE KEY UPDATE
	updateClauses := []string{}
	for _, col := range columns {
		updateClauses = append(updateClauses, fmt.Sprintf("%s = VALUES(%s)", col, col))
	}

	// Append ON DUPLICATE KEY UPDATE clause
	sql += " ON DUPLICATE KEY UPDATE " + strings.Join(updateClauses, ", ")

	// Execute query
	return h.db.Exec(sql, args...)
}

// getUpsertedData retrieves the upserted rows for representation mode
func (h *UpsertHandler) getUpsertedData(tableName string, values []map[string]interface{}) ([]map[string]interface{}, error) {
	// Extract primary key values from input data
	// For simplicity, we'll assume the first column is the primary key
	// In a real implementation, you'd query the database schema to find the actual primary key
	var primaryKeyValues []interface{}
	primaryKeyColumn := ""

	if len(values) > 0 {
		// Get the first column name as primary key (simplified approach)
		// Sort columns to get consistent ordering
		var columns []string
		for col := range values[0] {
			columns = append(columns, col)
		}
		sort.Strings(columns)
		primaryKeyColumn = columns[0]

		// Extract primary key values from all rows
		for _, row := range values {
			if val, exists := row[primaryKeyColumn]; exists {
				primaryKeyValues = append(primaryKeyValues, val)
			}
		}
	}

	if len(primaryKeyValues) == 0 {
		return []map[string]interface{}{}, nil
	}

	// Build SELECT query to get upserted rows
	queryExecutor := query.NewExecutor(h.db)

	// Create query parameters for IN clause
	queryParams := map[string][]string{
		fmt.Sprintf("%s", primaryKeyColumn): {fmt.Sprintf("in.(%s)", strings.Join(convertToStrings(primaryKeyValues), ","))},
	}

	// Execute SELECT query to get upserted data
	return queryExecutor.ExecuteSelect(tableName, queryParams)
}

// convertToStrings converts interface{} slice to string slice
func convertToStrings(values []interface{}) []string {
	result := make([]string, len(values))
	for i, val := range values {
		result[i] = fmt.Sprintf("%v", val)
	}
	return result
}

// UpsertError represents an upsert-specific error
type UpsertError struct {
	Message string
}

func (e *UpsertError) Error() string {
	return e.Message
}
