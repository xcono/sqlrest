package builder_test

import (
	"database/sql"
	"net/url"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/huandu/go-sqlbuilder"
	"github.com/xcono/sqlrest/builder"
)

// TestPostgRESTURLParsing tests URL parameter parsing

// TestPostgRESTURLParsing tests URL parameter parsing
func TestPostgRESTURLParsing(t *testing.T) {
	b := builder.NewPostgRESTBuilder()

	tt := []struct {
		name     string
		table    string
		params   url.Values
		expected *builder.PostgRESTQuery
	}{
		{
			name:  "simple equality filter",
			table: "users",
			params: url.Values{
				"name": []string{"Alice"},
			},
			expected: &builder.PostgRESTQuery{
				Table: "users",
				Filters: []interface{}{
					builder.Filter{Column: "name", Operator: builder.OpEQ, Value: "Alice"},
				},
			},
		},
		{
			name:  "greater than filter",
			table: "users",
			params: url.Values{
				"age": []string{"gt.18"},
			},
			expected: &builder.PostgRESTQuery{
				Table: "users",
				Filters: []interface{}{
					builder.Filter{Column: "age", Operator: builder.OpGT, Value: 18},
				},
			},
		},
		{
			name:  "in array filter",
			table: "users",
			params: url.Values{
				"status": []string{"in.(1,2,3)"},
			},
			expected: &builder.PostgRESTQuery{
				Table: "users",
				Filters: []interface{}{
					builder.Filter{Column: "status", Operator: builder.OpIn, Value: []interface{}{1, 2, 3}},
				},
			},
		},
		{
			name:  "like pattern filter",
			table: "users",
			params: url.Values{
				"name": []string{"like.*Alice*"},
			},
			expected: &builder.PostgRESTQuery{
				Table: "users",
				Filters: []interface{}{
					builder.Filter{Column: "name", Operator: builder.OpLike, Value: "%Alice%"},
				},
			},
		},
		{
			name:  "is null filter",
			table: "users",
			params: url.Values{
				"description": []string{"is.null"},
			},
			expected: &builder.PostgRESTQuery{
				Table: "users",
				Filters: []interface{}{
					builder.Filter{Column: "description", Operator: builder.OpIs, Value: nil},
				},
			},
		},
		{
			name:  "multiple filters",
			table: "users",
			params: url.Values{
				"name":   []string{"Alice"},
				"age":    []string{"gt.18"},
				"status": []string{"in.(1,2)"},
			},
			expected: &builder.PostgRESTQuery{
				Table: "users",
				Filters: []interface{}{
					builder.Filter{Column: "name", Operator: builder.OpEQ, Value: "Alice"},
					builder.Filter{Column: "age", Operator: builder.OpGT, Value: 18},
					builder.Filter{Column: "status", Operator: builder.OpIn, Value: []interface{}{1, 2}},
				},
			},
		},
		{
			name:  "with select and order",
			table: "users",
			params: url.Values{
				"select": []string{"id,name,email"},
				"order":  []string{"name,id"},
				"limit":  []string{"10"},
				"offset": []string{"5"},
			},
			expected: &builder.PostgRESTQuery{
				Table:  "users",
				Select: []string{"id", "name", "email"},
				Order:  []string{"name", "id"},
				Limit:  10,
				Offset: 5,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			result, err := b.ParseURLParams(tc.table, tc.params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Compare basic fields
			if result.Table != tc.expected.Table {
				t.Errorf("expected table %s, got %s", tc.expected.Table, result.Table)
			}

			if len(result.Filters) != len(tc.expected.Filters) {
				t.Errorf("expected %d filters, got %d", len(tc.expected.Filters), len(result.Filters))
			}

			// Compare select fields
			if !stringSlicesEqual(result.Select, tc.expected.Select) {
				t.Errorf("expected select %v, got %v", tc.expected.Select, result.Select)
			}

			// Compare order fields
			if !stringSlicesEqual(result.Order, tc.expected.Order) {
				t.Errorf("expected order %v, got %v", tc.expected.Order, result.Order)
			}

			// Compare limit and offset
			if result.Limit != tc.expected.Limit {
				t.Errorf("expected limit %d, got %d", tc.expected.Limit, result.Limit)
			}
			if result.Offset != tc.expected.Offset {
				t.Errorf("expected offset %d, got %d", tc.expected.Offset, result.Offset)
			}

			t.Logf("Parsed query: %+v", result)
		})
	}
}

// TestPostgRESTSQLGeneration tests SQL generation from PostgREST queries
func TestPostgRESTSQLGeneration(t *testing.T) {
	b := builder.NewPostgRESTBuilder()

	tt := []struct {
		name     string
		query    *builder.PostgRESTQuery
		expected string
	}{
		{
			name: "simple equality",
			query: &builder.PostgRESTQuery{
				Table: "users",
				Filters: []interface{}{
					builder.Filter{Column: "name", Operator: builder.OpEQ, Value: "Alice"},
				},
			},
			expected: "SELECT t1.* FROM users AS t1 WHERE name = ?",
		},
		{
			name: "greater than with select",
			query: &builder.PostgRESTQuery{
				Table:  "users",
				Select: []string{"id", "name"},
				Filters: []interface{}{
					builder.Filter{Column: "age", Operator: builder.OpGT, Value: 18},
				},
			},
			expected: "SELECT t1.id, t1.name FROM users AS t1 WHERE age > ?",
		},
		{
			name: "in array with order and limit",
			query: &builder.PostgRESTQuery{
				Table:  "users",
				Select: []string{"id", "name"},
				Filters: []interface{}{
					builder.Filter{Column: "status", Operator: builder.OpIn, Value: []interface{}{1, 2, 3}},
				},
				Order: []string{"name"},
				Limit: 10,
			},
			expected: "SELECT t1.id, t1.name FROM users AS t1 WHERE status IN (?, ?, ?) ORDER BY name LIMIT ?",
		},
		{
			name: "like pattern",
			query: &builder.PostgRESTQuery{
				Table: "users",
				Filters: []interface{}{
					builder.Filter{Column: "name", Operator: builder.OpLike, Value: "%Alice%"},
				},
			},
			expected: "SELECT t1.* FROM users AS t1 WHERE name LIKE ?",
		},
		{
			name: "is null",
			query: &builder.PostgRESTQuery{
				Table: "users",
				Filters: []interface{}{
					builder.Filter{Column: "description", Operator: builder.OpIs, Value: nil},
				},
			},
			expected: "SELECT t1.* FROM users AS t1 WHERE description IS NULL",
		},
		{
			name: "multiple filters",
			query: &builder.PostgRESTQuery{
				Table: "users",
				Filters: []interface{}{
					builder.Filter{Column: "name", Operator: builder.OpEQ, Value: "Alice"},
					builder.Filter{Column: "age", Operator: builder.OpGT, Value: 18},
				},
			},
			expected: "SELECT t1.* FROM users AS t1 WHERE age > ? AND name = ?",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			sb, err := b.BuildSQL(tc.query)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			sql, args := sb.BuildWithFlavor(sqlbuilder.MySQL)

			if sql != tc.expected {
				t.Errorf("expected SQL: %s, got: %s", tc.expected, sql)
			}

			t.Logf("SQL: %s", sql)
			t.Logf("Args: %v", args)
		})
	}
}

