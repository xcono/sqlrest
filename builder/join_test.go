package builder_test

import (
	"net/url"
	"testing"

	"github.com/xcono/sqlrest/builder"
)

// TestEmbedDefinition tests the EmbedDefinition data structure
func TestEmbedDefinition(t *testing.T) {
	t.Run("basic_embed_definition", func(t *testing.T) {
		embed := builder.EmbedDefinition{
			Table:       "posts",
			Columns:     []string{"id", "title", "content"},
			JoinType:    builder.JoinTypeInner,
			OnCondition: "users.id = posts.user_id",
		}

		if embed.Table != "posts" {
			t.Errorf("Expected table 'posts', got '%s'", embed.Table)
		}

		if len(embed.Columns) != 3 {
			t.Errorf("Expected 3 columns, got %d", len(embed.Columns))
		}

		if embed.JoinType != builder.JoinTypeInner {
			t.Errorf("Expected JoinTypeInner, got %s", embed.JoinType)
		}
	})

	t.Run("nested_embed_definition", func(t *testing.T) {
		nestedEmbed := builder.EmbedDefinition{
			Table:    "comments",
			Columns:  []string{"id", "text"},
			JoinType: builder.JoinTypeLeft,
		}

		embed := builder.EmbedDefinition{
			Table:        "posts",
			Columns:      []string{"id", "title"},
			JoinType:     builder.JoinTypeInner,
			NestedEmbeds: []builder.EmbedDefinition{nestedEmbed},
		}

		if len(embed.NestedEmbeds) != 1 {
			t.Errorf("Expected 1 nested embed, got %d", len(embed.NestedEmbeds))
		}

		if embed.NestedEmbeds[0].Table != "comments" {
			t.Errorf("Expected nested table 'comments', got '%s'", embed.NestedEmbeds[0].Table)
		}
	})

	t.Run("validation", func(t *testing.T) {
		// Valid embed
		validEmbed := builder.EmbedDefinition{
			Table:       "posts",
			OnCondition: "users.id = posts.user_id",
		}
		if err := validEmbed.ValidateEmbedDefinition(); err != nil {
			t.Errorf("Valid embed should not have validation error: %v", err)
		}

		// Invalid embed - missing table
		invalidEmbed := builder.EmbedDefinition{
			OnCondition: "users.id = posts.user_id",
		}
		if err := invalidEmbed.ValidateEmbedDefinition(); err == nil {
			t.Error("Invalid embed should have validation error")
		}

		// Invalid embed - missing ON condition
		invalidEmbed2 := builder.EmbedDefinition{
			Table: "posts",
		}
		if err := invalidEmbed2.ValidateEmbedDefinition(); err == nil {
			t.Error("Invalid embed should have validation error")
		}
	})
}

// TestJoinAliasManager tests the JoinAliasManager
func TestJoinAliasManager(t *testing.T) {
	jam := builder.NewJoinAliasManager()

	t.Run("get_alias", func(t *testing.T) {
		alias1 := jam.GetAlias("users")
		alias2 := jam.GetAlias("posts")

		if alias1 == alias2 {
			t.Error("Different tables should have different aliases")
		}

		// Getting alias for same table should return same alias
		alias1Again := jam.GetAlias("users")
		if alias1 != alias1Again {
			t.Error("Same table should return same alias")
		}
	})

	t.Run("get_alias_for_table", func(t *testing.T) {
		jam.GetAlias("comments")

		alias, exists := jam.GetAliasForTable("comments")
		if !exists {
			t.Error("Should find alias for existing table")
		}
		if alias == "" {
			t.Error("Alias should not be empty")
		}

		_, exists = jam.GetAliasForTable("nonexistent")
		if exists {
			t.Error("Should not find alias for non-existent table")
		}
	})

	t.Run("get_all_aliases", func(t *testing.T) {
		jam.GetAlias("table1")
		jam.GetAlias("table2")

		aliases := jam.GetAllAliases()
		if len(aliases) < 2 {
			t.Errorf("Expected at least 2 aliases, got %d", len(aliases))
		}
	})
}

