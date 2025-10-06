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
			Name:        "order_asc",
			Query:       "/artist?order=name&limit=10",
			Description: "Ascending ordering",
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
	}

	suite.RunTestCases(t, testCases)
}