// Helper function to compare string slices
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestPostgRESTIntegration tests integration with real database
func TestPostgRESTIntegration(t *testing.T) {
	// Setup test database connection
	dsn := "root:nopass@tcp(127.0.0.1:3306)/test"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("Skipping database test: %v", err)
		return
	}
	// Ensure the database is reachable; skip if not
	if pingErr := db.Ping(); pingErr != nil {
		t.Skipf("Skipping database test (MySQL unreachable): %v", pingErr)
		return
	}
	defer db.Close()

	// Create test table
	_, err = db.Exec("DROP TABLE IF EXISTS test_postgrest")
	if err != nil {
		t.Fatalf("failed to drop test table: %v", err)
	}

	_, err = db.Exec("CREATE TABLE test_postgrest (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(255), age INT, status INT, description TEXT)")
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

	// Clean up after test
	defer func() {
		db.Exec("DROP TABLE IF EXISTS test_postgrest")
	}()

	// Insert test data
	testData := []struct {
		name        string
		age         int
		status      int
		description string
	}{
		{"Alice", 25, 1, "Active user"},
		{"Bob", 17, 2, "Inactive user"},
		{"Charlie", 30, 1, ""}, // Empty description
		{"Diana", 22, 3, "Premium user"},
	}

	for _, data := range testData {
		_, err = db.Exec("INSERT INTO test_postgrest (name, age, status, description) VALUES (?, ?, ?, ?)",
			data.name, data.age, data.status, data.description)
		if err != nil {
			t.Fatalf("failed to insert test data: %v", err)
		}
	}

	b := builder.NewPostgRESTBuilder()

	tt := []struct {
		name       string
		params     url.Values
		expectRows int
	}{
		{
			name: "filter by age greater than 18",
			params: url.Values{
				"age": []string{"gt.18"},
			},
			expectRows: 3, // Alice, Charlie, Diana
		},
		{
			name: "filter by status in array",
			params: url.Values{
				"status": []string{"in.(1,3)"},
			},
			expectRows: 3, // Alice, Charlie, Diana
		},
		{
			name: "filter by name like pattern",
			params: url.Values{
				"name": []string{"like.%ice%"},
			},
			expectRows: 1, // Alice
		},
		{
			name: "filter by empty description",
			params: url.Values{
				"description": []string{"eq."},
			},
			expectRows: 1, // Charlie (empty description)
		},
		{
			name: "multiple filters with select",
			params: url.Values{
				"select": []string{"name", "age"},
				"age":    []string{"gte.20"},
				"status": []string{"1"},
			},
			expectRows: 2, // Alice, Charlie
		},
		{
			name: "with order and limit",
			params: url.Values{
				"order": []string{"age"},
				"limit": []string{"2"},
			},
			expectRows: 2,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			query, err := b.ParseURLParams("test_postgrest", tc.params)
			if err != nil {
				t.Fatalf("failed to parse URL params: %v", err)
			}

			sb, err := b.BuildSQL(query)
			if err != nil {
				t.Fatalf("failed to build SQL: %v", err)
			}

			sql, args := sb.BuildWithFlavor(sqlbuilder.MySQL)

			rows, err := db.Query(sql, args...)
			if err != nil {
				t.Fatalf("failed to execute query: %v", err)
			}
			defer rows.Close()

			count := 0
			for rows.Next() {
				count++
			}

			if err := rows.Err(); err != nil {
				t.Fatalf("error iterating rows: %v", err)
			}

			if count != tc.expectRows {
				t.Errorf("expected %d rows, got %d", tc.expectRows, count)
			}

			t.Logf("Query: %s", sql)
			t.Logf("Args: %v", args)
			t.Logf("Rows returned: %d", count)
		})
	}
}

