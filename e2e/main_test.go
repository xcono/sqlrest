package e2e_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"

	"github.com/xcono/sqlrest/e2e/compare"
	"github.com/xcono/sqlrest/e2e/containers"
	"github.com/xcono/sqlrest/web/database"
	"github.com/xcono/sqlrest/web/handlers"
)

// Package-level variables for containers
var testContainers *containers.TestContainers

// TestMain manages container lifecycle for all tests
func TestMain(m *testing.M) {
	// Commented out full container setup to allow individual container tests
	// Uncomment when ready to run full e2e tests with all containers

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Setup all containers
	var err error
	testContainers, err = containers.SetupAllContainers(ctx)
	if err != nil {
		log.Fatalf("Failed to setup containers: %v", err)
	}

	// Ensure cleanup on exit
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()

		if err := testContainers.Cleanup(cleanupCtx); err != nil {
			log.Printf("Warning: Failed to cleanup containers: %v", err)
		}
	}()

	// Run tests
	code := m.Run()
	os.Exit(code)
}

// TestConfig holds configuration for e2e tests
type TestConfig struct {
	PostgRESTURL string
	MySQLDSN     string
	PostgresDSN  string
}

// DefaultTestConfig returns the default test configuration using containers
func DefaultTestConfig() *TestConfig {
	if testContainers == nil {
		panic("TestMain must be called before DefaultTestConfig")
	}

	return &TestConfig{
		PostgRESTURL: testContainers.PostgRESTURL,
		MySQLDSN:     testContainers.MySQLDSN,
		PostgresDSN:  testContainers.PostgresDSN,
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

	// Connect to MySQL database (already seeded by init scripts)
	t.Logf("Connecting to MySQL with DSN: %s", config.MySQLDSN)
	mysqlDB, err := sql.Open("mysql", config.MySQLDSN)
	if err != nil {
		t.Fatalf("Failed to connect to MySQL: %v", err)
	}

	// Configure connection pool
	mysqlDB.SetMaxOpenConns(10)
	mysqlDB.SetMaxIdleConns(5)

	// Test the MySQL connection with retry
	var pingErr error
	for i := 0; i < 5; i++ {
		pingErr = mysqlDB.Ping()
		if pingErr == nil {
			break
		}
		t.Logf("MySQL ping attempt %d failed: %v", i+1, pingErr)
		time.Sleep(time.Second)
	}
	if pingErr != nil {
		t.Fatalf("Failed to ping MySQL database after 5 attempts: %v", pingErr)
	}

	// Verify PostgreSQL connection (already seeded by init scripts)
	postgresDB, err := sql.Open("postgres", config.PostgresDSN)
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}

	// Test the PostgreSQL connection
	if err := postgresDB.Ping(); err != nil {
		t.Fatalf("Failed to ping PostgreSQL database: %v", err)
	}
	postgresDB.Close()

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
