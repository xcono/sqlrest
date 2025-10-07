package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/xcono/sqlrest/builder"
	"github.com/xcono/sqlrest/web/database"
)

func TestUpsertHandler_Handle(t *testing.T) {
	tests := []struct {
		name           string
		tableName      string
		requestBody    string
		queryParams    string
		headers        map[string]string
		mockSetup      func(mock sqlmock.Sqlmock)
		expectedStatus int
		expectedError  bool
	}{
		{
			name:        "valid single object upsert - new record",
			tableName:   "artist",
			requestBody: `{"artist_id": 999, "name": "New Artist"}`,
			queryParams: "returning=minimal",
			headers:     map[string]string{"Prefer": "resolution=merge-duplicates"},
			mockSetup: func(mock sqlmock.Sqlmock) {
				// Mock UPSERT query (INSERT with ON DUPLICATE KEY UPDATE)
				// Use exact SQL string matching with float64 for JSON numbers
				mock.ExpectExec("INSERT INTO artist (artist_id, name) VALUES (?, ?) ON DUPLICATE KEY UPDATE artist_id = VALUES(artist_id), name = VALUES(name)").
					WithArgs(float64(999), "New Artist").
					WillReturnResult(sqlmock.NewResult(999, 1))
				// No SELECT query expected for minimal mode
			},
			expectedStatus: http.StatusCreated,
			expectedError:  false,
		},
		{
			name:        "valid single object upsert - existing record",
			tableName:   "artist",
			requestBody: `{"artist_id": 1, "name": "Updated Artist"}`,
			queryParams: "returning=minimal",
			mockSetup: func(mock sqlmock.Sqlmock) {
				// Mock UPSERT query (UPDATE case)
				mock.ExpectExec("INSERT INTO artist (artist_id, name) VALUES (?, ?) ON DUPLICATE KEY UPDATE artist_id = VALUES(artist_id), name = VALUES(name)").
					WithArgs(float64(1), "Updated Artist").
					WillReturnResult(sqlmock.NewResult(1, 2)) // 2 rows affected for UPDATE
				// No SELECT query expected for minimal mode
			},
			expectedStatus: http.StatusCreated,
			expectedError:  false,
		},
		{
			name:        "valid array upsert",
			tableName:   "genre",
			requestBody: `[{"genre_id": 1, "name": "Updated Rock"}, {"genre_id": 100, "name": "New Jazz"}]`,
			queryParams: "returning=minimal",
			mockSetup: func(mock sqlmock.Sqlmock) {
				// Mock UPSERT query for array
				mock.ExpectExec("INSERT INTO genre (genre_id, name) VALUES (?, ?), (?, ?) ON DUPLICATE KEY UPDATE genre_id = VALUES(genre_id), name = VALUES(name)").
					WithArgs(float64(1), "Updated Rock", float64(100), "New Jazz").
					WillReturnResult(sqlmock.NewResult(100, 2))
				// No SELECT query expected for minimal mode
			},
			expectedStatus: http.StatusCreated,
			expectedError:  false,
		},
		{
			name:        "upsert with returning minimal",
			tableName:   "artist",
			requestBody: `{"artist_id": 998, "name": "Minimal Upsert"}`,
			queryParams: "returning=minimal",
			mockSetup: func(mock sqlmock.Sqlmock) {
				// Mock UPSERT query
				mock.ExpectExec("INSERT INTO artist (artist_id, name) VALUES (?, ?) ON DUPLICATE KEY UPDATE artist_id = VALUES(artist_id), name = VALUES(name)").
					WithArgs(float64(998), "Minimal Upsert").
					WillReturnResult(sqlmock.NewResult(998, 1))
				// No SELECT query expected for minimal mode
			},
			expectedStatus: http.StatusCreated,
			expectedError:  false,
		},
		{
			name:        "upsert with returning representation - unsupported",
			tableName:   "artist",
			requestBody: `{"artist_id": 997, "name": "Representation Upsert"}`,
			queryParams: "returning=representation",
			mockSetup: func(mock sqlmock.Sqlmock) {
				// Mock UPSERT query
				mock.ExpectExec("INSERT INTO artist (artist_id, name) VALUES (?, ?) ON DUPLICATE KEY UPDATE artist_id = VALUES(artist_id), name = VALUES(name)").
					WithArgs(float64(997), "Representation Upsert").
					WillReturnResult(sqlmock.NewResult(997, 1))
				// No SELECT query expected - handler returns error for representation
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  false,
		},
		{
			name:        "invalid JSON body",
			tableName:   "artist",
			requestBody: `{"artist_id": 1, "name": "Invalid JSON"`,
			queryParams: "",
			mockSetup: func(mock sqlmock.Sqlmock) {
				// No database calls expected for invalid JSON
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  false,
		},
		{
			name:        "empty upsert object",
			tableName:   "artist",
			requestBody: `{}`,
			queryParams: "",
			mockSetup: func(mock sqlmock.Sqlmock) {
				// No database calls expected for empty object
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  false,
		},
		{
			name:        "array with invalid elements",
			tableName:   "artist",
			requestBody: `[{"artist_id": 1, "name": "Valid"}, "invalid"]`,
			queryParams: "",
			mockSetup: func(mock sqlmock.Sqlmock) {
				// No database calls expected for invalid array elements
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  false,
		},
		{
			name:        "database error during upsert",
			tableName:   "artist",
			requestBody: `{"artist_id": 1, "name": "Database Error"}`,
			queryParams: "",
			mockSetup: func(mock sqlmock.Sqlmock) {
				// Mock UPSERT query returning error
				mock.ExpectExec("INSERT INTO artist (artist_id, name) VALUES (?, ?) ON DUPLICATE KEY UPDATE artist_id = VALUES(artist_id), name = VALUES(name)").
					WithArgs(float64(1), "Database Error").
					WillReturnError(sql.ErrConnDone)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  false,
		},
		{
			name:        "constraint violation",
			tableName:   "artist",
			requestBody: `{"artist_id": 1, "name": "Constraint Violation"}`,
			queryParams: "",
			mockSetup: func(mock sqlmock.Sqlmock) {
				// Mock UPSERT query returning constraint violation error
				mock.ExpectExec("INSERT INTO artist (artist_id, name) VALUES (?, ?) ON DUPLICATE KEY UPDATE artist_id = VALUES(artist_id), name = VALUES(name)").
					WithArgs(float64(1), "Constraint Violation").
					WillReturnError(sql.ErrTxDone) // Use a valid SQL error
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock database with exact string matching
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("Failed to create mock: %v", err)
			}
			defer db.Close()

			// Setup mock expectations
			tt.mockSetup(mock)

			// Create handler with mock database
			dbExecutor := database.NewExecutor(db)
			handler := NewUpsertHandler(dbExecutor)

			// Create request
			req := httptest.NewRequest(http.MethodPost, "/"+tt.tableName+"?"+tt.queryParams, bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			// Add custom headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			// Create response recorder
			w := httptest.NewRecorder()

			// Execute handler
			handler.Handle(w, req, tt.tableName)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Check if we expected an error response
			if tt.expectedError {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err == nil {
					if _, hasError := response["error"]; !hasError {
						t.Error("Expected error response but got success")
					}
				}
			}

			// Verify all expectations were met
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("Unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestUpsertHandler_parseUpsertData(t *testing.T) {
	handler := &UpsertHandler{}

	tests := []struct {
		name      string
		input     interface{}
		expected  []map[string]interface{}
		expectErr bool
	}{
		{
			name:      "valid single object",
			input:     map[string]interface{}{"name": "Test", "age": 25},
			expected:  []map[string]interface{}{{"name": "Test", "age": 25}},
			expectErr: false,
		},
		{
			name: "valid array of objects",
			input: []interface{}{
				map[string]interface{}{"name": "Test1", "age": 25},
				map[string]interface{}{"name": "Test2", "age": 30},
			},
			expected: []map[string]interface{}{
				{"name": "Test1", "age": 25},
				{"name": "Test2", "age": 30},
			},
			expectErr: false,
		},
		{
			name:      "string should fail",
			input:     "invalid",
			expected:  nil,
			expectErr: true,
		},
		{
			name:      "number should fail",
			input:     123,
			expected:  nil,
			expectErr: true,
		},
		{
			name:      "array with non-object elements should fail",
			input:     []interface{}{"invalid", 123},
			expected:  nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.parseUpsertData(tt.input)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(result) != len(tt.expected) {
					t.Errorf("Expected %d items, got %d", len(tt.expected), len(result))
				}
			}
		})
	}
}

func TestUpsertHandler_executeUpsert(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	handler := &UpsertHandler{
		db:      database.NewExecutor(db),
		builder: builder.NewPostgRESTBuilder(),
	}

	// Mock successful upsert
	mock.ExpectExec("INSERT INTO artist \\(artist_id, name\\) VALUES \\(\\?, \\?\\) ON DUPLICATE KEY UPDATE artist_id = VALUES\\(artist_id\\), name = VALUES\\(name\\)").
		WithArgs(1, "Updated Name").
		WillReturnResult(sqlmock.NewResult(1, 1))

	values := []map[string]interface{}{
		{"artist_id": 1, "name": "Updated Name"},
	}

	result, err := handler.executeUpsert("artist", values)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		t.Errorf("Failed to get rows affected: %v", err)
	}

	if rowsAffected != 1 {
		t.Errorf("Expected 1 row affected, got %d", rowsAffected)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestUpsertError(t *testing.T) {
	err := &UpsertError{Message: "Test error"}
	if err.Error() != "Test error" {
		t.Errorf("Expected 'Test error', got '%s'", err.Error())
	}
}

func TestConvertToStrings(t *testing.T) {
	input := []interface{}{1, "test", 3.14, true}
	expected := []string{"1", "test", "3.14", "true"}

	result := convertToStrings(input)

	if len(result) != len(expected) {
		t.Errorf("Expected %d items, got %d", len(expected), len(result))
	}

	for i, val := range result {
		if val != expected[i] {
			t.Errorf("Expected %s, got %s", expected[i], val)
		}
	}
}