// TestPostgRESTExtendedOperators tests additional PostgREST operators
func TestPostgRESTExtendedOperators(t *testing.T) {
	b := builder.NewPostgRESTBuilder()

	tt := []struct {
		name     string
		table    string
		params   url.Values
		expected string
	}{
		{
			name:  "not equals operator (neq)",
			table: "users",
			params: url.Values{
				"status": []string{"neq.inactive"},
			},
			expected: "SELECT t1.* FROM users AS t1 WHERE status <> ?",
		},
		{
			name:  "less than or equal operator (lte)",
			table: "users",
			params: url.Values{
				"age": []string{"lte.65"},
			},
			expected: "SELECT t1.* FROM users AS t1 WHERE age <= ?",
		},
		{
			name:  "greater than or equal operator (gte)",
			table: "users",
			params: url.Values{
				"age": []string{"gte.18"},
			},
			expected: "SELECT t1.* FROM users AS t1 WHERE age >= ?",
		},
		{
			name:  "case-insensitive like operator (ilike)",
			table: "posts",
			params: url.Values{
				"title": []string{"ilike.%database%"},
			},
			expected: "SELECT t1.* FROM posts AS t1 WHERE LOWER(title) LIKE LOWER(?)",
		},
		{
			name:  "not null operator (is.not.null)",
			table: "users",
			params: url.Values{
				"email": []string{"is.not.null"},
			},
			expected: "SELECT t1.* FROM users AS t1 WHERE email IS NOT NULL",
		},
		{
			name:  "null operator (is.null)",
			table: "users",
			params: url.Values{
				"deleted_at": []string{"is.null"},
			},
			expected: "SELECT t1.* FROM users AS t1 WHERE deleted_at IS NULL",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			query, err := b.ParseURLParams(tc.table, tc.params)
			if err != nil {
				t.Fatalf("failed to parse URL params: %v", err)
			}

			sb, err := b.BuildSQL(query)
			if err != nil {
				t.Fatalf("failed to build SQL: %v", err)
			}

			sql, args := sb.BuildWithFlavor(sqlbuilder.MySQL)

			if sql != tc.expected {
				t.Errorf("expected SQL: %s, got: %s", tc.expected, sql)
			}

			t.Logf("SQL: %s", sql)
			t.Logf("Args: %v", args)
		})
	}
}

