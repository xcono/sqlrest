package web

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/xcono/legs/web/database"
	"github.com/xcono/legs/web/handlers"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create test table with comprehensive schema
	createTableSQL := `
	CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		email TEXT UNIQUE,
		age INTEGER,
		status INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		salary REAL,
		is_active BOOLEAN DEFAULT 1,
		department TEXT
	)`

	if _, err := db.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	// Insert test data
	insertSQL := `
	INSERT INTO users (name, email, age, status, salary, is_active, department) VALUES 
	('Alice', 'alice@example.com', 25, 1, 50000.0, 1, 'Engineering'),
	('Bob Smith', 'bob@example.com', 30, 1, 60000.0, 1, 'Marketing'),
	('Charlie Brown', 'charlie@example.com', 35, 0, 70000.0, 0, 'Engineering'),
	('Diana Prince', 'diana@example.com', 28, 1, 55000.0, 1, 'Sales'),
	('Eve Wilson', 'eve@example.com', 22, 1, 45000.0, 1, 'Marketing'),
	('Frank Miller', 'frank@example.com', 40, 0, 80000.0, 0, 'Engineering'),
	('Grace Lee', 'grace@example.com', 26, 1, 52000.0, 1, 'Sales'),
	('Henry Davis', 'henry@example.com', 33, 1, 65000.0, 1, 'Marketing'),
	('Ivy Chen', 'ivy@example.com', 29, 1, 58000.0, 1, 'Engineering'),
	('Jack Wilson', 'jack@example.com', 31, 0, 62000.0, 0, 'Sales'),
	('Mary Jane Watson', 'mary.jane@example.com', 27, 1, 55000.0, 1, 'Engineering')
	`

	if _, err := db.Exec(insertSQL); err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	return db
}

