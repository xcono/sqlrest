package e2e

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xcono/sqlrest/e2e/compare"
	"github.com/xcono/sqlrest/e2e/dbseed"
	"github.com/xcono/sqlrest/web/database"
	"github.com/xcono/sqlrest/web/handlers"
)

// TestConfig holds configuration for e2e tests
type TestConfig struct {
	PostgRESTURL string
	MySQLDSN     string
	PostgresDSN  string
}

// DefaultTestConfig returns the default test configuration
func DefaultTestConfig() *TestConfig {
	return &TestConfig{
		PostgRESTURL: "http://localhost:3001",
		MySQLDSN:     "testuser:testpass@tcp(127.0.0.1:3307)/chinook_auto_increment",
		PostgresDSN:  "postgres://postgres:postgres@localhost:5433/chinook_auto_increment?sslmode=disable",
	}
}

// TestSuite provides utilities for running e2e tests
type TestSuite struct {
	config        *TestConfig
	mysqlDB       *sql.DB
	sqlRestServer *httptest.Server
}

// NewTestSuite creates a new test suite with the given configuration
func NewTestSuite(t *testing.T, config *TestConfig) *TestSuite {
	if config == nil {
		config = DefaultTestConfig()
	}

	// Seed databases
	mysqlDB := dbseed.SeedMySQL(t, config.MySQLDSN)
	dbseed.SeedPostgres(t, config.PostgresDSN)

	// Start SQL-REST server
	sqlRestServer := startSQLRestServer(t, mysqlDB)

	return &TestSuite{
		config:        config,
		mysqlDB:       mysqlDB,
		sqlRestServer: sqlRestServer,
	}
}

// Close cleans up the test suite
func (ts *TestSuite) Close() {
	if ts.mysqlDB != nil {
		ts.mysqlDB.Close()
	}
	if ts.sqlRestServer != nil {
		ts.sqlRestServer.Close()
	}
}

// QueryPostgREST queries the PostgREST server
func (ts *TestSuite) QueryPostgREST(t *testing.T, query string) compare.Response {
	return queryAPI(t, ts.config.PostgRESTURL+query)
}

// QuerySQLREST queries the sqlrest server
func (ts *TestSuite) QuerySQLREST(t *testing.T, query string) compare.Response {
	return queryAPI(t, ts.sqlRestServer.URL+query)
}

// CompareQueries compares responses from both servers
func (ts *TestSuite) CompareQueries(t *testing.T, query string) error {
	pgResp := ts.QueryPostgREST(t, query)
	srResp := ts.QuerySQLREST(t, query)
	return compare.CompareResponses(pgResp, srResp)
}

// TestCase represents a single test case
type TestCase struct {
	Name        string
	Query       string
	ExpectError bool
	Description string
}

// RunTestCase runs a single test case
func (ts *TestSuite) RunTestCase(t *testing.T, tc TestCase) {
	t.Run(tc.Name, func(t *testing.T) {
		if tc.Description != "" {
			t.Logf("Description: %s", tc.Description)
		}

		err := ts.CompareQueries(t, tc.Query)

		if tc.ExpectError {
			if err == nil {
				t.Logf("Expected error but got none for query: %s", tc.Query)
			} else {
				t.Logf("Expected error (documented incompatibility): %v", err)
			}
		} else {
			if err != nil {
				t.Errorf("Response mismatch for %s: %v", tc.Query, err)
			}
		}
	})
}

// RunTestCases runs multiple test cases
func (ts *TestSuite) RunTestCases(t *testing.T, testCases []TestCase) {
	for _, tc := range testCases {
		ts.RunTestCase(t, tc)
	}
}

// Helper functions
func startSQLRestServer(t *testing.T, db *sql.DB) *httptest.Server {
	dbExecutor := database.NewExecutor(db)
	router := handlers.NewRouter(dbExecutor)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		router.HandleTable(w, r)
	})

	return httptest.NewServer(mux)
}

func queryAPI(t *testing.T, url string) compare.Response {
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to query %s: %v", url, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var data interface{}
	json.Unmarshal(body, &data)

	return compare.Response{
		Data:       data,
		StatusCode: resp.StatusCode,
		Headers: map[string]string{
			"Content-Type": resp.Header.Get("Content-Type"),
		},
	}
}