// TestPostgRESTLogicalOperators tests logical operators (and, or, not)
func TestPostgRESTLogicalOperators(t *testing.T) {
	b := builder.NewPostgRESTBuilder()

	tt := []struct {
		name     string
		table    string
		params   url.Values
		expected string
	}{
		{
			name:  "OR operator with multiple conditions",
			table: "users",
			params: url.Values{
				"or": []string{"(age.gt.25,name.eq.Alice)"},
			},
			expected: "SELECT t1.* FROM users AS t1 WHERE (age > ? OR name = ?)",
		},
		{
			name:  "AND operator with multiple conditions",
			table: "users",
			params: url.Values{
				"and": []string{"(status.eq.1,active.eq.true)"},
			},
			expected: "SELECT t1.* FROM users AS t1 WHERE (status = ? AND active = ?)",
		},
		{
			name:  "complex OR with mixed operators",
			table: "posts",
			params: url.Values{
				"or": []string{"(status.eq.published,created_at.gt.2024-01-01)"},
			},
			expected: "SELECT t1.* FROM posts AS t1 WHERE (status = ? OR created_at > ?)",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			query, err := b.ParseURLParams(tc.table, tc.params)
			if err != nil {
				t.Fatalf("failed to parse URL params: %v", err)
			}

			sb, err := b.BuildSQL(query)
			if err != nil {
				t.Fatalf("failed to build SQL: %v", err)
			}

			sql, args := sb.BuildWithFlavor(sqlbuilder.MySQL)

			if sql != tc.expected {
				t.Errorf("expected SQL: %s, got: %s", tc.expected, sql)
			}

			t.Logf("SQL: %s", sql)
			t.Logf("Args: %v", args)
		})
	}
}

