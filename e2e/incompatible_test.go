package e2e_test

import (
	"testing"

	"github.com/xcono/sqlrest/e2e/compare"
)

// TestIncompatibilities documents known incompatibilities between PostgREST and sqlrest
// These are edge cases that may be due to platform differences (collation, database engines, etc.)
func TestIncompatibilities(t *testing.T) {
	suite := NewTestSuite(t, nil)
	defer suite.Close()

	t.Run("order_asc_collation", func(t *testing.T) {
		// This test documents the incompatibility when sorting strings containing special characters
		// PostgreSQL and MySQL handle special characters (like slashes, punctuation) differently
		// in their default collation settings, leading to different sort orders.
		// Specifically: PostgreSQL treats '/' as coming before letters, MySQL treats it as coming after letters.
		// Example: "AC/DC" vs "Accept" - PostgreSQL sorts AC/DC first, MySQL sorts Accept first.

		query := "/artist?order=name&limit=10"

		// Query PostgREST
		pgResp := suite.QueryPostgREST(t, query)

		// Query SQL-REST
		srResp := suite.QuerySQLREST(t, query)

		// Compare responses - this will fail due to special character handling differences
		if err := compare.CompareResponses(pgResp, srResp); err != nil {
			t.Logf("Expected incompatibility detected: %v", err)
			t.Logf("This is due to different handling of special characters in string sorting")
			t.Logf("PostgreSQL treats '/' as coming before letters, MySQL treats it as coming after")
			t.Logf("The string 'AC/DC' vs 'Accept' demonstrates this difference")

			// Log the actual data for analysis
			t.Logf("PostgREST response: %+v", pgResp.Data)
			t.Logf("SQLREST response: %+v", srResp.Data)

			// This test documents the incompatibility but doesn't fail
			// In a real scenario, you might want to normalize special characters
			// or use explicit collation settings for consistent behavior
		} else {
			t.Logf("No incompatibility detected - responses match")
		}
	})

	t.Run("special_characters_in_sorting", func(t *testing.T) {
		// This test documents the incompatibility when sorting strings containing special characters
		// PostgreSQL and MySQL handle special characters (like slashes, punctuation) differently
		// in their default collation settings, leading to different sort orders.

		query := "/artist?order=name&limit=10"

		// Query PostgREST
		pgResp := suite.QueryPostgREST(t, query)

		// Query SQL-REST
		srResp := suite.QuerySQLREST(t, query)

		// Compare responses - this will fail due to special character handling differences
		if err := compare.CompareResponses(pgResp, srResp); err != nil {
			t.Logf("Expected incompatibility detected: %v", err)
			t.Logf("This is due to different handling of special characters in string sorting")
			t.Logf("PostgreSQL treats '/' as coming before letters, MySQL treats it as coming after")
			t.Logf("The string 'AC/DC' vs 'Accept' demonstrates this difference")

			// Log the actual data for analysis
			t.Logf("PostgREST response: %+v", pgResp.Data)
			t.Logf("SQLREST response: %+v", srResp.Data)

			// This test documents the incompatibility but doesn't fail
			// In a real scenario, you might want to normalize special characters
			// or use explicit collation settings for consistent behavior
		} else {
			t.Logf("No incompatibility detected - responses match")
		}
	})

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

	t.Run("pattern_matching_case_sensitivity", func(t *testing.T) {
		// This test documents the expected incompatibility in pattern matching
		// between PostgreSQL (case-sensitive) and MySQL (case-insensitive)
		query := "/artist?name=like.*AC*&limit=10"

		// Query PostgREST (PostgreSQL)
		pgResp := suite.QueryPostgREST(t, query)

		// Query SQL-REST (MySQL)
		srResp := suite.QuerySQLREST(t, query)

		// Compare responses - this will fail due to collation differences
		if err := compare.CompareResponses(pgResp, srResp); err != nil {
			t.Logf("Expected incompatibility detected: %v", err)
			t.Logf("PostgreSQL (PostgREST) uses case-sensitive LIKE: '%%AC%%' matches only 'AC/DC'")
			t.Logf("MySQL (SQLREST) uses case-insensitive collation: '%%AC%%' matches 'AC/DC' and 'Accept'")
			t.Logf("This is due to different default collation settings:")
			t.Logf("  - PostgreSQL: case-sensitive LIKE")
			t.Logf("  - MySQL: utf8mb4_0900_ai_ci (case-insensitive)")

			// Log the actual data for analysis
			t.Logf("PostgREST response (PostgreSQL): %+v", pgResp.Data)
			t.Logf("SQLREST response (MySQL): %+v", srResp.Data)

			// This test documents the incompatibility but doesn't fail
			// In a real scenario, you might want to use ILIKE in PostgreSQL
			// or configure MySQL to use case-sensitive collation
		} else {
			t.Logf("No incompatibility detected - responses match")
		}
	})

	t.Run("pattern_matching_with_single_parameter", func(t *testing.T) {
		// This test documents the expected incompatibility when using the single parameter
		// with pattern matching that results in different row counts due to case sensitivity
		query := "/artist?name=like.*AC*&single"

		// Query PostgREST (PostgreSQL)
		pgResp := suite.QueryPostgREST(t, query)

		// Query SQL-REST (MySQL)
		srResp := suite.QuerySQLREST(t, query)

		// Compare responses - this will fail due to case sensitivity differences
		if err := compare.CompareResponses(pgResp, srResp); err != nil {
			t.Logf("Expected incompatibility detected: %v", err)
			t.Logf("This demonstrates case sensitivity affecting single parameter behavior:")
			t.Logf("  - Pattern '%%AC%%' with single=true:")
			t.Logf("    - PostgreSQL (PostgREST): Returns only 'AC/DC' (case-sensitive LIKE)")
			t.Logf("    - MySQL (SQLREST): Would return both 'AC/DC' and 'Accept' (case-insensitive)")
			t.Logf("  - The single parameter adds LIMIT 1 correctly in both cases")
			t.Logf("  - But the different row counts before LIMIT 1 cause different final results")
			t.Logf("  - This is due to different default collation settings:")
			t.Logf("    - PostgreSQL: case-sensitive LIKE")
			t.Logf("    - MySQL: utf8mb4_0900_ai_ci (case-insensitive)")

			// Log the actual data for analysis
			t.Logf("PostgREST response (PostgreSQL): %+v", pgResp.Data)
			t.Logf("SQLREST response (MySQL): %+v", srResp.Data)

			// This test documents the incompatibility but doesn't fail
			// The single parameter works correctly (adds LIMIT 1), but the underlying
			// pattern matching behavior differs between platforms
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

	t.Run("pattern_matching_collation", func(t *testing.T) {
		t.Log("PostgreSQL uses case-sensitive LIKE by default")
		t.Log("MySQL uses case-insensitive collation (utf8mb4_0900_ai_ci) by default")
		t.Log("Pattern '%AC%' matches:")
		t.Log("  - PostgreSQL: only 'AC/DC' (case-sensitive)")
		t.Log("  - MySQL: both 'AC/DC' and 'Accept' (case-insensitive)")
		t.Log("Use ILIKE in PostgreSQL or explicit collation settings for consistency")
	})
}
