package builder

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/huandu/go-sqlbuilder"
)

// JoinType represents the type of JOIN operation
type JoinType string

const (
	JoinTypeInner JoinType = "inner"
	JoinTypeLeft  JoinType = "left"
	JoinTypeRight JoinType = "right"
	JoinTypeFull  JoinType = "full"
)

// EmbedDefinition represents a PostgREST embed relationship
type EmbedDefinition struct {
	Table        string            `json:"table"`         // Target table name
	Columns      []string          `json:"columns"`       // Selected columns from embedded table
	JoinType     JoinType          `json:"join_type"`     // Type of JOIN (inner, left, right, full)
	Filters      []Filter          `json:"filters"`       // Filters applied to embedded table
	NestedEmbeds []EmbedDefinition `json:"nested_embeds"` // Recursive embedding for nested relationships
	Alias        string            `json:"alias"`         // Table alias for JOIN
	OnCondition  string            `json:"on_condition"`  // JOIN condition (e.g., "users.id = posts.user_id")
}

// JoinAliasManager manages table aliases for JOIN operations
type JoinAliasManager struct {
	aliases map[string]string // table -> alias mapping
	counter int               // counter for generating unique aliases
}

// NewJoinAliasManager creates a new alias manager
func NewJoinAliasManager() *JoinAliasManager {
	return &JoinAliasManager{
		aliases: make(map[string]string),
		counter: 0,
	}
}

// GetAlias returns an alias for the given table, creating one if it doesn't exist
func (jam *JoinAliasManager) GetAlias(table string) string {
	if alias, exists := jam.aliases[table]; exists {
		return alias
	}

	// Generate new alias
	jam.counter++
	alias := "t" + fmt.Sprint(jam.counter)
	jam.aliases[table] = alias
	return alias
}

// GetAliasForTable returns the alias for a specific table
func (jam *JoinAliasManager) GetAliasForTable(table string) (string, bool) {
	alias, exists := jam.aliases[table]
	return alias, exists
}

// GetAllAliases returns all table-alias mappings
func (jam *JoinAliasManager) GetAllAliases() map[string]string {
	result := make(map[string]string)
	for table, alias := range jam.aliases {
		result[table] = alias
	}
	return result
}

// ForeignKeyResolver handles automatic foreign key relationship detection
type ForeignKeyResolver struct {
	db *sql.DB
}

// NewForeignKeyResolver creates a new foreign key resolver
func NewForeignKeyResolver(db *sql.DB) *ForeignKeyResolver {
	return &ForeignKeyResolver{db: db}
}

// DetectRelationship attempts to detect the relationship between two tables
func (fkr *ForeignKeyResolver) DetectRelationship(parentTable, childTable string) (string, error) {
	// Try common naming conventions
	possibleKeys := []string{
		parentTable + "_id",
		parentTable + "Id",
		"id",
	}

	// Check if any of these columns exist in the child table
	for _, key := range possibleKeys {
		if fkr.columnExists(childTable, key) {
			return parentTable + "." + "id" + " = " + childTable + "." + key, nil
		}
	}

	return "", fmt.Errorf("no foreign key relationship found between %s and %s", parentTable, childTable)
}

// columnExists checks if a column exists in a table
func (fkr *ForeignKeyResolver) columnExists(table, column string) bool {
	query := "SELECT COUNT(*) FROM information_schema.columns WHERE table_name = ? AND column_name = ?"
	var count int
	err := fkr.db.QueryRow(query, table, column).Scan(&count)
	return err == nil && count > 0
}

// EmbedParser handles parsing of PostgREST embed syntax
type EmbedParser struct {
	fkResolver *ForeignKeyResolver
}

// NewEmbedParser creates a new embed parser
func NewEmbedParser(fkResolver *ForeignKeyResolver) *EmbedParser {
	return &EmbedParser{fkResolver: fkResolver}
}

// ParseEmbedSyntax parses PostgREST embed syntax into EmbedDefinition
// Examples:
//   - "posts" -> EmbedDefinition{Table: "posts", JoinType: "left"}
//   - "posts!inner" -> EmbedDefinition{Table: "posts", JoinType: "inner"}
//   - "posts(text,author)" -> EmbedDefinition{Table: "posts", Columns: ["text"], NestedEmbeds: [...]}
func (ep *EmbedParser) ParseEmbedSyntax(embedStr string, parentTable string) (*EmbedDefinition, error) {
	// Find the first opening parenthesis
	openParen := strings.Index(embedStr, "(")
	if openParen == -1 {
		// No parentheses, treat as simple table
		tableName, joinType := ep.ParseTableAndJoinType(embedStr)
		embed := &EmbedDefinition{
			Table:    tableName,
			JoinType: joinType,
			Columns:  []string{"*"}, // Default to all columns
		}

		// Detect foreign key relationship
		if ep.fkResolver != nil {
			onCondition, err := ep.fkResolver.DetectRelationship(parentTable, tableName)
			if err != nil {
				return nil, fmt.Errorf("failed to detect relationship between %s and %s: %w", parentTable, tableName, err)
			}
			embed.OnCondition = onCondition
		}

		return embed, nil
	}

	// Split at the first opening parenthesis
	tablePart := embedStr[:openParen]
	contentPart := embedStr[openParen+1:]

	// Remove the closing parenthesis
	if !strings.HasSuffix(contentPart, ")") {
		return nil, fmt.Errorf("missing closing parenthesis in embed: %s", embedStr)
	}
	contentPart = strings.TrimSuffix(contentPart, ")")

	// Parse table name and join type
	tableName, joinType := ep.ParseTableAndJoinType(tablePart)

	embed := &EmbedDefinition{
		Table:    tableName,
		JoinType: joinType,
		Columns:  []string{"*"}, // Default to all columns
	}

	// Parse columns and nested embeds if present
	if contentPart != "" {
		columns, nestedEmbeds, err := ep.ParseEmbedContent(contentPart, tableName)
		if err != nil {
			return nil, err
		}

		if len(columns) > 0 {
			embed.Columns = columns
		}
		embed.NestedEmbeds = nestedEmbeds
	}

	// Detect foreign key relationship
	if ep.fkResolver != nil {
		onCondition, err := ep.fkResolver.DetectRelationship(parentTable, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to detect relationship between %s and %s: %w", parentTable, tableName, err)
		}
		embed.OnCondition = onCondition
	}

	return embed, nil
}