// TestPostgRESTEmbedding tests resource embedding functionality
func TestPostgRESTEmbedding(t *testing.T) {
	b := builder.NewPostgRESTBuilder()

	tt := []struct {
		name     string
		table    string
		params   url.Values
		expected *builder.PostgRESTQuery
	}{
		{
			name:  "simple embedding",
			table: "posts",
			params: url.Values{
				"embed": []string{"author"},
			},
			expected: &builder.PostgRESTQuery{
				Table: "posts",
				Embeds: []builder.EmbedDefinition{
					{Table: "author", JoinType: builder.JoinTypeLeft, Columns: []string{"*"}},
				},
			},
		},
		{
			name:  "multiple embeddings",
			table: "posts",
			params: url.Values{
				"embed": []string{"author,comments"},
			},
			expected: &builder.PostgRESTQuery{
				Table: "posts",
				Embeds: []builder.EmbedDefinition{
					{Table: "author", JoinType: builder.JoinTypeLeft, Columns: []string{"*"}},
					{Table: "comments", JoinType: builder.JoinTypeLeft, Columns: []string{"*"}},
				},
			},
		},
		{
			name:  "nested embedding",
			table: "posts",
			params: url.Values{
				"embed": []string{"author(profile)"},
			},
			expected: &builder.PostgRESTQuery{
				Table: "posts",
				Embeds: []builder.EmbedDefinition{
					{Table: "author(profile)", JoinType: builder.JoinTypeLeft, Columns: []string{"*"}},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			query, err := b.ParseURLParams(tc.table, tc.params)
			if err != nil {
				t.Fatalf("failed to parse URL params: %v", err)
			}

			if len(query.Embeds) != len(tc.expected.Embeds) {
				t.Errorf("expected %d embeds, got %d", len(tc.expected.Embeds), len(query.Embeds))
			}

			for i, embed := range query.Embeds {
				if embed.Table != tc.expected.Embeds[i].Table {
					t.Errorf("expected embed table %s, got %s", tc.expected.Embeds[i].Table, embed.Table)
				}
			}

			t.Logf("Parsed embedding: %v", query.Embeds)
		})
	}
}

// TestPostgRESTEdgeCases tests edge cases and error handling
func TestPostgRESTEdgeCases(t *testing.T) {
	b := builder.NewPostgRESTBuilder()

	tt := []struct {
		name        string
		table       string
		params      url.Values
		expectError bool
		errorMsg    string
	}{
		{
			name:        "invalid operator",
			table:       "users",
			params:      url.Values{"age": []string{"invalid.18"}},
			expectError: true,
			errorMsg:    "invalid operator: invalid",
		},
		{
			name:        "empty table name",
			table:       "",
			params:      url.Values{"name": []string{"Alice"}},
			expectError: false, // Should not error during parsing, only during SQL building
		},
		{
			name:        "empty parameter value",
			table:       "users",
			params:      url.Values{"name": []string{""}},
			expectError: false,
		},
		{
			name:        "malformed logical filter",
			table:       "users",
			params:      url.Values{"or": []string{"age.gt.25"}}, // Missing parentheses
			expectError: true,
			errorMsg:    "logical filter must be wrapped in parentheses",
		},
		{
			name:        "invalid is operator value",
			table:       "users",
			params:      url.Values{"description": []string{"is.invalid"}},
			expectError: true,
			errorMsg:    "invalid is operator value: invalid",
		},
		{
			name:        "valid empty logical filter",
			table:       "users",
			params:      url.Values{"or": []string{"()"}},
			expectError: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			query, err := b.ParseURLParams(tc.table, tc.params)

			if tc.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if !strings.Contains(err.Error(), tc.errorMsg) {
					t.Errorf("expected error message containing '%s', got '%s'", tc.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if query != nil {
					t.Logf("Parsed query: %+v", query)
				}
			}
		})
	}
}

// TestPostgRESTComplexScenarios tests complex query scenarios
func TestPostgRESTComplexScenarios(t *testing.T) {
	b := builder.NewPostgRESTBuilder()

	tt := []struct {
		name     string
		table    string
		params   url.Values
		expected string
	}{
		{
			name:  "complex query with all features",
			table: "posts",
			params: url.Values{
				"select":     []string{"id,title,content"},
				"status":     []string{"eq.published"},
				"created_at": []string{"gte.2024-01-01"},
				"category":   []string{"in.(tech,startup)"},
				"order":      []string{"created_at.desc"},
				"limit":      []string{"10"},
				"offset":     []string{"5"},
			},
			expected: "SELECT t1.id, t1.title, t1.content FROM posts AS t1 WHERE category IN (?, ?) AND created_at >= ? AND status = ? ORDER BY created_at DESC LIMIT ? OFFSET ?",
		},
		{
			name:  "mixed data types and operators",
			table: "products",
			params: url.Values{
				"price":       []string{"gte.10.99"},
				"category":    []string{"in.(electronics,books)"},
				"in_stock":    []string{"eq.true"},
				"description": []string{"like.%smartphone%"},
				"status":      []string{"in.(new,featured)"},
			},
			expected: "SELECT t1.* FROM products AS t1 WHERE category IN (?, ?) AND description LIKE ? AND in_stock = ? AND price >= ? AND status IN (?, ?)",
		},
		{
			name:  "range operations with dates",
			table: "events",
			params: url.Values{
				"start_date": []string{"gte.2024-01-01"},
				"end_date":   []string{"lte.2024-01-31"},
				"venue":      []string{"neq.null"},
				"capacity":   []string{"gt.50"},
			},
			expected: "SELECT t1.* FROM events AS t1 WHERE capacity > ? AND end_date <= ? AND start_date >= ? AND venue <> ?",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			query, err := b.ParseURLParams(tc.table, tc.params)
			if err != nil {
				t.Fatalf("failed to parse URL params: %v", err)
			}

			sb, err := b.BuildSQL(query)
			if err != nil {
				t.Fatalf("failed to build SQL: %v", err)
			}

			sql, args := sb.BuildWithFlavor(sqlbuilder.MySQL)

			if sql != tc.expected {
				t.Errorf("expected SQL: %s, got: %s", tc.expected, sql)
			}

			t.Logf("SQL: %s", sql)
			t.Logf("Args: %v", args)
		})
	}
}