// TestEmbedParser tests the EmbedParser
func TestEmbedParser(t *testing.T) {
	parser := builder.NewEmbedParser(nil)

	t.Run("parse_simple_embed", func(t *testing.T) {
		embed, err := parser.ParseEmbedSyntax("posts", "users")
		if err != nil {
			t.Fatalf("Failed to parse simple embed: %v", err)
		}

		if embed.Table != "posts" {
			t.Errorf("Expected table 'posts', got '%s'", embed.Table)
		}

		if embed.JoinType != builder.JoinTypeLeft {
			t.Errorf("Expected JoinTypeLeft (default), got %s", embed.JoinType)
		}
	})

	t.Run("parse_inner_join", func(t *testing.T) {
		embed, err := parser.ParseEmbedSyntax("posts!inner", "users")
		if err != nil {
			t.Fatalf("Failed to parse inner join: %v", err)
		}

		if embed.Table != "posts" {
			t.Errorf("Expected table 'posts', got '%s'", embed.Table)
		}

		if embed.JoinType != builder.JoinTypeInner {
			t.Errorf("Expected JoinTypeInner, got %s", embed.JoinType)
		}
	})

	t.Run("parse_embed_with_columns", func(t *testing.T) {
		embed, err := parser.ParseEmbedSyntax("posts(id,title)", "users")
		if err != nil {
			t.Fatalf("Failed to parse embed with columns: %v", err)
		}

		if len(embed.Columns) != 2 {
			t.Errorf("Expected 2 columns, got %d", len(embed.Columns))
		}

		if embed.Columns[0] != "id" || embed.Columns[1] != "title" {
			t.Errorf("Expected columns ['id', 'title'], got %v", embed.Columns)
		}
	})

	t.Run("parse_nested_embed", func(t *testing.T) {
		embed, err := parser.ParseEmbedSyntax("posts(id,comments(text))", "users")
		if err != nil {
			t.Fatalf("Failed to parse nested embed: %v", err)
		}

		t.Logf("Parsed embed: %+v", embed)
		t.Logf("Nested embeds count: %d", len(embed.NestedEmbeds))

		if len(embed.NestedEmbeds) != 1 {
			t.Errorf("Expected 1 nested embed, got %d", len(embed.NestedEmbeds))
			return
		}

		nested := embed.NestedEmbeds[0]
		if nested.Table != "comments" {
			t.Errorf("Expected nested table 'comments', got '%s'", nested.Table)
		}

		if len(nested.Columns) != 1 || nested.Columns[0] != "text" {
			t.Errorf("Expected nested columns ['text'], got %v", nested.Columns)
		}
	})

	t.Run("parse_complex_embed", func(t *testing.T) {
		embed, err := parser.ParseEmbedSyntax("posts!inner(id,title,comments!left(text,author(name)))", "users")
		if err != nil {
			t.Fatalf("Failed to parse complex embed: %v", err)
		}

		if embed.JoinType != builder.JoinTypeInner {
			t.Errorf("Expected JoinTypeInner, got %s", embed.JoinType)
		}

		if len(embed.Columns) != 2 {
			t.Errorf("Expected 2 columns, got %d", len(embed.Columns))
		}

		if len(embed.NestedEmbeds) != 1 {
			t.Errorf("Expected 1 nested embed, got %d", len(embed.NestedEmbeds))
		}

		nested := embed.NestedEmbeds[0]
		if nested.JoinType != builder.JoinTypeLeft {
			t.Errorf("Expected nested JoinTypeLeft, got %s", nested.JoinType)
		}

		if len(nested.NestedEmbeds) != 1 {
			t.Errorf("Expected 1 nested-nested embed, got %d", len(nested.NestedEmbeds))
		}
	})
}