// ParseTableAndJoinType parses table name and join type from embed string
func (ep *EmbedParser) ParseTableAndJoinType(tablePart string) (string, JoinType) {
	// Check for join type modifiers
	if strings.Contains(tablePart, "!inner") {
		return strings.Replace(tablePart, "!inner", "", 1), JoinTypeInner
	}
	if strings.Contains(tablePart, "!left") {
		return strings.Replace(tablePart, "!left", "", 1), JoinTypeLeft
	}
	if strings.Contains(tablePart, "!right") {
		return strings.Replace(tablePart, "!right", "", 1), JoinTypeRight
	}
	if strings.Contains(tablePart, "!full") {
		return strings.Replace(tablePart, "!full", "", 1), JoinTypeFull
	}

	// Default to left join (PostgREST default)
	return tablePart, JoinTypeLeft
}

// ParseEmbedContent parses the content inside parentheses
func (ep *EmbedParser) ParseEmbedContent(content string, parentTable string) ([]string, []EmbedDefinition, error) {
	var columns []string
	var nestedEmbeds []EmbedDefinition

	// Split by comma, but be careful with nested parentheses
	parts := ep.SplitEmbedContent(content)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check if this is a nested embed (contains parentheses)
		if strings.Contains(part, "(") {
			nestedEmbed, err := ep.ParseEmbedSyntax(part, parentTable)
			if err != nil {
				return nil, nil, err
			}
			nestedEmbeds = append(nestedEmbeds, *nestedEmbed)
		} else {
			// It's a column
			columns = append(columns, part)
		}
	}

	return columns, nestedEmbeds, nil
}

// SplitEmbedContent splits embed content by comma, handling nested parentheses
func (ep *EmbedParser) SplitEmbedContent(content string) []string {
	var parts []string
	var current strings.Builder
	depth := 0

	for _, char := range content {
		switch char {
		case '(':
			depth++
			current.WriteRune(char)
		case ')':
			depth--
			current.WriteRune(char)
		case ',':
			if depth == 0 {
				parts = append(parts, current.String())
				current.Reset()
			} else {
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// ToSQLJoinOption converts JoinType to sqlbuilder.JoinOption
func (jt JoinType) ToSQLJoinOption() sqlbuilder.JoinOption {
	switch jt {
	case JoinTypeInner:
		return sqlbuilder.InnerJoin
	case JoinTypeLeft:
		return sqlbuilder.LeftJoin
	case JoinTypeRight:
		return sqlbuilder.RightJoin
	case JoinTypeFull:
		return sqlbuilder.FullJoin
	default:
		return sqlbuilder.LeftJoin // Default to left join
	}
}

// ValidateEmbedDefinition validates an embed definition
func (ed *EmbedDefinition) ValidateEmbedDefinition() error {
	if ed.Table == "" {
		return fmt.Errorf("table name is required for embed definition")
	}

	if ed.OnCondition == "" {
		return fmt.Errorf("JOIN condition is required for embed definition")
	}

	// Validate nested embeds recursively
	for i, nested := range ed.NestedEmbeds {
		if err := nested.ValidateEmbedDefinition(); err != nil {
			return fmt.Errorf("nested embed %d validation failed: %w", i, err)
		}
	}

	return nil
}

// GetColumnNames returns all column names including nested ones
func (ed *EmbedDefinition) GetColumnNames() []string {
	var columns []string

	// Add columns from this embed
	for _, col := range ed.Columns {
		if col != "*" {
			columns = append(columns, fmt.Sprintf("%s.%s", ed.Alias, col))
		}
	}

	// Add columns from nested embeds
	for _, nested := range ed.NestedEmbeds {
		columns = append(columns, nested.GetColumnNames()...)
	}

	return columns
}

// GetJoinTables returns all tables involved in this embed and its nested embeds
func (ed *EmbedDefinition) GetJoinTables() []string {
	var tables []string
	tables = append(tables, ed.Table)

	for _, nested := range ed.NestedEmbeds {
		tables = append(tables, nested.GetJoinTables()...)
	}

	return tables
}