// TestPostgRESTInvariants tests that the same input always produces the same output
func TestPostgRESTInvariants(t *testing.T) {
	b := builder.NewPostgRESTBuilder()

	tt := []struct {
		name   string
		table  string
		params url.Values
	}{
		{
			name:  "basic query invariant",
			table: "users",
			params: url.Values{
				"name": []string{"Alice"},
				"age":  []string{"gt.18"},
			},
		},
		{
			name:  "complex query invariant",
			table: "posts",
			params: url.Values{
				"select":   []string{"id,title"},
				"status":   []string{"eq.published"},
				"category": []string{"in.(tech,startup)"},
				"order":    []string{"created_at"},
				"limit":    []string{"10"},
			},
		},
		{
			name:  "logical operators invariant",
			table: "users",
			params: url.Values{
				"or": []string{"(age.gt.25,status.eq.active)"},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Generate SQL multiple times and ensure consistency
			var results []string
			for i := 0; i < 5; i++ {
				query, err := b.ParseURLParams(tc.table, tc.params)
				if err != nil {
					t.Fatalf("failed to parse URL params: %v", err)
				}

				sb, err := b.BuildSQL(query)
				if err != nil {
					t.Fatalf("failed to build SQL: %v", err)
				}

				sql, _ := sb.BuildWithFlavor(sqlbuilder.MySQL)
				results = append(results, sql)
			}

			// All results should be identical
			for i := 1; i < len(results); i++ {
				if results[0] != results[i] {
					t.Errorf("inconsistent SQL generation: first=%s, iteration %d=%s", results[0], i+1, results[i])
				}
			}

			t.Logf("Consistent SQL: %s", results[0])
		})
	}
}

