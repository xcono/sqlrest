package database

import (
	"database/sql"
	"log"
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
		for i, col := range columns {
			val := values[i]
			if val != nil {
				// Convert []byte to string for JSON serialization
				if b, ok := val.([]byte); ok {
					row[col] = string(b)
				} else {
					row[col] = val
				}
			} else {
				row[col] = nil
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

// GetRowCount returns the number of rows in the result set
func (s *Scanner) GetRowCount(rows *sql.Rows) (int, error) {
	count := 0
	for rows.Next() {
		count++
	}
	return count, rows.Err()
}
