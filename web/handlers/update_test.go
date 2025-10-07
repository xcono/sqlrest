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

func TestUpdateHandler_Handle(t *testing.T) {
	tests := []struct {
		name           string
		tableName      string
		requestBody    string
		queryParams    string
		mockSetup      func(mock sqlmock.Sqlmock)
		expectedStatus int
		expectedError  bool
	}{
		{
			name:        "valid single column update",
			tableName:   "artist",
			requestBody: `{"name": "Updated Artist"}`,
			queryParams: "artist_id=eq.1",
			mockSetup: func(mock sqlmock.Sqlmock) {
				// Mock UPDATE query
				mock.ExpectExec("UPDATE artist SET name = \\? WHERE artist_id = \\?").
					WithArgs("Updated Artist", 1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectedStatus: http.StatusNoContent,
			expectedError:  false,
		},
		{
			name:        "valid single column update album",
			tableName:   "album",
			requestBody: `{"title": "New Title"}`,
			queryParams: "album_id=eq.1",
			mockSetup: func(mock sqlmock.Sqlmock) {
				// Mock UPDATE query
				mock.ExpectExec("UPDATE album SET title = \\? WHERE album_id = \\?").
					WithArgs("New Title", 1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectedStatus: http.StatusNoContent,
			expectedError:  false,
		},
		{
			name:        "update with returning representation",
			tableName:   "artist",
			requestBody: `{"name": "Representation Return"}`,
			queryParams: "artist_id=eq.1&returning=representation",
			mockSetup: func(mock sqlmock.Sqlmock) {
				// Mock UPDATE query
				mock.ExpectExec("UPDATE artist SET name = \\? WHERE artist_id = \\?").
					WithArgs("Representation Return", 1).
					WillReturnResult(sqlmock.NewResult(0, 1))

				// Mock SELECT query for returning data
				rows := sqlmock.NewRows([]string{"artist_id", "name"}).
					AddRow(1, "Representation Return")
				mock.ExpectQuery("SELECT (.+) FROM artist AS (.+) WHERE artist_id = \\?").
					WithArgs(1).
					WillReturnRows(rows)
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
		},
		{
			name:        "no rows affected - should return 404",
			tableName:   "artist",
			requestBody: `{"name": "Non-existent"}`,
			queryParams: "artist_id=eq.999",
			mockSetup: func(mock sqlmock.Sqlmock) {
				// Mock UPDATE query returning 0 rows affected
				mock.ExpectExec("UPDATE artist SET name = \\? WHERE artist_id = \\?").
					WithArgs("Non-existent", 999).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  false,
		},
		{
			name:        "invalid JSON body",
			tableName:   "artist",
			requestBody: `{"name": "Invalid JSON"`,
			queryParams: "artist_id=eq.1",
			mockSetup: func(mock sqlmock.Sqlmock) {
				// No database calls expected for invalid JSON
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  false,
		},
		{
			name:        "array instead of object - should fail",
			tableName:   "artist",
			requestBody: `[{"name": "Array Item"}]`,
			queryParams: "artist_id=eq.1",
			mockSetup: func(mock sqlmock.Sqlmock) {
				// No database calls expected for invalid data format
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  false,
		},
		{
			name:        "empty update object",
			tableName:   "artist",
			requestBody: `{}`,
			queryParams: "artist_id=eq.1",
			mockSetup: func(mock sqlmock.Sqlmock) {
				// No database calls expected for empty object
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  false,
		},
		{
			name:        "no filters provided - should fail",
			tableName:   "artist",
			requestBody: `{"name": "Should Fail"}`,
			queryParams: "",
			mockSetup: func(mock sqlmock.Sqlmock) {
				// No database calls expected for missing filters
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  false,
		},
		{
			name:        "database error during update",
			tableName:   "artist",
			requestBody: `{"name": "Database Error"}`,
			queryParams: "artist_id=eq.1",
			mockSetup: func(mock sqlmock.Sqlmock) {
				// Mock UPDATE query returning error
				mock.ExpectExec("UPDATE artist SET name = \\? WHERE artist_id = \\?").
					WithArgs("Database Error", 1).
					WillReturnError(sql.ErrConnDone)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  false,
		},
		{
			name:        "complex filter with gt operator",
			tableName:   "track",
			requestBody: `{"unit_price": 1.99}`,
			queryParams: "track_id=gt.500&limit=5",
			mockSetup: func(mock sqlmock.Sqlmock) {
				// Mock UPDATE query with gt filter
				mock.ExpectExec("UPDATE track SET unit_price = \\? WHERE track_id > \\?").
					WithArgs(1.99, 500).
					WillReturnResult(sqlmock.NewResult(0, 3))
			},
			expectedStatus: http.StatusNoContent,
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock database
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("Failed to create mock: %v", err)
			}
			defer db.Close()

			// Setup mock expectations
			tt.mockSetup(mock)

			// Create handler with mock database
			dbExecutor := database.NewExecutor(db)
			handler := NewUpdateHandler(dbExecutor)

			// Create request
			req := httptest.NewRequest(http.MethodPatch, "/"+tt.tableName+"?"+tt.queryParams, bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

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

func TestUpdateHandler_parseUpdateData(t *testing.T) {
	handler := &UpdateHandler{}

	tests := []struct {
		name      string
		input     interface{}
		expected  map[string]interface{}
		expectErr bool
	}{
		{
			name:      "valid object",
			input:     map[string]interface{}{"name": "Test", "age": 25},
			expected:  map[string]interface{}{"name": "Test", "age": 25},
			expectErr: false,
		},
		{
			name:      "array should fail",
			input:     []interface{}{map[string]interface{}{"name": "Test"}},
			expected:  nil,
			expectErr: true,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.parseUpdateData(tt.input)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(result) != len(tt.expected) {
					t.Errorf("Expected %d fields, got %d", len(tt.expected), len(result))
				}
			}
		})
	}
}

func TestUpdateHandler_executeUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	handler := &UpdateHandler{
		db:      database.NewExecutor(db),
		builder: builder.NewPostgRESTBuilder(),
	}

	// Mock successful update
	mock.ExpectExec("UPDATE artist SET name = \\?").
		WithArgs("Updated Name").
		WillReturnResult(sqlmock.NewResult(0, 1))

	updates := map[string]interface{}{"name": "Updated Name"}
	filters := []interface{}{} // Empty filters for this test

	result, err := handler.executeUpdate("artist", updates, filters)
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

func TestUpdateError(t *testing.T) {
	err := &UpdateError{Message: "Test error"}
	if err.Error() != "Test error" {
		t.Errorf("Expected 'Test error', got '%s'", err.Error())
	}
}