// TestPostgRESTJOINOperations tests JOIN operations with go-sqlbuilder
func TestPostgRESTJOINOperations(t *testing.T) {
	b := builder.NewPostgRESTBuilder()

	t.Run("simple_left_join", func(t *testing.T) {
		query := &builder.PostgRESTQuery{
			Table:  "users",
			Select: []string{"id", "name"},
			Embeds: []builder.EmbedDefinition{
				{
					Table:       "posts",
					JoinType:    builder.JoinTypeLeft,
					Columns:     []string{"id", "title"},
					OnCondition: "users.id = posts.user_id",
				},
			},
		}

		sb, err := b.BuildSQL(query)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sql, args := sb.BuildWithFlavor(sqlbuilder.MySQL)

		// Verify SQL contains JOIN
		if !strings.Contains(sql, "LEFT JOIN") {
			t.Errorf("Expected LEFT JOIN in SQL, got: %s", sql)
		}

		// Verify table aliases
		if !strings.Contains(sql, "users AS t1") {
			t.Errorf("Expected table alias in SQL, got: %s", sql)
		}

		// Verify column selection with aliases
		if !strings.Contains(sql, "t1.id") || !strings.Contains(sql, "t1.name") {
			t.Errorf("Expected aliased columns in SQL, got: %s", sql)
		}

		if !strings.Contains(sql, "t2.id") || !strings.Contains(sql, "t2.title") {
			t.Errorf("Expected aliased embed columns in SQL, got: %s", sql)
		}

		t.Logf("JOIN SQL: %s", sql)
		t.Logf("Args: %v", args)
	})

	t.Run("inner_join_with_nested_embed", func(t *testing.T) {
		query := &builder.PostgRESTQuery{
			Table:  "users",
			Select: []string{"id", "name"},
			Embeds: []builder.EmbedDefinition{
				{
					Table:       "posts",
					JoinType:    builder.JoinTypeInner,
					Columns:     []string{"id", "title"},
					OnCondition: "users.id = posts.user_id",
					NestedEmbeds: []builder.EmbedDefinition{
						{
							Table:       "comments",
							JoinType:    builder.JoinTypeLeft,
							Columns:     []string{"id", "text"},
							OnCondition: "posts.id = comments.post_id",
						},
					},
				},
			},
		}

		sb, err := b.BuildSQL(query)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sql, args := sb.BuildWithFlavor(sqlbuilder.MySQL)

		// Verify both JOIN types
		if !strings.Contains(sql, "INNER JOIN") {
			t.Errorf("Expected INNER JOIN in SQL, got: %s", sql)
		}

		if !strings.Contains(sql, "LEFT JOIN") {
			t.Errorf("Expected LEFT JOIN in SQL, got: %s", sql)
		}

		// Verify nested embed columns
		if !strings.Contains(sql, "t3.id") || !strings.Contains(sql, "t3.text") {
			t.Errorf("Expected nested embed columns in SQL, got: %s", sql)
		}

		t.Logf("Nested JOIN SQL: %s", sql)
		t.Logf("Args: %v", args)
	})

	t.Run("multiple_embeds", func(t *testing.T) {
		query := &builder.PostgRESTQuery{
			Table:  "users",
			Select: []string{"id", "name"},
			Embeds: []builder.EmbedDefinition{
				{
					Table:       "posts",
					JoinType:    builder.JoinTypeLeft,
					Columns:     []string{"id", "title"},
					OnCondition: "users.id = posts.user_id",
				},
				{
					Table:       "profiles",
					JoinType:    builder.JoinTypeLeft,
					Columns:     []string{"bio", "avatar"},
					OnCondition: "users.id = profiles.user_id",
				},
			},
		}

		sb, err := b.BuildSQL(query)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sql, args := sb.BuildWithFlavor(sqlbuilder.MySQL)

		// Count JOIN occurrences
		joinCount := strings.Count(sql, "LEFT JOIN")
		if joinCount != 2 {
			t.Errorf("Expected 2 LEFT JOINs, got %d", joinCount)
		}

		// Verify all table aliases are present
		if !strings.Contains(sql, "t1.") || !strings.Contains(sql, "t2.") || !strings.Contains(sql, "t3.") {
			t.Errorf("Expected all table aliases in SQL, got: %s", sql)
		}

		t.Logf("Multiple JOINs SQL: %s", sql)
		t.Logf("Args: %v", args)
	})

	t.Run("join_with_filters", func(t *testing.T) {
		query := &builder.PostgRESTQuery{
			Table:  "users",
			Select: []string{"id", "name"},
			Embeds: []builder.EmbedDefinition{
				{
					Table:       "posts",
					JoinType:    builder.JoinTypeLeft,
					Columns:     []string{"id", "title"},
					OnCondition: "users.id = posts.user_id",
				},
			},
			Filters: []interface{}{
				builder.Filter{Column: "users.status", Operator: builder.OpEQ, Value: 1},
				builder.Filter{Column: "posts.published", Operator: builder.OpEQ, Value: true},
			},
		}

		sb, err := b.BuildSQL(query)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sql, args := sb.BuildWithFlavor(sqlbuilder.MySQL)

		// Verify WHERE clause contains both conditions
		if !strings.Contains(sql, "WHERE") {
			t.Errorf("Expected WHERE clause in SQL, got: %s", sql)
		}

		if !strings.Contains(sql, "status = ?") || !strings.Contains(sql, "published = ?") {
			t.Errorf("Expected filter conditions in SQL, got: %s", sql)
		}

		t.Logf("JOIN with filters SQL: %s", sql)
		t.Logf("Args: %v", args)
	})

	t.Run("join_with_order_and_limit", func(t *testing.T) {
		query := &builder.PostgRESTQuery{
			Table:  "users",
			Select: []string{"id", "name"},
			Embeds: []builder.EmbedDefinition{
				{
					Table:       "posts",
					JoinType:    builder.JoinTypeLeft,
					Columns:     []string{"id", "title"},
					OnCondition: "users.id = posts.user_id",
				},
			},
			Order:  []string{"users.name", "posts.created_at DESC"},
			Limit:  10,
			Offset: 5,
		}

		sb, err := b.BuildSQL(query)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sql, args := sb.BuildWithFlavor(sqlbuilder.MySQL)

		// Verify ORDER BY clause
		if !strings.Contains(sql, "ORDER BY") {
			t.Errorf("Expected ORDER BY clause in SQL, got: %s", sql)
		}

		// Verify LIMIT and OFFSET
		if !strings.Contains(sql, "LIMIT ?") || !strings.Contains(sql, "OFFSET ?") {
			t.Errorf("Expected LIMIT and OFFSET in SQL, got: %s", sql)
		}

		t.Logf("JOIN with order/limit SQL: %s", sql)
		t.Logf("Args: %v", args)
	})
}

