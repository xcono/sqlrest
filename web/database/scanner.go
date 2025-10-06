package database

import (
    "database/sql"
    "log"
    "strings"
)

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

	var results []map[string]interface{}
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

        // Populate row fields, supporting nested alias paths like "posts__comments__text"
        for i, col := range columns {
            rawVal := values[i]

            // Convert []byte to string for JSON serialization
            var val interface{}
            if rawVal != nil {
                if b, ok := rawVal.([]byte); ok {
                    val = string(b)
                } else {
                    val = rawVal
                }
            } else {
                val = nil
            }

            // If the column alias encodes a nested path, materialize nested maps
            if strings.Contains(col, "__") {
                if val == nil {
                    // Skip creating nested structure when value is null to avoid empty objects
                    continue
                }
                s.setNestedValue(row, col, val)
                continue
            }

            // Default: set as top-level field
            row[col] = val
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

// GetRowCount returns the number of rows in the result set
func (s *Scanner) GetRowCount(rows *sql.Rows) (int, error) {
	count := 0
	for rows.Next() {
		count++
	}
	return count, rows.Err()
}

// setNestedValue creates nested objects for a column alias path and assigns the value.
// Example: path "posts__comments__text" with value "hello" produces:
// row["posts"]["comments"]["text"] = "hello"
func (s *Scanner) setNestedValue(row map[string]interface{}, aliasPath string, value interface{}) {
    parts := strings.Split(aliasPath, "__")
    if len(parts) == 0 {
        return
    }

    // Traverse or create nested maps
    current := row
    for idx := 0; idx < len(parts)-1; idx++ {
        key := parts[idx]
        // Ensure a map exists at this key
        next, ok := current[key]
        if !ok || next == nil {
            child := make(map[string]interface{})
            current[key] = child
            current = child
            continue
        }
        // If existing value is not a map, overwrite with a map to maintain consistency
        if asMap, ok := next.(map[string]interface{}); ok {
            current = asMap
        } else {
            child := make(map[string]interface{})
            current[key] = child
            current = child
        }
    }

    // Set leaf
    leafKey := parts[len(parts)-1]
    current[leafKey] = value
}