// TestPostgRESTBuilderEmbedParsing tests the enhanced PostgRESTBuilder
func TestPostgRESTBuilderEmbedParsing(t *testing.T) {
	b := builder.NewPostgRESTBuilder()

	t.Run("parse_select_with_embeds", func(t *testing.T) {
		columns, embeds := b.ParseSelectWithEmbeds("name,email,posts!inner(id,title)", "users")

		if len(columns) != 2 {
			t.Errorf("Expected 2 columns, got %d", len(columns))
		}

		if columns[0] != "name" || columns[1] != "email" {
			t.Errorf("Expected columns ['name', 'email'], got %v", columns)
		}

		if len(embeds) != 1 {
			t.Errorf("Expected 1 embed, got %d", len(embeds))
		}

		embed := embeds[0]
		if embed.Table != "posts" {
			t.Errorf("Expected embed table 'posts', got '%s'", embed.Table)
		}

		if embed.JoinType != builder.JoinTypeInner {
			t.Errorf("Expected JoinTypeInner, got %s", embed.JoinType)
		}
	})

	t.Run("parse_url_params_with_embeds", func(t *testing.T) {
		params := url.Values{
			"select": []string{"name,posts!inner(id,title)"},
		}

		query, err := b.ParseURLParams("users", params)
		if err != nil {
			t.Fatalf("Failed to parse URL params: %v", err)
		}

		if len(query.Select) != 1 {
			t.Errorf("Expected 1 select column, got %d", len(query.Select))
		}

		if len(query.Embeds) != 1 {
			t.Errorf("Expected 1 embed, got %d", len(query.Embeds))
		}

		embed := query.Embeds[0]
		if embed.Table != "posts" {
			t.Errorf("Expected embed table 'posts', got '%s'", embed.Table)
		}
	})

	t.Run("backward_compatibility", func(t *testing.T) {
		params := url.Values{
			"embed": []string{"posts,comments"},
		}

		query, err := b.ParseURLParams("users", params)
		if err != nil {
			t.Fatalf("Failed to parse URL params: %v", err)
		}

		if len(query.Embeds) != 2 {
			t.Errorf("Expected 2 embeds, got %d", len(query.Embeds))
		}

		// Check that embeds are created with default values
		for _, embed := range query.Embeds {
			if embed.JoinType != builder.JoinTypeLeft {
				t.Errorf("Expected default JoinTypeLeft, got %s", embed.JoinType)
			}

			if len(embed.Columns) != 1 || embed.Columns[0] != "*" {
				t.Errorf("Expected default columns ['*'], got %v", embed.Columns)
			}
		}
	})
}

// TestJoinTypeConversion tests JOIN type conversion to sqlbuilder
func TestJoinTypeConversion(t *testing.T) {
	testCases := []struct {
		joinType    builder.JoinType
		expectedSQL string
	}{
		{builder.JoinTypeInner, "INNER"},
		{builder.JoinTypeLeft, "LEFT"},
		{builder.JoinTypeRight, "RIGHT"},
		{builder.JoinTypeFull, "FULL"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.joinType), func(t *testing.T) {
			sqlOption := tc.joinType.ToSQLJoinOption()
			if string(sqlOption) != tc.expectedSQL {
				t.Errorf("Expected SQL option '%s', got '%s'", tc.expectedSQL, string(sqlOption))
			}
		})
	}
}

// TestEmbedDefinitionMethods tests utility methods
func TestEmbedDefinitionMethods(t *testing.T) {
	embed := builder.EmbedDefinition{
		Table:   "posts",
		Columns: []string{"id", "title"},
		Alias:   "p",
		NestedEmbeds: []builder.EmbedDefinition{
			{
				Table:   "comments",
				Columns: []string{"text"},
				Alias:   "c",
			},
		},
	}

	t.Run("get_column_names", func(t *testing.T) {
		columns := embed.GetColumnNames()
		expected := []string{"p.id", "p.title", "c.text"}

		if len(columns) != len(expected) {
			t.Errorf("Expected %d columns, got %d", len(expected), len(columns))
		}

		for i, col := range columns {
			if col != expected[i] {
				t.Errorf("Expected column '%s', got '%s'", expected[i], col)
			}
		}
	})

	t.Run("get_join_tables", func(t *testing.T) {
		tables := embed.GetJoinTables()
		expected := []string{"posts", "comments"}

		if len(tables) != len(expected) {
			t.Errorf("Expected %d tables, got %d", len(expected), len(tables))
		}

		for i, table := range tables {
			if table != expected[i] {
				t.Errorf("Expected table '%s', got '%s'", expected[i], table)
			}
		}
	})
}