// TestPostgRESTEndToEndJOIN tests complete PostgREST JOIN workflow
func TestPostgRESTEndToEndJOIN(t *testing.T) {
	b := builder.NewPostgRESTBuilder()

	t.Run("postgrest_select_with_embeds", func(t *testing.T) {
		// Test PostgREST URL: /users?select=id,name,posts!inner(id,title,comments(text))
		params := url.Values{
			"select": []string{"id,name,posts!inner(id,title,comments(text))"},
		}

		query, err := b.ParseURLParams("users", params)
		if err != nil {
			t.Fatalf("failed to parse URL params: %v", err)
		}

		sb, err := b.BuildSQL(query)
		if err != nil {
			t.Fatalf("failed to build SQL: %v", err)
		}

		sql, args := sb.BuildWithFlavor(sqlbuilder.MySQL)

		// Verify the complete PostgREST JOIN query
		expectedParts := []string{
			"SELECT t1.id, t1.name, t2.id, t2.title, t3.text",
			"FROM users AS t1",
			"INNER JOIN posts AS t2",
			"LEFT JOIN comments AS t3",
		}

		for _, part := range expectedParts {
			if !strings.Contains(sql, part) {
				t.Errorf("Expected SQL to contain '%s', got: %s", part, sql)
			}
		}

		t.Logf("PostgREST JOIN SQL: %s", sql)
		t.Logf("Args: %v", args)
	})

	t.Run("postgrest_complex_join_query", func(t *testing.T) {
		// Test complex PostgREST query with filters and ordering
		params := url.Values{
			"select":          []string{"id,name,posts!left(id,title),profiles!left(bio)"},
			"status":          []string{"eq.1"},
			"posts.published": []string{"eq.true"},
			"order":           []string{"name,posts.created_at.desc"},
			"limit":           []string{"10"},
		}

		query, err := b.ParseURLParams("users", params)
		if err != nil {
			t.Fatalf("failed to parse URL params: %v", err)
		}

		sb, err := b.BuildSQL(query)
		if err != nil {
			t.Fatalf("failed to build SQL: %v", err)
		}

		sql, args := sb.BuildWithFlavor(sqlbuilder.MySQL)

		// Verify complex query structure
		if !strings.Contains(sql, "LEFT JOIN posts AS t2") {
			t.Errorf("Expected LEFT JOIN posts, got: %s", sql)
		}

		if !strings.Contains(sql, "LEFT JOIN profiles AS t3") {
			t.Errorf("Expected LEFT JOIN profiles, got: %s", sql)
		}

		if !strings.Contains(sql, "WHERE") {
			t.Errorf("Expected WHERE clause, got: %s", sql)
		}

		if !strings.Contains(sql, "ORDER BY") {
			t.Errorf("Expected ORDER BY clause, got: %s", sql)
		}

		if !strings.Contains(sql, "LIMIT ?") {
			t.Errorf("Expected LIMIT clause, got: %s", sql)
		}

		t.Logf("Complex PostgREST JOIN SQL: %s", sql)
		t.Logf("Args: %v", args)
	})

	t.Run("postgrest_legacy_embed_compatibility", func(t *testing.T) {
		// Test legacy embed parameter for backward compatibility
		params := url.Values{
			"embed":  []string{"posts,comments"},
			"select": []string{"id,name"},
		}

		query, err := b.ParseURLParams("users", params)
		if err != nil {
			t.Fatalf("failed to parse URL params: %v", err)
		}

		sb, err := b.BuildSQL(query)
		if err != nil {
			t.Fatalf("failed to build SQL: %v", err)
		}

		sql, args := sb.BuildWithFlavor(sqlbuilder.MySQL)

		// Verify legacy embed creates JOINs
		if !strings.Contains(sql, "LEFT JOIN posts AS t2") {
			t.Errorf("Expected LEFT JOIN posts from legacy embed, got: %s", sql)
		}

		if !strings.Contains(sql, "LEFT JOIN comments AS t3") {
			t.Errorf("Expected LEFT JOIN comments from legacy embed, got: %s", sql)
		}

		t.Logf("Legacy embed JOIN SQL: %s", sql)
		t.Logf("Args: %v", args)
	})
}
