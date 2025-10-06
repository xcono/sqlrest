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
	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		log.Printf("Error getting columns: %v", err)
		return nil, err
	}

	// Detect whether we have nested aliases (e.g., users__id or users__posts__id)
	hasNested := false
	for _, c := range columns {
		if strings.Contains(c, "__") || strings.Contains(c, ".") {
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

		if hasNested {
			// Build nested structure based on alias paths
			for i, col := range columns {
				rawVal := values[i]
				val := convertValue(rawVal)

				// Support both alias styles: users__posts__id or users.id
				var path []string
				if strings.Contains(col, "__") {
					path = strings.Split(col, "__")
				} else if strings.Contains(col, ".") {
					path = strings.Split(col, ".")
				} else {
					// Fallback to flat if we cannot detect path
					row[col] = val
					continue
				}

				// Normalize empty segments
				normalized := make([]string, 0, len(path))
				for _, seg := range path {
					seg = strings.TrimSpace(seg)
					if seg != "" {
						normalized = append(normalized, seg)
					}
				}

				if len(normalized) == 0 {
					continue
				}

				setNested(row, normalized, val)
			}
		} else {
			// Flat mapping (backward compatibility)
			for i, col := range columns {
				rawVal := values[i]
				row[col] = convertValue(rawVal)
			}
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
