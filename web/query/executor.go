package query

import (
	"net/http"
	"net/url"

	"github.com/huandu/go-sqlbuilder"
	"github.com/xcono/sqlrest/builder"
	"github.com/xcono/sqlrest/web/database"
	"github.com/xcono/sqlrest/web/response"
)

// Executor handles query execution
type Executor struct {
	db      *database.Executor
	scanner *database.Scanner
	builder *builder.PostgRESTBuilder
}

// NewExecutor creates a new query executor
func NewExecutor(db *database.Executor) *Executor {
	return &Executor{
		db:      db,
		scanner: database.NewScanner(),
		builder: builder.NewPostgRESTBuilder(),
	}
}

// ExecuteSelect executes a SELECT query
func (e *Executor) ExecuteSelect(tableName string, params url.Values) ([]map[string]interface{}, error) {
	// Parse PostgREST query
	query, err := e.builder.ParseURLParams(tableName, params)
	if err != nil {
		return nil, err
	}

	// Build SQL query
	sb, err := e.builder.BuildSQL(query)
	if err != nil {
		return nil, err
	}

	// Execute query
	sql, args := sb.BuildWithFlavor(sqlbuilder.MySQL)
	rows, err := e.db.Query(sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Scan results
	return e.scanner.ScanRows(rows)
}

// HandleSingleRow handles single row requests
func (e *Executor) HandleSingleRow(w http.ResponseWriter, results []map[string]interface{}, params url.Values) {
	single := params.Get("single")
	maybeSingle := params.Get("maybeSingle")

	if single == "true" {
		if len(results) == 0 {
			response.WriteNotFound(w, "No rows found", "Single row requested but no results")
			return
		}
		if len(results) > 1 {
			response.WriteBadRequest(w, "Multiple rows found", "Single row requested but multiple results returned")
			return
		}
		response.WriteSingle(w, results[0])
		return
	}

	if maybeSingle == "true" {
		if len(results) == 0 {
			response.WriteNull(w)
			return
		}
		if len(results) > 1 {
			response.WriteBadRequest(w, "Multiple rows found", "MaybeSingle row requested but multiple results returned")
			return
		}
		response.WriteSingle(w, results[0])
		return
	}

	// Default: return array
	response.WritePostgRESTSuccess(w, results, len(results))
}
