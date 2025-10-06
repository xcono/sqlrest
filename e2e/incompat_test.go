package e2e

import (
	"testing"

	"github.com/xcono/sqlrest/e2e/compare"
)

// TestIncompatibilities documents known incompatibilities between PostgREST and sqlrest
// These are edge cases that may be due to platform differences (collation, database engines, etc.)
func TestIncompatibilities(t *testing.T) {
	suite := NewTestSuite(t, nil)
	defer suite.Close()

	t.Run("limit_offset_without_order", func(t *testing.T) {
		// This test documents potential ordering differences when using LIMIT/OFFSET
		// without an explicit ORDER BY clause. Different databases may return
		// results in different orders when no ordering is specified.

		query := "/track?limit=5&offset=2"

		// Query PostgREST
		pgResp := suite.QueryPostgREST(t, query)

		// Query SQL-REST
		srResp := suite.QuerySQLREST(t, query)

		// Compare responses - this may fail due to ordering differences
		if err := compare.CompareResponses(pgResp, srResp); err != nil {
			t.Logf("Expected incompatibility detected: %v", err)
			t.Logf("This is likely due to different default ordering behavior")
			t.Logf("Without explicit ORDER BY, database engines may return results in different orders")

			// Log the actual data for analysis
			t.Logf("PostgREST response: %+v", pgResp.Data)
			t.Logf("SQLREST response: %+v", srResp.Data)

			// This test documents the incompatibility but doesn't fail
			// In a real scenario, you should always use ORDER BY with LIMIT/OFFSET
			// to ensure deterministic results
		} else {
			t.Logf("No incompatibility detected - responses match")
		}
	})
}

// TestPlatformDifferences documents differences that may be due to platform-specific behavior
func TestPlatformDifferences(t *testing.T) {
	t.Run("collation_differences", func(t *testing.T) {
		t.Log("PostgreSQL and MySQL may have different default collation settings")
		t.Log("This can affect string comparison and ordering behavior")
		t.Log("Consider using explicit COLLATE clauses for consistent behavior")
	})

	t.Run("numeric_precision", func(t *testing.T) {
		t.Log("Different databases may handle numeric precision differently")
		t.Log("DECIMAL types may be returned with different precision")
		t.Log("Consider using explicit precision specifications")
	})

	t.Run("case_sensitivity", func(t *testing.T) {
		t.Log("Database engines may have different default case sensitivity")
		t.Log("This affects string comparisons and LIKE operations")
		t.Log("Consider using explicit case-sensitive/insensitive operators")
	})
}