// createTestServer creates a test server with the refactored architecture
func createTestServer(t *testing.T, db *sql.DB) *httptest.Server {
	dbExecutor := database.NewExecutor(db)
	router := handlers.NewRouter(dbExecutor)

	mux := http.NewServeMux()

	// Add CORS middleware
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept")
		w.Header().Set("Content-Type", "application/json")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Route requests
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"message": "PostgREST API Server"}`))
			return
		}

		// Handle dynamic table routes
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pathParts) > 0 && pathParts[0] != "" {
			router.HandleTable(w, r)
			return
		}

		http.NotFound(w, r)
	})

	return httptest.NewServer(mux)
}

// TestPostgRESTSelectOperations tests all SELECT operations following PostgREST documentation
func TestPostgRESTSelectOperations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	server := createTestServer(t, db)
	defer server.Close()

	// Helper function to clean up test data before each test
	cleanupTestData := func() {
		db.Exec("DELETE FROM users")
		// Re-insert test data
		insertSQL := `
		INSERT INTO users (name, email, age, status, salary, is_active, department) VALUES 
		('Alice', 'alice@example.com', 25, 1, 50000.0, 1, 'Engineering'),
		('Bob Smith', 'bob@example.com', 30, 1, 60000.0, 1, 'Marketing'),
		('Charlie Brown', 'charlie@example.com', 35, 0, 70000.0, 0, 'Engineering'),
		('Diana Prince', 'diana@example.com', 28, 1, 55000.0, 1, 'Sales'),
		('Eve Wilson', 'eve@example.com', 22, 1, 45000.0, 1, 'Marketing'),
		('Frank Miller', 'frank@example.com', 40, 0, 80000.0, 0, 'Engineering'),
		('Grace Lee', 'grace@example.com', 26, 1, 52000.0, 1, 'Sales'),
		('Henry Davis', 'henry@example.com', 33, 1, 65000.0, 1, 'Marketing'),
		('Ivy Chen', 'ivy@example.com', 29, 1, 58000.0, 1, 'Engineering'),
		('Jack Wilson', 'jack@example.com', 31, 0, 62000.0, 0, 'Sales'),
		('Mary Jane Watson', 'mary.jane@example.com', 27, 1, 55000.0, 1, 'Engineering')
		`
		db.Exec(insertSQL)
	}

	// Test 1: Basic SELECT with column selection
	t.Run("select_columns", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/users?select=id,name,email")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var result []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(result) == 0 {
			t.Error("Expected at least one result")
		}

		// Check that only selected columns are present
		firstRow := result[0]
		if _, exists := firstRow["id"]; !exists {
			t.Error("Expected 'id' column")
		}
		if _, exists := firstRow["name"]; !exists {
			t.Error("Expected 'name' column")
		}
		if _, exists := firstRow["age"]; exists {
			t.Error("Expected 'age' column to be excluded")
		}

		t.Logf("SELECT columns test: %d rows returned", len(result))
	})

	// Test 2: Equality filter
	t.Run("filter_eq", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/users?age=eq.25")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var result []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(result) != 1 {
			t.Errorf("Expected 1 result, got %d", len(result))
		}

		if result[0]["name"] != "Alice" {
			t.Errorf("Expected Alice, got %v", result[0]["name"])
		}

		t.Logf("Filter eq test: %d rows returned", len(result))
	})

	// Test 3: Greater than filter
	t.Run("filter_gt", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/users?age=gt.30")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var result []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(result) < 2 {
			t.Errorf("Expected at least 2 results, got %d", len(result))
		}

		// Verify all results have age > 30
		for _, row := range result {
			if age, ok := row["age"].(float64); ok {
				if age <= 30 {
					t.Errorf("Expected age > 30, got %v", age)
				}
			}
		}

		t.Logf("Filter gt test: %d rows returned", len(result))
	})

	// Test 4: IN filter
	t.Run("filter_in", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/users?status=in.(1)")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var result []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(result) < 5 {
			t.Errorf("Expected at least 5 results, got %d", len(result))
		}

		// Verify all results have status = 1
		for _, row := range result {
			if status, ok := row["status"].(float64); ok {
				if status != 1 {
					t.Errorf("Expected status = 1, got %v", status)
				}
			}
		}

		t.Logf("Filter in test: %d rows returned", len(result))
	})

	// Test 5: LIKE filter
	t.Run("filter_like", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/users?name=like.%25Smith")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var result []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(result) != 1 {
			t.Errorf("Expected 1 result, got %d", len(result))
		}

		if result[0]["name"] != "Bob Smith" {
			t.Errorf("Expected Bob Smith, got %v", result[0]["name"])
		}

		t.Logf("Filter like test: %d rows returned", len(result))
	})

	// Test 6: IS NULL filter
	t.Run("filter_is_null", func(t *testing.T) {
		// First, insert a row with NULL email
		_, err := db.Exec("INSERT INTO users (name, age, status) VALUES ('Test User', 25, 1)")
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}

		resp, err := http.Get(server.URL + "/users?email=is.null")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var result []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(result) != 1 {
			t.Errorf("Expected 1 result, got %d", len(result))
		}

		if result[0]["name"] != "Test User" {
			t.Errorf("Expected Test User, got %v", result[0]["name"])
		}

		t.Logf("Filter is null test: %d rows returned", len(result))
	})

	// Test 7: Ordering
	t.Run("order_by", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/users?select=name,age&order=age")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var result []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(result) < 2 {
			t.Errorf("Expected at least 2 results, got %d", len(result))
		}

		// Verify ordering (ascending by default)
		for i := 1; i < len(result); i++ {
			prevAge, _ := result[i-1]["age"].(float64)
			currAge, _ := result[i]["age"].(float64)
			if prevAge > currAge {
				t.Errorf("Expected ascending order, but %v > %v", prevAge, currAge)
			}
		}

		t.Logf("Order by test: %d rows returned", len(result))
	})

	// Test 8: Ordering descending
	t.Run("order_by_desc", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/users?select=name,age&order=age.desc")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var result []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(result) < 2 {
			t.Errorf("Expected at least 2 results, got %d", len(result))
		}

		// Verify ordering (descending)
		for i := 1; i < len(result); i++ {
			prevAge, _ := result[i-1]["age"].(float64)
			currAge, _ := result[i]["age"].(float64)
			if prevAge < currAge {
				t.Errorf("Expected descending order, but %v < %v", prevAge, currAge)
			}
		}

		t.Logf("Order by desc test: %d rows returned", len(result))
	})

	// Test 9: Limit
	t.Run("limit", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/users?limit=3")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var result []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(result) != 3 {
			t.Errorf("Expected 3 results, got %d", len(result))
		}

		t.Logf("Limit test: %d rows returned", len(result))
	})

	// Test 10: Offset
	t.Run("offset", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/users?limit=2&offset=2")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var result []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(result) != 2 {
			t.Errorf("Expected 2 results, got %d", len(result))
		}

		t.Logf("Offset test: %d rows returned", len(result))
	})

	// Test 11: Single row
	t.Run("single", func(t *testing.T) {
		cleanupTestData()
		resp, err := http.Get(server.URL + "/users?single=true&name=eq.Alice")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected status 200, got %d. Response: %s", resp.StatusCode, string(body))
			return
		}

		// Should return single object, not array
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if result["name"] != "Alice" {
			t.Errorf("Expected Alice, got %v", result["name"])
		}

		t.Logf("Single test: single object returned")
	})

	// Test 12: Maybe single (no results)
	t.Run("maybe_single_no_results", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/users?maybeSingle=true&name=eq.NonExistent")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		// Should return null
		var result interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if result != nil {
			t.Errorf("Expected null, got %v", result)
		}

		t.Logf("Maybe single no results test: null returned")
	})

	// Test 13: Maybe single (one result)
	t.Run("maybe_single_one_result", func(t *testing.T) {
		cleanupTestData()
		resp, err := http.Get(server.URL + "/users?maybeSingle=true&name=eq.Alice")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		// Should return single object
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if result["name"] != "Alice" {
			t.Errorf("Expected Alice, got %v", result["name"])
		}

		t.Logf("Maybe single one result test: single object returned")
	})

	// Test 14: Complex query with multiple filters
	t.Run("complex_query", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/users?select=name,age,department&age=gte.25&status=eq.1&department=in.(Engineering,Marketing)&order=age.desc&limit=3")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var result []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(result) != 3 {
			t.Errorf("Expected 3 results, got %d", len(result))
		}

		// Verify all results meet criteria
		for _, row := range result {
			if age, ok := row["age"].(float64); ok {
				if age < 25 {
					t.Errorf("Expected age >= 25, got %v", age)
				}
			}
			if status, ok := row["status"].(float64); ok {
				if status != 1 {
					t.Errorf("Expected status = 1, got %v", status)
				}
			}
			dept := row["department"].(string)
			if dept != "Engineering" && dept != "Marketing" {
				t.Errorf("Expected department in (Engineering, Marketing), got %v", dept)
			}
		}

		t.Logf("Complex query test: %d rows returned", len(result))
	})
}

// TestPostgRESTErrorHandling tests error scenarios
func TestPostgRESTErrorHandling(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	server := createTestServer(t, db)
	defer server.Close()

	// Test 1: Single with no results should return 404
	t.Run("single_no_results", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/users?single=true&name=eq.NonExistent")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", resp.StatusCode)
		}

		var errorResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			t.Fatalf("Failed to decode error response: %v", err)
		}

		if errorResp["error"] == nil {
			t.Error("Expected error message in response")
		}

		t.Logf("Single no results test: 404 returned with error message")
	})

	// Test 2: Single with multiple results should return 400
	t.Run("single_multiple_results", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/users?single=true&status=eq.1")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}

		var errorResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			t.Fatalf("Failed to decode error response: %v", err)
		}

		if errorResp["error"] == nil {
			t.Error("Expected error message in response")
		}

		t.Logf("Single multiple results test: 400 returned with error message")
	})

	// Test 3: Maybe single with multiple results should return 400
	t.Run("maybe_single_multiple_results", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/users?maybeSingle=true&status=eq.1")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}

		t.Logf("Maybe single multiple results test: 400 returned")
	})

	// Test 4: Invalid method should return 405
	t.Run("invalid_method", func(t *testing.T) {
		req, err := http.NewRequest("PUT", server.URL+"/users", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", resp.StatusCode)
		}

		t.Logf("Invalid method test: 405 returned")
	})
}

// TestSupabaseCompatibility tests specific Supabase client patterns
func TestSupabaseCompatibility(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	server := createTestServer(t, db)
	defer server.Close()

	// Test 1: Supabase-style select with chaining
	t.Run("supabase_select_chain", func(t *testing.T) {
		// Equivalent to: supabase.from('users').select('id,name').eq('status', 1).limit(5)
		resp, err := http.Get(server.URL + "/users?select=id,name&status=eq.1&limit=5")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var result []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(result) > 5 {
			t.Errorf("Expected max 5 results, got %d", len(result))
		}

		// Verify all results have status = 1
		for _, row := range result {
			if status, ok := row["status"].(float64); ok {
				if status != 1 {
					t.Errorf("Expected status = 1, got %v", status)
				}
			}
		}

		t.Logf("Supabase select chain test: %d rows returned", len(result))
	})

	// Test 2: Supabase-style single row
	t.Run("supabase_single", func(t *testing.T) {
		// Equivalent to: supabase.from('users').select('*').eq('name', 'Alice').single()
		resp, err := http.Get(server.URL + "/users?select=*&name=eq.Alice&single=true")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if result["name"] != "Alice" {
			t.Errorf("Expected Alice, got %v", result["name"])
		}

		t.Logf("Supabase single test: single object returned")
	})

	// Test 3: Supabase-style maybe single
	t.Run("supabase_maybe_single", func(t *testing.T) {
		// Equivalent to: supabase.from('users').select('*').eq('name', 'Alice').maybeSingle()
		resp, err := http.Get(server.URL + "/users?select=*&name=eq.Alice&maybeSingle=true")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if result["name"] != "Alice" {
			t.Errorf("Expected Alice, got %v", result["name"])
		}

		t.Logf("Supabase maybe single test: single object returned")
	})

	// Test 4: Response format compatibility
	t.Run("response_format_compatibility", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/users?limit=2")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		// Check Content-Type header
		contentType := resp.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", contentType)
		}

		// Check X-Total-Count header
		totalCount := resp.Header.Get("X-Total-Count")
		if totalCount == "" {
			t.Error("Expected X-Total-Count header")
		}

		// Verify response is a direct array (not wrapped in object)
		var result []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(result) != 2 {
			t.Errorf("Expected 2 results, got %d", len(result))
		}

		t.Logf("Response format compatibility test: proper headers and array format")
	})
}

// TestJoinEmbeddingNestedScanning verifies JOIN embedding produces nested JSON objects
func TestJoinEmbeddingNestedScanning(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    // Create related tables for embedding
    _, err := db.Exec(`
        CREATE TABLE posts (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            users_id INTEGER,
            title TEXT
        );
    `)
    if err != nil {
        t.Fatalf("Failed to create posts table: %v", err)
    }

    _, err = db.Exec(`
        CREATE TABLE profiles (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            users_id INTEGER,
            bio TEXT
        );
    `)
    if err != nil {
        t.Fatalf("Failed to create profiles table: %v", err)
    }

    // Seed one-to-one like data for deterministic nesting
    _, err = db.Exec(`
        INSERT INTO posts (users_id, title) VALUES 
        (1, 'Alice Post'),
        (2, 'Bob Post');
    `)
    if err != nil {
        t.Fatalf("Failed to insert posts: %v", err)
    }

    _, err = db.Exec(`
        INSERT INTO profiles (users_id, bio) VALUES 
        (1, 'Alice Bio'),
        (2, 'Bob Bio');
    `)
    if err != nil {
        t.Fatalf("Failed to insert profiles: %v", err)
    }

    server := createTestServer(t, db)
    defer server.Close()

    t.Run("embed_single_level", func(t *testing.T) {
        // Expect nested object at key "posts" with id and title
        resp, err := http.Get(server.URL + "/users?select=id,name,posts!left(id,title)&order=name&limit=1")
        if err != nil {
            t.Fatalf("Failed to make request: %v", err)
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            body, _ := io.ReadAll(resp.Body)
            t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
        }

        var result []map[string]interface{}
        if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
            t.Fatalf("Failed to decode response: %v", err)
        }

        if len(result) != 1 {
            t.Fatalf("Expected 1 row, got %d", len(result))
        }

        row := result[0]
        // Validate base columns
        if row["id"] == nil || row["name"] == nil {
            t.Fatalf("Expected base columns id and name present")
        }

        // Validate nested object structure
        postsObj, ok := row["posts"].(map[string]interface{})
        if !ok {
            t.Fatalf("Expected nested 'posts' object, got: %T", row["posts"])
        }

        if postsObj["id"] == nil || postsObj["title"] == nil {
            t.Fatalf("Expected 'posts' to contain id and title")
        }
    })

    t.Run("embed_multiple_single_level", func(t *testing.T) {
        // Expect two nested objects: posts and profiles
        resp, err := http.Get(server.URL + "/users?select=id,name,posts!left(id,title),profiles!left(bio)&order=name&limit=1")
        if err != nil {
            t.Fatalf("Failed to make request: %v", err)
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            body, _ := io.ReadAll(resp.Body)
            t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
        }

        var result []map[string]interface{}
        if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
            t.Fatalf("Failed to decode response: %v", err)
        }

        if len(result) != 1 {
            t.Fatalf("Expected 1 row, got %d", len(result))
        }

        row := result[0]

        // Validate posts object
        if _, ok := row["posts"].(map[string]interface{}); !ok {
            t.Fatalf("Expected nested 'posts' object")
        }

        // Validate profiles object
        profilesObj, ok := row["profiles"].(map[string]interface{})
        if !ok {
            t.Fatalf("Expected nested 'profiles' object")
        }
        if profilesObj["bio"] == nil {
            t.Fatalf("Expected 'profiles' to contain bio")
        }
    })
}

// TestSpecialCharacterHandling tests how PostgREST handles special characters in URLs and data
func TestSpecialCharacterHandling(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	server := createTestServer(t, db)
	defer server.Close()

	// Test 1: Space character in URL parameters
	t.Run("space_in_url_params", func(t *testing.T) {
		// Test different ways spaces might be encoded in URLs
		testCases := []struct {
			name        string
			url         string
			expected    string
			description string
			shouldWork  bool
		}{
			{
				name:        "space_as_plus",
				url:         "/users?name=eq.Mary+Jane+Watson",
				expected:    "Mary Jane Watson",
				description: "Space encoded as + in URL",
				shouldWork:  true,
			},
			{
				name:        "space_as_percent_20",
				url:         "/users?name=eq.Mary%20Jane%20Watson",
				expected:    "Mary Jane Watson",
				description: "Space encoded as %20 in URL",
				shouldWork:  true,
			},
			{
				name:        "space_as_literal_space",
				url:         "/users?name=eq.Mary Jane Watson",
				expected:    "Mary Jane Watson",
				description: "Space as literal space in URL (should fail)",
				shouldWork:  false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				resp, err := http.Get(server.URL + tc.url)
				if err != nil {
					t.Fatalf("Failed to make request: %v", err)
				}
				defer resp.Body.Close()

				// Handle shouldWork field for space tests
				if resp.StatusCode != http.StatusOK {
					body, _ := io.ReadAll(resp.Body)
					if tc.shouldWork {
						t.Errorf("Expected status 200, got %d. Response: %s", resp.StatusCode, string(body))
						return
					} else {
						t.Logf("%s: Expected failure, got status %d", tc.description, resp.StatusCode)
						return
					}
				}

				if !tc.shouldWork {
					t.Errorf("Expected failure, but got status 200")
					return
				}

				var result []map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if len(result) != 1 {
					t.Errorf("Expected 1 result, got %d", len(result))
					return
				}

				if result[0]["name"] != tc.expected {
					t.Errorf("Expected %s, got %v", tc.expected, result[0]["name"])
				}

				t.Logf("%s: %s", tc.description, result[0]["name"])
			})
		}
	})

	// Test 2: Special characters in LIKE patterns
	t.Run("special_chars_in_like", func(t *testing.T) {
		// Insert a user with special characters
		_, err := db.Exec("INSERT INTO users (name, email, age, status) VALUES ('Test%User', 'test@example.com', 30, 1)")
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}

		testCases := []struct {
			name        string
			url         string
			expected    int
			description string
		}{
			{
				name:        "percent_wildcard",
				url:         "/users?name=like.%25Jane%25",
				expected:    1,
				description: "LIKE with % wildcards",
			},
			{
				name:        "underscore_wildcard",
				url:         "/users?name=like.Mary_%25",
				expected:    1,
				description: "LIKE with _ wildcard",
			},
			{
				name:        "literal_percent",
				url:         "/users?name=eq.Test%25User",
				expected:    1,
				description: "Exact match with literal % character",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				resp, err := http.Get(server.URL + tc.url)
				if err != nil {
					t.Fatalf("Failed to make request: %v", err)
				}
				defer resp.Body.Close()

				var result []map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if len(result) != tc.expected {
					t.Errorf("Expected %d results, got %d", tc.expected, len(result))
				}

				t.Logf("%s: %d results", tc.description, len(result))
			})
		}
	})

	// Test 3: Single quotes and other special characters
	t.Run("quotes_and_special_chars", func(t *testing.T) {
		// Insert a user with special characters
		_, err := db.Exec("INSERT INTO users (name, email, age, status) VALUES ('O''Reilly', 'oreilly@example.com', 30, 1)")
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}

		testCases := []struct {
			name        string
			url         string
			expected    string
			description string
		}{
			{
				name:        "single_quote_escaped",
				url:         "/users?name=eq.O''Reilly",
				expected:    "O'Reilly",
				description: "Single quote escaped as double quote",
			},
			{
				name:        "single_quote_url_encoded",
				url:         "/users?name=eq.O%27Reilly",
				expected:    "O'Reilly",
				description: "Single quote URL encoded",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				resp, err := http.Get(server.URL + tc.url)
				if err != nil {
					t.Fatalf("Failed to make request: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					body, _ := io.ReadAll(resp.Body)
					t.Errorf("Expected status 200, got %d. Response: %s", resp.StatusCode, string(body))
					return
				}

				var result []map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if len(result) != 1 {
					t.Errorf("Expected 1 result, got %d", len(result))
					return
				}

				if result[0]["name"] != tc.expected {
					t.Errorf("Expected %s, got %v", tc.expected, result[0]["name"])
				}

				t.Logf("%s: %s", tc.description, result[0]["name"])
			})
		}
	})

	// Test 4: URL parameter parsing edge cases
	t.Run("url_parsing_edge_cases", func(t *testing.T) {
		testCases := []struct {
			name        string
			url         string
			shouldWork  bool
			description string
		}{
			{
				name:        "empty_value",
				url:         "/users?name=eq.",
				shouldWork:  true,
				description: "Empty value after operator",
			},
			{
				name:        "multiple_operators",
				url:         "/users?age=gt.25&age=lt.35",
				shouldWork:  true,
				description: "Multiple operators on same column",
			},
			{
				name:        "invalid_operator",
				url:         "/users?age=invalid.25",
				shouldWork:  false,
				description: "Invalid operator",
			},
			{
				name:        "malformed_filter",
				url:         "/users?age=gt",
				shouldWork:  true,
				description: "Malformed filter without value (treated as eq.gt)",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				resp, err := http.Get(server.URL + tc.url)
				if err != nil {
					t.Fatalf("Failed to make request: %v", err)
				}
				defer resp.Body.Close()

				if tc.shouldWork {
					if resp.StatusCode != http.StatusOK {
						body, _ := io.ReadAll(resp.Body)
						t.Errorf("Expected status 200, got %d. Response: %s", resp.StatusCode, string(body))
					}
				} else {
					if resp.StatusCode == http.StatusOK {
						t.Errorf("Expected error status, got 200")
					}
				}

				t.Logf("%s: status %d", tc.description, resp.StatusCode)
			})
		}
	})
}

// TestAdvancedSpecialCharacterHandling tests comprehensive special character scenarios
func TestAdvancedSpecialCharacterHandling(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	server := createTestServer(t, db)
	defer server.Close()

	// Test 1: Comprehensive special characters in data
	t.Run("comprehensive_special_chars_in_data", func(t *testing.T) {
		// Insert users with various special characters
		specialUsers := []struct {
			name  string
			email string
		}{
			{"User_With_Underscores", "user_underscore@example.com"},
			{"User/With/Slashes", "user/slash@example.com"},
			{"User\\With\\Backslashes", "user\\backslash@example.com"},
			{"User%With%Percents", "user%percent@example.com"},
			{"User'With'Quotes", "user'quote@example.com"},
			{"User\"With\"DoubleQuotes", "user\"double@example.com"},
			{"User<With>Brackets", "user<bracket@example.com"},
			{"User[With]SquareBrackets", "user[square@example.com"},
			{"User{With}Braces", "user{brace@example.com"},
			{"User(With)Parentheses", "user(parent@example.com"},
			{"User@With@AtSigns", "user@at@example.com"},
			{"User#With#Hash", "user#hash@example.com"},
			{"User$With$Dollar", "user$dollar@example.com"},
			{"User&With&Ampersand", "user&amp@example.com"},
			{"User*With*Asterisk", "user*asterisk@example.com"},
			{"User+With+Plus", "user+plus@example.com"},
			{"User=With=Equals", "user=equals@example.com"},
			{"User?With?Question", "user?question@example.com"},
			{"User!With!Exclamation", "user!exclamation@example.com"},
			{"User,With,Comma", "user,comma@example.com"},
			{"User;With;Semicolon", "user;semicolon@example.com"},
			{"User:With:Colon", "user:colon@example.com"},
		}

		for _, user := range specialUsers {
			_, err := db.Exec("INSERT INTO users (name, email, age, status) VALUES (?, ?, 30, 1)", user.name, user.email)
			if err != nil {
				t.Fatalf("Failed to insert user %s: %v", user.name, err)
			}
		}

		// Test exact matches for each special character type
		testCases := []struct {
			name        string
			url         string
			expected    string
			description string
		}{
			{
				name:        "underscore_exact",
				url:         "/users?name=eq.User_With_Underscores",
				expected:    "User_With_Underscores",
				description: "Exact match with underscores",
			},
			{
				name:        "slash_exact",
				url:         "/users?name=eq.User%2FWith%2FSlashes",
				expected:    "User/With/Slashes",
				description: "Exact match with slashes (URL encoded)",
			},
			{
				name:        "backslash_exact",
				url:         "/users?name=eq.User%5CWith%5CBackslashes",
				expected:    "User\\With\\Backslashes",
				description: "Exact match with backslashes (URL encoded)",
			},
			{
				name:        "percent_exact",
				url:         "/users?name=eq.User%25With%25Percents",
				expected:    "User%With%Percents",
				description: "Exact match with percents (URL encoded)",
			},
			{
				name:        "quote_exact",
				url:         "/users?name=eq.User%27With%27Quotes",
				expected:    "User'With'Quotes",
				description: "Exact match with quotes (URL encoded)",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				resp, err := http.Get(server.URL + tc.url)
				if err != nil {
					t.Fatalf("Failed to make request: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					body, _ := io.ReadAll(resp.Body)
					t.Errorf("Expected status 200, got %d. Response: %s", resp.StatusCode, string(body))
					return
				}

				var result []map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if len(result) != 1 {
					t.Errorf("Expected 1 result, got %d", len(result))
					return
				}

				if result[0]["name"] != tc.expected {
					t.Errorf("Expected %s, got %v", tc.expected, result[0]["name"])
				}

				t.Logf("%s: %s", tc.description, result[0]["name"])
			})
		}
	})

	// Test 2: LIKE patterns with special characters
	t.Run("like_patterns_with_special_chars", func(t *testing.T) {
		// Clear existing data to ensure test isolation
		db.Exec("DELETE FROM users")

		// Insert users with special characters for this test
		specialUsers := []struct {
			name  string
			email string
		}{
			{"User_With_Underscores", "user_underscore_like@example.com"},
			{"User/With/Slashes", "user/slash/like@example.com"},
			{"User\\With\\Backslashes", "user\\backslash\\like@example.com"},
			{"User%With%Percents", "user%percent%like@example.com"},
		}

		for _, user := range specialUsers {
			_, err := db.Exec("INSERT INTO users (name, email, age, status) VALUES (?, ?, 30, 1)", user.name, user.email)
			if err != nil {
				t.Fatalf("Failed to insert user %s: %v", user.name, err)
			}
		}

		testCases := []struct {
			name        string
			url         string
			expected    int
			description string
		}{
			{
				name:        "like_underscore_wildcard",
				url:         "/users?name=like.User_%25",
				expected:    4, // Matches all users with underscores
				description: "LIKE with underscore wildcard",
			},
			{
				name:        "like_percent_wildcard",
				url:         "/users?name=like.%25With%25",
				expected:    4, // Matches all users with "With" in name
				description: "LIKE with percent wildcard",
			},
			{
				name:        "like_slash_pattern",
				url:         "/users?name=like.%25%2F%25",
				expected:    1,
				description: "LIKE with slash pattern",
			},
			{
				name:        "like_backslash_pattern",
				url:         "/users?name=like.%25%5C%25",
				expected:    1,
				description: "LIKE with backslash pattern",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				resp, err := http.Get(server.URL + tc.url)
				if err != nil {
					t.Fatalf("Failed to make request: %v", err)
				}
				defer resp.Body.Close()

				var result []map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if len(result) != tc.expected {
					t.Errorf("Expected %d results, got %d", tc.expected, len(result))
				}

				t.Logf("%s: %d results", tc.description, len(result))
			})
		}
	})

	// Test 3: IN operator with special characters
	t.Run("in_operator_with_special_chars", func(t *testing.T) {
		// Clear existing data to ensure test isolation
		db.Exec("DELETE FROM users")

		// Insert users with special characters for this test
		specialUsers := []struct {
			name  string
			email string
		}{
			{"User_With_Underscores", "user_underscore_in@example.com"},
			{"User/With/Slashes", "user/slash/in@example.com"},
			{"User'With'Quotes", "user'quote'in@example.com"},
			{"User\"With\"DoubleQuotes", "user\"double\"in@example.com"},
		}

		for _, user := range specialUsers {
			_, err := db.Exec("INSERT INTO users (name, email, age, status) VALUES (?, ?, 30, 1)", user.name, user.email)
			if err != nil {
				t.Fatalf("Failed to insert user %s: %v", user.name, err)
			}
		}

		testCases := []struct {
			name        string
			url         string
			expected    int
			description string
		}{
			{
				name:        "in_with_underscores",
				url:         "/users?name=in.(User_With_Underscores,User%2FWith%2FSlashes)",
				expected:    2,
				description: "IN with underscores and slashes",
			},
			{
				name:        "in_with_quotes",
				url:         "/users?name=in.(User%27With%27Quotes,User%22With%22DoubleQuotes)",
				expected:    2,
				description: "IN with single and double quotes",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				resp, err := http.Get(server.URL + tc.url)
				if err != nil {
					t.Fatalf("Failed to make request: %v", err)
				}
				defer resp.Body.Close()

				var result []map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if len(result) != tc.expected {
					t.Errorf("Expected %d results, got %d", tc.expected, len(result))
				}

				t.Logf("%s: %d results", tc.description, len(result))
			})
		}
	})

	// Test 4: Edge cases and error handling
	t.Run("edge_cases_and_error_handling", func(t *testing.T) {
		testCases := []struct {
			name        string
			url         string
			shouldWork  bool
			description string
		}{
			{
				name:        "empty_string_value",
				url:         "/users?name=eq.",
				shouldWork:  true,
				description: "Empty string value",
			},
			{
				name:        "only_special_chars",
				url:         "/users?name=eq.%25%2F%5C",
				shouldWork:  true,
				description: "Only special characters",
			},
			{
				name:        "mixed_special_chars",
				url:         "/users?name=eq.Test%25%2F%5C%27%22",
				shouldWork:  true,
				description: "Mixed special characters",
			},
			{
				name:        "unicode_characters",
				url:         "/users?name=eq.Test%C3%A9%C3%A0%C3%A7",
				shouldWork:  true,
				description: "Unicode characters (éàç)",
			},
			{
				name:        "malformed_url_encoding",
				url:         "/users?name=eq.Test%2",
				shouldWork:  true, // HTTP client handles malformed encoding gracefully
				description: "Malformed URL encoding (handled by HTTP client)",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				resp, err := http.Get(server.URL + tc.url)
				if err != nil {
					if tc.shouldWork {
						t.Fatalf("Failed to make request: %v", err)
					} else {
						t.Logf("%s: Request failed as expected: %v", tc.description, err)
						return
					}
				}
				defer resp.Body.Close()

				if tc.shouldWork {
					if resp.StatusCode != http.StatusOK {
						body, _ := io.ReadAll(resp.Body)
						t.Errorf("Expected status 200, got %d. Response: %s", resp.StatusCode, string(body))
					}
				} else {
					if resp.StatusCode == http.StatusOK {
						t.Errorf("Expected error status, got 200")
					}
				}

				t.Logf("%s: status %d", tc.description, resp.StatusCode)
			})
		}
	})

	// Test 5: SQL injection prevention
	t.Run("sql_injection_prevention", func(t *testing.T) {
		// Insert a test user
		_, err := db.Exec("INSERT INTO users (name, email, age, status) VALUES ('NormalUser', 'normal@example.com', 30, 1)")
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}

		testCases := []struct {
			name        string
			url         string
			shouldWork  bool
			description string
		}{
			{
				name:        "sql_injection_attempt_1",
				url:         "/users?name=eq.'; DROP TABLE users; --",
				shouldWork:  true, // Should work but not execute SQL injection
				description: "SQL injection attempt 1",
			},
			{
				name:        "sql_injection_attempt_2",
				url:         "/users?name=eq.1' OR '1'='1",
				shouldWork:  true, // Should work but not execute SQL injection
				description: "SQL injection attempt 2",
			},
			{
				name:        "sql_injection_attempt_3",
				url:         "/users?name=eq.1' UNION SELECT * FROM users --",
				shouldWork:  true, // Should work but not execute SQL injection
				description: "SQL injection attempt 3",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				resp, err := http.Get(server.URL + tc.url)
				if err != nil {
					t.Fatalf("Failed to make request: %v", err)
				}
				defer resp.Body.Close()

				// Check if the response is an error (which is good for SQL injection prevention)
				if resp.StatusCode != http.StatusOK {
					body, _ := io.ReadAll(resp.Body)
					t.Logf("%s: SQL injection prevented with status %d: %s", tc.description, resp.StatusCode, string(body))
					return
				}

				// Should return empty results, not execute the injection
				var result []map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					// If we can't decode as array, it might be an error response
					body, _ := io.ReadAll(resp.Body)
					t.Logf("%s: SQL injection prevented with error response: %s", tc.description, string(body))
					return
				}

				// Should return 0 results (no user with that exact name)
				if len(result) != 0 {
					t.Errorf("Expected 0 results (SQL injection prevented), got %d", len(result))
				}

				t.Logf("%s: %d results (injection prevented)", tc.description, len(result))
			})
		}

		// Verify the table still exists and has data
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to verify table integrity: %v", err)
		}
		if count == 0 {
			t.Error("Table was dropped by SQL injection!")
		}
		t.Logf("Table integrity verified: %d users remain", count)
	})
}
