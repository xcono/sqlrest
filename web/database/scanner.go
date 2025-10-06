package database

import (
	"database/sql"
	"log"
	"strconv"
	"strings"
)

// convertValue converts database values to appropriate Go types
func convertValue(rawVal interface{}) interface{} {
	if rawVal == nil {
		return nil
	}

	// Handle []byte (common for text/numeric types)
	if b, ok := rawVal.([]byte); ok {
		str := string(b)

		// Try to convert to number if it looks like one
		if num, err := strconv.ParseFloat(str, 64); err == nil {
			return num
		}

		// Return as string if not a number
		return str
	}

	return rawVal
}

// Scanner provides utilities for scanning database results
type Scanner struct{}

// NewScanner creates a new result scanner
func NewScanner() *Scanner {
	return &Scanner{}
}

// ScanRows scans SQL rows into a slice of maps
func (s *Scanner) ScanRows(rows *sql.Rows) ([]map[string]interface{}, error) {
	return s.ScanRowsWithEmbeds(rows, "", []string{})
}

// ScanRowsWithEmbeds scans SQL rows with PostgREST-style nested embedding
// parentTable: the main table name (e.g., "album" for /album?select=*,artist(name))
// embedTables: list of embedded table names (e.g., ["artist"])
func (s *Scanner) ScanRowsWithEmbeds(rows *sql.Rows, parentTable string, embedTables []string) ([]map[string]interface{}, error) {
	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		log.Printf("Error getting columns: %v", err)
		return nil, err
	}

	// Detect whether we have nested aliases (e.g., album__id, artist__name)
	hasNested := false
	for _, c := range columns {
		if strings.Contains(c, "__") {
			hasNested = true
			break
		}
	}

	results := make([]map[string]interface{}, 0)
	rowCount := 0

	for rows.Next() {
		// Create slice of interface{} to hold column values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		// Scan the row into valuePtrs
		if err := rows.Scan(valuePtrs...); err != nil {
			log.Printf("Error scanning row %d: %v", rowCount, err)
			continue
		}

		// Create map for this row
		row := make(map[string]interface{})

		if hasNested && parentTable != "" {
			// Build PostgREST-style nested structure
			for i, col := range columns {
				rawVal := values[i]
				val := convertValue(rawVal)

				// Handle aliased columns with __ delimiter
				if strings.Contains(col, "__") {
					parts := strings.Split(col, "__")
					if len(parts) >= 2 {
						if parts[0] == parentTable {
							// Parent table columns go at root level
							row[parts[1]] = val
						} else {
							// Handle nested structure: table1__table2__column
							current := row
							for i := 0; i < len(parts)-1; i++ {
								tableName := parts[i]
								if _, exists := current[tableName]; !exists {
									current[tableName] = make(map[string]interface{})
								}
								if nestedObj, ok := current[tableName].(map[string]interface{}); ok {
									current = nestedObj
								}
							}
							// Set the final column value
							columnName := parts[len(parts)-1]
							current[columnName] = val
						}
					}
				} else {
					// Handle unaliased columns (fallback for SELECT *)
					// For SELECT * with embeds, we need to determine which table each column belongs to
					// This is complex without schema information, so we'll use a heuristic approach

					// Common patterns for determining table ownership:
					// - Columns with table_id suffix likely belong to that table
					// - Primary key columns (id, table_id) belong to their respective tables

					isEmbedColumn := false
					for _, embedTable := range embedTables {
						// Check if column name suggests it belongs to an embed table
						if strings.HasSuffix(col, "_id") && strings.Contains(col, embedTable) {
							// This looks like a foreign key to the embed table
							if _, exists := row[embedTable]; !exists {
								row[embedTable] = make(map[string]interface{})
							}
							if nestedObj, ok := row[embedTable].(map[string]interface{}); ok {
								nestedObj[col] = val
							}
							isEmbedColumn = true
							break
						} else if col == embedTable+"_id" {
							// This is the foreign key column, belongs to parent table
							row[col] = val
							isEmbedColumn = true
							break
						}
					}

					if !isEmbedColumn {
						// Default to parent table column
						row[col] = val
					}
				}
			}
		} else {
			// Flat mapping (backward compatibility)
			for i, col := range columns {
				rawVal := values[i]
				row[col] = convertValue(rawVal)
			}
		}

		// Post-process: Convert all-NULL embedded objects to null (PostgREST behavior)
		if hasNested && parentTable != "" {
			row = s.convertNullEmbedsToNull(row, embedTables)
		}

		results = append(results, row)
		rowCount++
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		log.Printf("Error iterating rows: %v", err)
		return nil, err
	}

	log.Printf("Query returned %d rows", rowCount)
	return results, nil
}

// setNested sets a value into a nested map structure given a path.
// Example: path ["users", "posts", "id"] will create row["users"]["posts"]["id"] = value
func setNested(root map[string]interface{}, path []string, value interface{}) {
	current := root
	for i := 0; i < len(path); i++ {
		key := path[i]
		isLast := i == len(path)-1
		if isLast {
			current[key] = value
			return
		}

		// Ensure next level is a map
		next, exists := current[key]
		if !exists || next == nil {
			child := make(map[string]interface{})
			current[key] = child
			current = child
			continue
		}

		if m, ok := next.(map[string]interface{}); ok {
			current = m
			continue
		}

		// If there is a type conflict, overwrite with a new map
		child := make(map[string]interface{})
		current[key] = child
		current = child
	}
}

// GetRowCount returns the number of rows in the result set
func (s *Scanner) GetRowCount(rows *sql.Rows) (int, error) {
	count := 0
	for rows.Next() {
		count++
	}
	return count, rows.Err()
}

// convertNullEmbedsToNull converts embedded objects that are all-NULL to null
// This matches PostgREST's behavior for LEFT JOINs where no matching rows exist
func (s *Scanner) convertNullEmbedsToNull(row map[string]interface{}, embedTables []string) map[string]interface{} {
	for _, embedTable := range embedTables {
		if embedObj, exists := row[embedTable]; exists {
			if embedMap, ok := embedObj.(map[string]interface{}); ok {
				// Check if all values in the embed object are nil
				allNull := true
				for _, val := range embedMap {
					if val != nil {
						allNull = false
						break
					}
				}

				// If all values are null, convert the entire embed to null
				if allNull {
					row[embedTable] = nil
				}
			}
		}
	}
	return row
}
