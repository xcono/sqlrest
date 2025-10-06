package e2e

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
	}

	suite.RunTestCases(t, testCases)
}
