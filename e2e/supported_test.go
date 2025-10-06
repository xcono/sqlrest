package e2e_test

import (
	"testing"
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
		// Pattern Matching Tests - Case-sensitive and case-insensitive LIKE operations
		{
			Name:        "pattern_matching_like_start_with",
			Query:       "/artist?name=like.A*&limit=10",
			Description: "Pattern matching with LIKE - artists starting with 'A'",
		},
		{
			Name:        "pattern_matching_like_end_with",
			Query:       "/artist?name=like.*DC&limit=10",
			Description: "Pattern matching with LIKE - artists ending with 'DC'",
		},
		{
			Name:        "pattern_matching_ilike_case_insensitive",
			Query:       "/artist?name=ilike.*ac*&limit=10",
			Description: "Pattern matching with ILIKE (case-insensitive) - artists with 'ac' in name",
		},
		{
			Name:        "pattern_matching_ilike_start_with_case_insensitive",
			Query:       "/artist?name=ilike.a*&limit=10",
			Description: "Pattern matching with ILIKE - artists starting with 'a' (case-insensitive)",
		},
		// Null Operations Tests - IS NULL and IS NOT NULL
		{
			Name:        "null_operations_is_null",
			Query:       "/track?name=is.null&limit=10",
			Description: "Null operations - tracks with null names",
		},
		{
			Name:        "null_operations_is_not_null",
			Query:       "/track?name=not.is.null&limit=10",
			Description: "Null operations - tracks with non-null names",
		},
		{
			Name:        "null_operations_is_null_artist",
			Query:       "/artist?name=is.null&limit=10",
			Description: "Null operations - artists with null names",
		},
		{
			Name:        "null_operations_is_not_null_artist",
			Query:       "/artist?name=not.is.null&limit=10",
			Description: "Null operations - artists with non-null names",
		},
		// Logical Operators Tests - AND, OR combinations
		{
			Name:        "logical_operators_and_implicit",
			Query:       "/album?album_id=gt.2&album_id=lt.10&limit=10",
			Description: "Logical operators - implicit AND: album_id > 2 AND < 10",
		},
		{
			Name:        "logical_operators_or_simple",
			Query:       "/album?or=(album_id.eq.1,album_id.eq.5)&limit=10",
			Description: "Logical operators - OR: album_id = 1 OR album_id = 5",
		},
		{
			Name:        "logical_operators_or_complex",
			Query:       "/track?or=(track_id.lt.5,track_id.gt.15)&limit=10",
			Description: "Logical operators - OR: track_id < 5 OR track_id > 15",
		},
		{
			Name:        "logical_operators_and_explicit",
			Query:       "/track?and=(track_id.gt.5,track_id.lt.15)&limit=10",
			Description: "Logical operators - explicit AND: track_id > 5 AND < 15",
		},
		{
			Name:        "logical_operators_or_with_filters",
			Query:       "/track?or=(genre_id.eq.1,genre_id.eq.2)&album_id=gt.2&limit=10",
			Description: "Logical operators - OR with additional filter: (genre_id=1 OR genre_id=2) AND album_id>2",
		},
		// Single Row Tests - single parameter
		{
			Name:        "single_row_artist",
			Query:       "/artist?artist_id=eq.1&single",
			Description: "Single row - get single artist by ID",
		},
		{
			Name:        "single_row_album",
			Query:       "/album?album_id=eq.1&single",
			Description: "Single row - get single album by ID",
		},
		{
			Name:        "single_row_track",
			Query:       "/track?track_id=eq.1&single",
			Description: "Single row - get single track by ID",
		},
		{
			Name:        "single_row_with_select",
			Query:       "/artist?artist_id=eq.1&select=artist_id,name&single",
			Description: "Single row with column selection - get single artist with specific columns",
		},
		// Additional Comparison Operators Tests - gte, lte, neq
		{
			Name:        "comparison_gte_greater_than_equal",
			Query:       "/album?album_id=gte.5&limit=10",
			Description: "Comparison operators - greater than or equal: album_id >= 5",
		},
		{
			Name:        "comparison_lte_less_than_equal",
			Query:       "/album?album_id=lte.5&limit=10",
			Description: "Comparison operators - less than or equal: album_id <= 5",
		},
		{
			Name:        "comparison_neq_not_equal",
			Query:       "/album?album_id=neq.1&limit=10",
			Description: "Comparison operators - not equal: album_id != 1",
		},
		{
			Name:        "comparison_gte_with_order",
			Query:       "/track?track_id=gte.10&order=track_id&limit=10",
			Description: "Comparison operators - gte with ordering: track_id >= 10 ordered by track_id",
		},
		{
			Name:        "comparison_lte_with_order",
			Query:       "/track?track_id=lte.10&order=track_id.desc&limit=10",
			Description: "Comparison operators - lte with ordering: track_id <= 10 ordered by track_id desc",
		},
		// Complex Combined Operations Tests
		{
			Name:        "complex_pattern_and_null",
			Query:       "/artist?name=like.A*&name=not.is.null&limit=10",
			Description: "Complex query - pattern matching AND null check: artists starting with 'A' AND name not null",
		},
		{
			Name:        "complex_logical_and_comparison",
			Query:       "/track?and=(track_id.gte.5,track_id.lte.15)&or=(genre_id.eq.1,genre_id.eq.2)&limit=10",
			Description: "Complex query - logical AND with OR: track_id between 5-15 AND (genre_id=1 OR genre_id=2)",
		},
		{
			Name:        "single_row_pattern_matching_genres",
			Query:       "/genre?name=like.*Rock*&single",
			Description: "Single row with pattern matching on genres (no special chars) - verify single parameter works",
		},
		{
			Name:        "complex_null_and_comparison",
			Query:       "/album?album_id=gte.3&album_id=lte.7&title=not.is.null&limit=10",
			Description: "Complex query - range filter with null check: album_id 3-7 AND title not null",
		},
	}

	suite.RunTestCases(t, testCases)
}
