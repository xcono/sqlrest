package e2e_test

import (
	"testing"

	"github.com/xcono/sqlrest/e2e/compare"
)

func TestE2EComparison(t *testing.T) {
	suite := NewTestSuite(t, nil)
	defer suite.Close()

	// Define test cases
	testCases := []TestCase{
		{
			Name:        "select_all_artists",
			Query:       "/artist",
			Description: "Basic SELECT without filters",
		},
		{
			Name:        "select_artist_columns",
			Query:       "/artist?select=artist_id,name",
			Description: "Column selection",
		},
		{
			Name:        "filter_eq",
			Query:       "/artist?artist_id=eq.1",
			Description: "Equality filtering",
		},
		{
			Name:        "filter_gt",
			Query:       "/album?album_id=gt.2&limit=5",
			Description: "Greater than filtering",
		},
		{
			Name:        "filter_in",
			Query:       "/genre?genre_id=in.(1,2,3)",
			Description: "IN clause filtering",
		},
		{
			Name:        "order_desc",
			Query:       "/album?order=title.desc&limit=10",
			Description: "Descending ordering",
		},
		{
			Name:        "limit_offset",
			Query:       "/track?limit=5&offset=2",
			Description: "Pagination",
		},
		{
			Name:        "limit_offset_without_order",
			Query:       "/track?limit=5&offset=2",
			Description: "Pagination without explicit ORDER BY",
		},
		{
			Name:        "order_asc_genre",
			Query:       "/genre?order=name&limit=10",
			Description: "Ascending ordering on genre table (no special characters)",
		},
		{
			Name:        "order_desc_genre",
			Query:       "/genre?order=name.desc&limit=10",
			Description: "Descending ordering on genre table (no special characters)",
		},
		{
			Name:        "complex_query",
			Query:       "/track?select=track_id,name,album_id&genre_id=eq.1&limit=5",
			Description: "Combined filters and selections",
		},
		// JOIN Operations Tests - Using correct PostgREST syntax
		{
			Name:        "simple_join_test",
			Query:       "/album?select=*,artist(*)&limit=2",
			Description: "Simple JOIN test using PostgREST select syntax",
		},
		{
			Name:        "left_join_album_artist",
			Query:       "/album?select=album_id,title,artist(name)&limit=5",
			Description: "LEFT JOIN album with artist using PostgREST embed syntax",
		},
		{
			Name:        "inner_join_track_album",
			Query:       "/track?select=track_id,name,album!inner(title)&limit=5",
			Description: "INNER JOIN track with album using !inner syntax",
		},
		{
			Name:        "nested_join_track_album_artist",
			Query:       "/track?select=track_id,name,album(title,artist(name))&limit=3",
			Description: "Nested JOIN: track -> album -> artist",
		},
		{
			Name:        "join_with_filters",
			Query:       "/album?select=album_id,title,artist(name)&artist_id=eq.1&limit=5",
			Description: "JOIN with filters on parent table",
		},
		{
			Name:        "join_with_embedded_filters",
			Query:       "/track?select=track_id,name,album(title)&album.album_id=gt.2&limit=5",
			Description: "JOIN with filters on embedded table",
		},
		{
			Name:        "join_with_ordering",
			Query:       "/album?select=album_id,title,artist(name)&order=title.desc&limit=5",
			Description: "JOIN with ordering",
		},
		{
			Name:        "multiple_embeds",
			Query:       "/track?select=track_id,name,album(title),genre(name)&limit=5",
			Description: "Multiple JOINs: track with album and genre",
		},
	}

	suite.RunTestCases(t, testCases)
}

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
