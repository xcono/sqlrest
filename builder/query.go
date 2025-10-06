package builder

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/huandu/go-sqlbuilder"
)

// PostgREST operators (lightweight implementation focusing on core functionality)
const (
	OpEQ    = "eq"    // equals
	OpNEQ   = "neq"   // not equals
	OpGT    = "gt"    // greater than
	OpGTE   = "gte"   // greater than or equal
	OpLT    = "lt"    // less than
	OpLTE   = "lte"   // less than or equal
	OpLike  = "like"  // case-sensitive pattern matching
	OpILike = "ilike" // case-insensitive pattern matching
	OpIn    = "in"    // in array
	OpIs    = "is"    // is null/not null
	OpNot   = "not"   // not operator
)

// Logical operators
const (
	LogAnd = "and"
	LogOr  = "or"
)

// Filter represents a single filter condition
type Filter struct {
	Column   string      `json:"column"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// LogicalFilter represents a logical combination of filters
type LogicalFilter struct {
	Operator string        `json:"operator"` // "and" or "or"
	Filters  []interface{} `json:"filters"`  // Can contain Filter or LogicalFilter
}

// PostgRESTQuery represents a PostgREST-compatible query
type PostgRESTQuery struct {
	Table   string            `json:"table"`
	Select  []string          `json:"select,omitempty"`
	Filters []interface{}     `json:"filters,omitempty"` // Can contain Filter or LogicalFilter
	Order   []string          `json:"order,omitempty"`
	Limit   int               `json:"limit,omitempty"`
	Offset  int               `json:"offset,omitempty"`
	Embeds  []EmbedDefinition `json:"embeds,omitempty"`  // PostgREST resource embedding with JOIN support
	Headers map[string]string `json:"headers,omitempty"` // HTTP headers
}

// PostgRESTBuilder builds PostgREST-compatible queries
type PostgRESTBuilder struct{}

// NewPostgRESTBuilder creates a new PostgREST query builder
func NewPostgRESTBuilder() *PostgRESTBuilder {
	return &PostgRESTBuilder{}
}

// ParseURLParams parses PostgREST URL query parameters into PostgRESTQuery
func (b *PostgRESTBuilder) ParseURLParams(table string, params url.Values) (*PostgRESTQuery, error) {
	query := &PostgRESTQuery{
		Table:   table,
		Filters: []interface{}{},
	}

	// Parse select columns and embeds
	if selectParam := params.Get("select"); selectParam != "" {
		query.Select, query.Embeds = b.ParseSelectWithEmbeds(selectParam, table)
	}

	// Parse filters
	for key, values := range params {
		if len(values) == 0 || values[0] == "" {
			continue
		}

		switch key {
		case "select", "order", "limit", "offset", "embed", "single", "maybeSingle", "returning", "count":
			continue // Handle these separately
		default:
			filter, err := b.parseFilterParam(key, values[0])
			if err != nil {
				return nil, fmt.Errorf("failed to parse filter %s: %w", key, err)
			}
			if filter != nil {
				query.Filters = append(query.Filters, filter)
			}
		}
	}

	// Parse order
	if orderParam := params.Get("order"); orderParam != "" {
		orderParts := strings.Split(orderParam, ",")
		for i, part := range orderParts {
			part = strings.TrimSpace(part)
			if strings.Contains(part, ".") {
				// Handle PostgREST order syntax: column.desc or column.asc
				parts := strings.Split(part, ".")
				if len(parts) == 2 {
					column := parts[0]
					direction := strings.ToUpper(parts[1])
					if direction == "DESC" || direction == "ASC" {
						orderParts[i] = column + " " + direction
					}
				}
			}
		}
		query.Order = orderParts
	}

	// Parse limit
	if limitParam := params.Get("limit"); limitParam != "" {
		if limit, err := strconv.Atoi(limitParam); err == nil {
			query.Limit = limit
		}
	}

	// Parse offset
	if offsetParam := params.Get("offset"); offsetParam != "" {
		if offset, err := strconv.Atoi(offsetParam); err == nil {
			query.Offset = offset
		}
	}

	// Parse embed parameter (legacy support)
	if embedParam := params.Get("embed"); embedParam != "" {
		// For backward compatibility, parse simple embed parameter
		embedStrings := strings.Split(embedParam, ",")
		for _, embedStr := range embedStrings {
			embedStr = strings.TrimSpace(embedStr)
			if embedStr != "" {
				// Create simple embed definition for backward compatibility
				embed := EmbedDefinition{
					Table:    embedStr,
					JoinType: JoinTypeLeft, // Default to left join
					Columns:  []string{"*"},
				}
				query.Embeds = append(query.Embeds, embed)
			}
		}
	}

	return query, nil
}

// parseFilterParam parses a single filter parameter
func (b *PostgRESTBuilder) parseFilterParam(key, value string) (interface{}, error) {
	// Handle logical operators (or, and)
	if key == "or" || key == "and" {
		return b.parseLogicalFilter(key, value)
	}

	// Handle column filters
	if strings.Contains(value, ".") {
		// Check if it's an operator (eq.value, gt.18, etc.)
		parts := strings.SplitN(value, ".", 2)
		if len(parts) == 2 {
			operator := parts[0]
			filterValue := parts[1]

			// Validate operator
			validOps := []string{
				OpEQ, OpNEQ, OpGT, OpGTE, OpLT, OpLTE, OpLike, OpILike, OpIn, OpIs,
			}
			isValid := false
			for _, op := range validOps {
				if operator == op {
					isValid = true
					break
				}
			}
			if !isValid {
				return nil, fmt.Errorf("invalid operator: %s", operator)
			}

			// Parse value based on operator
			parsedValue, err := b.parseFilterValue(operator, filterValue)
			if err != nil {
				return nil, err
			}

			return Filter{
				Column:   key,
				Operator: operator,
				Value:    parsedValue,
			}, nil
		}
	}

	// Default to equality
	return Filter{
		Column:   key,
		Operator: OpEQ,
		Value:    b.parseSimpleValue(value),
	}, nil
}

// parseLogicalFilter parses logical operators (or, and)
func (b *PostgRESTBuilder) parseLogicalFilter(operator, value string) (*LogicalFilter, error) {
	// Handle parentheses: (age.gt.18,name.eq.Alice)
	if !strings.HasPrefix(value, "(") || !strings.HasSuffix(value, ")") {
		return nil, fmt.Errorf("logical filter must be wrapped in parentheses")
	}

	content := value[1 : len(value)-1]
	filters := []interface{}{}

	// Split by comma, but be careful with nested parentheses
	parts := b.splitLogicalParts(content)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Parse as nested filter (format: column.operator.value)
		if strings.Contains(part, ".") {
			parts := strings.Split(part, ".")
			if len(parts) >= 3 {
				column := parts[0]
				operator := parts[1]
				value := strings.Join(parts[2:], ".")

				parsedValue, err := b.parseFilterValue(operator, value)
				if err != nil {
					return nil, err
				}

				filters = append(filters, Filter{
					Column:   column,
					Operator: operator,
					Value:    parsedValue,
				})
			}
		}
	}

	return &LogicalFilter{
		Operator: operator,
		Filters:  filters,
	}, nil
}

// splitLogicalParts splits logical filter parts, handling nested parentheses
func (b *PostgRESTBuilder) splitLogicalParts(content string) []string {
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

// parseFilterValue parses filter values based on operator
func (b *PostgRESTBuilder) parseFilterValue(operator, value string) (interface{}, error) {
	switch operator {
	case OpIn:
		// Parse array: (1,2,3) or "1,2,3"
		if strings.HasPrefix(value, "(") && strings.HasSuffix(value, ")") {
			value = value[1 : len(value)-1]
		}
		parts := strings.Split(value, ",")
		var result []interface{}
		for _, part := range parts {
			result = append(result, b.parseSimpleValue(strings.TrimSpace(part)))
		}
		return result, nil
	case OpIs:
		// Parse null/not null
		if value == "null" {
			return nil, nil
		} else if value == "not.null" {
			return "not null", nil
		}
		return nil, fmt.Errorf("invalid is operator value: %s", value)
	default:
		return b.parseSimpleValue(value), nil
	}
}

// parseSimpleValue parses simple values
func (b *PostgRESTBuilder) parseSimpleValue(value string) interface{} {
	// Handle PostgREST quote escaping: '' -> '
	value = strings.ReplaceAll(value, "''", "'")

	// Try to parse as number
	if intVal, err := strconv.Atoi(value); err == nil {
		return intVal
	}
	if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
		return floatVal
	}
	// Try to parse as boolean
	if boolVal, err := strconv.ParseBool(value); err == nil {
		return boolVal
	}
	// Return as string
	return value
}

// BuildSQL builds SQL from PostgRESTQuery with JOIN support
func (b *PostgRESTBuilder) BuildSQL(q *PostgRESTQuery) (*sqlbuilder.SelectBuilder, error) {
	if q.Table == "" {
		return nil, errors.New("table is required")
	}

	sb := sqlbuilder.NewSelectBuilder()

	// Initialize alias manager for JOIN operations
	aliasManager := NewJoinAliasManager()

	// Get alias for main table
	mainTableAlias := aliasManager.GetAlias(q.Table)

	// Build SELECT clause with JOIN support
	if err := b.buildSelectClause(sb, q, aliasManager); err != nil {
		return nil, err
	}

	// Build FROM clause with main table
	sb.From(fmt.Sprintf("%s AS %s", q.Table, mainTableAlias))

	// Build JOIN clauses
	if err := b.buildJoinClauses(sb, q, aliasManager); err != nil {
		return nil, err
	}

	// Apply filters (sort for deterministic output)
	if len(q.Filters) > 0 {
		// Sort filters by column name for consistent ordering
		sort.Slice(q.Filters, func(i, j int) bool {
			filterI, okI := q.Filters[i].(Filter)
			filterJ, okJ := q.Filters[j].(Filter)
			if okI && okJ {
				return filterI.Column < filterJ.Column
			}
			// For non-Filter types (like LogicalFilter), use string representation
			return fmt.Sprintf("%v", q.Filters[i]) < fmt.Sprintf("%v", q.Filters[j])
		})

		for _, filter := range q.Filters {
			if err := b.applyFilter(sb, filter); err != nil {
				return nil, err
			}
		}
	}

    // Apply ordering
    if len(q.Order) > 0 {
        // When JOINs/embeds are present, unqualified columns in ORDER BY can be ambiguous.
        // Prefix with main table alias unless already qualified.
        orderParts := make([]string, 0, len(q.Order))
        for _, part := range q.Order {
            p := strings.TrimSpace(part)
            // Extract potential "col DESC" or "col ASC"
            col := p
            dir := ""
            if idx := strings.LastIndex(p, " "); idx > 0 {
                col = strings.TrimSpace(p[:idx])
                dir = strings.TrimSpace(p[idx+1:])
            }

            // Qualify with alias if embeds exist and column is unqualified (no dot)
            if len(q.Embeds) > 0 && !strings.Contains(col, ".") {
                col = fmt.Sprintf("%s.%s", mainTableAlias, col)
            }

            if dir != "" {
                orderParts = append(orderParts, col+" "+dir)
            } else {
                orderParts = append(orderParts, col)
            }
        }
        sb.OrderBy(orderParts...)
    }

	// Apply limit and offset
	if q.Limit > 0 {
		sb.Limit(q.Limit)
	}
	if q.Offset > 0 {
		sb.Offset(q.Offset)
	}

	return sb, nil
}

// applyFilter applies a filter to the SQL builder
func (b *PostgRESTBuilder) applyFilter(sb *sqlbuilder.SelectBuilder, filter interface{}) error {
	switch f := filter.(type) {
	case Filter:
		return b.applySimpleFilter(sb, f)
	case *LogicalFilter:
		return b.applyLogicalFilter(sb, f)
	default:
		return fmt.Errorf("unknown filter type: %T", filter)
	}
}

// applySimpleFilter applies a simple filter
func (b *PostgRESTBuilder) applySimpleFilter(sb *sqlbuilder.SelectBuilder, filter Filter) error {
	switch filter.Operator {
	case OpEQ:
		sb.Where(sb.EQ(filter.Column, filter.Value))
	case OpNEQ:
		sb.Where(sb.NE(filter.Column, filter.Value))
	case OpGT:
		sb.Where(sb.GT(filter.Column, filter.Value))
	case OpGTE:
		sb.Where(sb.GE(filter.Column, filter.Value))
	case OpLT:
		sb.Where(sb.LT(filter.Column, filter.Value))
	case OpLTE:
		sb.Where(sb.LE(filter.Column, filter.Value))
	case OpLike:
		sb.Where(sb.Like(filter.Column, filter.Value))
	case OpILike:
		sb.Where(sb.ILike(filter.Column, filter.Value))
	case OpIn:
		if values, ok := filter.Value.([]interface{}); ok {
			sb.Where(sb.In(filter.Column, values...))
		}
	case OpIs:
		if filter.Value == nil {
			sb.Where(sb.IsNull(filter.Column))
		} else if filter.Value == "not null" {
			sb.Where(sb.IsNotNull(filter.Column))
		}
	default:
		return fmt.Errorf("unknown operator: %s", filter.Operator)
	}
	return nil
}

// applyLogicalFilter applies a logical filter
func (b *PostgRESTBuilder) applyLogicalFilter(sb *sqlbuilder.SelectBuilder, filter *LogicalFilter) error {
	if len(filter.Filters) == 0 {
		return nil
	}

	var conditions []string
	for _, subFilter := range filter.Filters {
		condition, err := b.buildFilterCondition(sb, subFilter)
		if err != nil {
			return err
		}
		conditions = append(conditions, condition)
	}

	if len(conditions) > 0 {
		op := " AND "
		if filter.Operator == LogOr {
			op = " OR "
		}
		sb.Where("(" + strings.Join(conditions, op) + ")")
	}

	return nil
}

// buildFilterCondition builds a single filter condition
func (b *PostgRESTBuilder) buildFilterCondition(sb *sqlbuilder.SelectBuilder, filter interface{}) (string, error) {
	switch f := filter.(type) {
	case Filter:
		return b.buildSimpleCondition(sb, f)
	case *LogicalFilter:
		return b.buildLogicalCondition(sb, f)
	default:
		return "", fmt.Errorf("unknown filter type: %T", filter)
	}
}

// buildSimpleCondition builds a simple condition string
func (b *PostgRESTBuilder) buildSimpleCondition(sb *sqlbuilder.SelectBuilder, filter Filter) (string, error) {
	switch filter.Operator {
	case OpEQ:
		return sb.EQ(filter.Column, filter.Value), nil
	case OpNEQ:
		return sb.NE(filter.Column, filter.Value), nil
	case OpGT:
		return sb.GT(filter.Column, filter.Value), nil
	case OpGTE:
		return sb.GE(filter.Column, filter.Value), nil
	case OpLT:
		return sb.LT(filter.Column, filter.Value), nil
	case OpLTE:
		return sb.LE(filter.Column, filter.Value), nil
	case OpLike:
		return sb.Like(filter.Column, filter.Value), nil
	case OpILike:
		return sb.ILike(filter.Column, filter.Value), nil
	case OpIn:
		if values, ok := filter.Value.([]interface{}); ok {
			return sb.In(filter.Column, values...), nil
		}
	case OpIs:
		if filter.Value == nil {
			return sb.IsNull(filter.Column), nil
		} else if filter.Value == "not null" {
			return sb.IsNotNull(filter.Column), nil
		}
	default:
		return "", fmt.Errorf("unknown operator: %s", filter.Operator)
	}
	return "", nil
}

// buildLogicalCondition builds a logical condition string
func (b *PostgRESTBuilder) buildLogicalCondition(sb *sqlbuilder.SelectBuilder, filter *LogicalFilter) (string, error) {
	if len(filter.Filters) == 0 {
		return "", nil
	}

	var conditions []string
	for _, subFilter := range filter.Filters {
		condition, err := b.buildFilterCondition(sb, subFilter)
		if err != nil {
			return "", err
		}
		conditions = append(conditions, condition)
	}

	if len(conditions) > 0 {
		op := " AND "
		if filter.Operator == LogOr {
			op = " OR "
		}
		return "(" + strings.Join(conditions, op) + ")", nil
	}

	return "", nil
}

// ParseSelectWithEmbeds parses select parameter and extracts both columns and embeds
func (b *PostgRESTBuilder) ParseSelectWithEmbeds(selectParam, parentTable string) ([]string, []EmbedDefinition) {
	var columns []string
	var embeds []EmbedDefinition

	// Split by comma, but be careful with nested parentheses
	parts := b.splitSelectParts(selectParam)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check if this is an embed (contains parentheses or join modifiers)
		if strings.Contains(part, "(") || strings.Contains(part, "!") {
			// Parse as embed
			embed, err := b.parseEmbedFromSelect(part, parentTable)
			if err == nil {
				embeds = append(embeds, *embed)
			}
		} else {
			// It's a regular column
			columns = append(columns, part)
		}
	}

	return columns, embeds
}

// splitSelectParts splits select parameter by comma, handling nested parentheses
func (b *PostgRESTBuilder) splitSelectParts(selectParam string) []string {
	var parts []string
	var current strings.Builder
	depth := 0

	for _, char := range selectParam {
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

// parseEmbedFromSelect parses an embed from select parameter
func (b *PostgRESTBuilder) parseEmbedFromSelect(embedStr, parentTable string) (*EmbedDefinition, error) {
	// Create a temporary embed parser
	parser := NewEmbedParser(nil) // No FK resolver for now, will be added later

	return parser.ParseEmbedSyntax(embedStr, parentTable)
}

// buildSelectClause builds the SELECT clause with JOIN support
func (b *PostgRESTBuilder) buildSelectClause(sb *sqlbuilder.SelectBuilder, q *PostgRESTQuery, aliasManager *JoinAliasManager) error {
	var selectColumns []string

	// Pre-create aliases for all embed tables to ensure they exist
	for _, embed := range q.Embeds {
		b.preCreateEmbedAliases(embed, aliasManager)
	}

	// Add columns from main table
	if len(q.Select) > 0 {
		mainTableAlias := aliasManager.GetAlias(q.Table)
		for _, col := range q.Select {
			if col == "*" {
				selectColumns = append(selectColumns, fmt.Sprintf("%s.*", mainTableAlias))
			} else {
				selectColumns = append(selectColumns, fmt.Sprintf("%s.%s", mainTableAlias, col))
			}
		}
	} else {
		// Default to all columns from main table
		mainTableAlias := aliasManager.GetAlias(q.Table)
		selectColumns = append(selectColumns, fmt.Sprintf("%s.*", mainTableAlias))
	}

    // Add columns from embedded tables
    for _, embed := range q.Embeds {
        // Start alias path with the embed table name
        embedColumns, err := b.buildEmbedSelectColumns(embed, aliasManager, embed.Table)
        if err != nil {
            return err
        }
        selectColumns = append(selectColumns, embedColumns...)
    }

	sb.Select(selectColumns...)
	return nil
}

// preCreateEmbedAliases creates aliases for embed tables recursively
func (b *PostgRESTBuilder) preCreateEmbedAliases(embed EmbedDefinition, aliasManager *JoinAliasManager) {
	aliasManager.GetAlias(embed.Table)

	// Pre-create aliases for nested embeds
	for _, nestedEmbed := range embed.NestedEmbeds {
		b.preCreateEmbedAliases(nestedEmbed, aliasManager)
	}
}

// buildEmbedSelectColumns builds SELECT columns for an embed definition
func (b *PostgRESTBuilder) buildEmbedSelectColumns(embed EmbedDefinition, aliasManager *JoinAliasManager, path string) ([]string, error) {
	var columns []string

	// Get alias for this embed table
	alias, exists := aliasManager.GetAliasForTable(embed.Table)
	if !exists {
		return nil, fmt.Errorf("no alias found for table %s", embed.Table)
	}

	// Add columns from this embed
	for _, col := range embed.Columns {
        if col == "*" {
            // Fallback: select all without aliasing when '*' is used
            // Note: Nested scanner won't be able to disambiguate these columns.
            columns = append(columns, fmt.Sprintf("%s.*", alias))
            continue
        }
        // Alias columns with a stable nested path for scanner, e.g. posts__id, posts__comments__text
        aliasName := fmt.Sprintf("%s__%s", path, col)
        columns = append(columns, fmt.Sprintf("%s.%s AS %s", alias, col, aliasName))
	}

	// Add columns from nested embeds
	for _, nestedEmbed := range embed.NestedEmbeds {
        nestedPath := fmt.Sprintf("%s__%s", path, nestedEmbed.Table)
        nestedColumns, err := b.buildEmbedSelectColumns(nestedEmbed, aliasManager, nestedPath)
		if err != nil {
			return nil, err
		}
		columns = append(columns, nestedColumns...)
	}

	return columns, nil
}

// buildJoinClauses builds JOIN clauses for embedded tables
func (b *PostgRESTBuilder) buildJoinClauses(sb *sqlbuilder.SelectBuilder, q *PostgRESTQuery, aliasManager *JoinAliasManager) error {
	for _, embed := range q.Embeds {
		if err := b.buildJoinClause(sb, embed, q.Table, aliasManager); err != nil {
			return err
		}
	}
	return nil
}

// buildJoinClause builds a single JOIN clause
func (b *PostgRESTBuilder) buildJoinClause(sb *sqlbuilder.SelectBuilder, embed EmbedDefinition, parentTable string, aliasManager *JoinAliasManager) error {
	// Get alias for the embed table
	embedAlias := aliasManager.GetAlias(embed.Table)

	// Build JOIN condition
	joinCondition := embed.OnCondition
	if joinCondition == "" {
		// Try to detect relationship automatically
		// For now, use a simple default pattern
		joinCondition = fmt.Sprintf("%s.id = %s.%s_id",
			aliasManager.GetAlias(parentTable),
			embedAlias,
			parentTable)
	}

	// Apply JOIN with appropriate type
	joinOption := embed.JoinType.ToSQLJoinOption()
	sb.JoinWithOption(joinOption, fmt.Sprintf("%s AS %s", embed.Table, embedAlias), joinCondition)

	// Build nested JOINs
	for _, nestedEmbed := range embed.NestedEmbeds {
		if err := b.buildJoinClause(sb, nestedEmbed, embed.Table, aliasManager); err != nil {
			return err
		}
	}

	return nil
}
