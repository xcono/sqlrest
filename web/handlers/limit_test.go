package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/xcono/sqlrest/web/database"
)

func TestLimitParameter(t *testing.T) {
	// Create a test database connection
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
	_, err = db.Exec("DROP TABLE IF EXISTS test_limit")
	if err != nil {
		t.Fatalf("failed to drop test table: %v", err)
	}

	_, err = db.Exec("CREATE TABLE test_limit (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(255))")
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

	// Clean up after test
	defer func() {
		db.Exec("DROP TABLE IF EXISTS test_limit")
	}()

	// Insert test data (more than the limit we'll test)
	_, err = db.Exec("INSERT INTO test_limit (name) VALUES (?), (?), (?), (?), (?)",
		"User1", "User2", "User3", "User4", "User5")
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Create handler using the new refactored structure
	dbExecutor := database.NewExecutor(db)
	router := NewRouter(dbExecutor)

	// Test with limit=3
	t.Run("limit_3", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test_limit?limit=3", nil)
		w := httptest.NewRecorder()

		router.HandleTable(w, req)

		// Check response
		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		// Check response body - should be direct array for PostgREST compatibility
		var result []map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Should return exactly 3 rows due to limit
		if len(result) != 3 {
			t.Errorf("expected 3 rows due to limit=3, got %d rows", len(result))
		}

		t.Logf("Limit test result: %d rows returned", len(result))
	})

	// Test with limit=1
	t.Run("limit_1", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test_limit?limit=1", nil)
		w := httptest.NewRecorder()

		router.HandleTable(w, req)

		// Check response
		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		// Check response body - should be direct array for PostgREST compatibility
		var result []map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Should return exactly 1 row due to limit
		if len(result) != 1 {
			t.Errorf("expected 1 row due to limit=1, got %d rows", len(result))
		}

		t.Logf("Limit test result: %d rows returned", len(result))
	})

	// Test with limit=10 (more than available data)
	t.Run("limit_10", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test_limit?limit=10", nil)
		w := httptest.NewRecorder()

		router.HandleTable(w, req)

		// Check response
		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		// Check response body - should be direct array for PostgREST compatibility
		var result []map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Should return all 5 rows (less than limit)
		if len(result) != 5 {
			t.Errorf("expected 5 rows (all available), got %d rows", len(result))
		}

		t.Logf("Limit test result: %d rows returned", len(result))
	})
}
