package schema

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type (
	Database interface {
		Tables(tables ...string) ([]Table, error)
	}

	Table struct {
		Name    string   `json:"name"`
		Columns []Column `json:"columns"`
		Indexes []Index  `json:"indexes"`
	}

	Column struct {
		Name            string `json:"name"`
		Type            string `json:"type"`
		Nullable        bool   `json:"nullable"`
		Default         string `json:"default"`
		Comment         string `json:"comment"`
		Indexed         bool   `json:"indexed"`
		AutoIncrement   bool   `json:"autoIncrement"`
		PrimaryKey      bool   `json:"primaryKey"`
		ForeignKey      bool   `json:"foreignKey"`
		UniqueKey       bool   `json:"uniqueKey"`
		CheckConstraint string `json:"checkConstraint"`
	}

	Index struct {
		Name    string   `json:"name"`
		Columns []string `json:"columns"`
		Unique  bool     `json:"unique"`
	}
)

type (
	MySQL struct {
		db *sql.DB
	}
)

// NewMySQL creates a new MySQL database instance
func NewMySQL(db *sql.DB) *MySQL {
	return &MySQL{db: db}
}

// Tables loads table information:
// - columns from `information_schema.columns`
// - indexes from `information_schema.statistics`
func (d *MySQL) Tables(tables ...string) ([]Table, error) {
	var result []Table

	// Get current database name from the connection
	var dbName string
	err := d.db.QueryRow("SELECT DATABASE()").Scan(&dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve database name: %w", err)
	}

	// If no tables specified, get all tables
	if len(tables) == 0 {
		allTables, err := d.getAllTables(dbName)
		if err != nil {
			return nil, fmt.Errorf("failed to get all tables: %w", err)
		}
		tables = allTables
	}

	// Process each table
	for _, tableName := range tables {
		table, err := d.getTableInfo(dbName, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get info for table %s: %w", tableName, err)
		}
		result = append(result, table)
	}

	return result, nil
}

// getAllTables retrieves all table names from the database
func (d *MySQL) getAllTables(dbName string) ([]string, error) {
	query := `SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE'`

	rows, err := d.db.Query(query, dbName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		tables = append(tables, tableName)
	}

	return tables, nil
}

// getTableInfo retrieves complete information for a single table
func (d *MySQL) getTableInfo(dbName, tableName string) (Table, error) {
	columns, err := d.getTableColumns(dbName, tableName)
	if err != nil {
		return Table{}, err
	}

	indexes, err := d.getTableIndexes(dbName, tableName)
	if err != nil {
		return Table{}, err
	}

	return Table{
		Name:    tableName,
		Columns: columns,
		Indexes: indexes,
	}, nil
}

// getTableColumns retrieves column information for a table
func (d *MySQL) getTableColumns(dbName, tableName string) ([]Column, error) {
	query := `
		SELECT 
			COLUMN_NAME,
			DATA_TYPE,
			IS_NULLABLE,
			COLUMN_DEFAULT,
			COLUMN_COMMENT,
			COLUMN_KEY,
			EXTRA
		FROM INFORMATION_SCHEMA.COLUMNS 
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`

	rows, err := d.db.Query(query, dbName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var col Column
		var isNullable, key, extra string
		var defaultValue sql.NullString

		err := rows.Scan(
			&col.Name,
			&col.Type,
			&isNullable,
			&defaultValue,
			&col.Comment,
			&key,
			&extra,
		)
		if err != nil {
			return nil, err
		}

		// Convert IS_NULLABLE from string to bool
		col.Nullable = isNullable == "YES"

		if defaultValue.Valid {
			col.Default = defaultValue.String
		}

		// Set key flags
		col.PrimaryKey = key == "PRI"
		col.ForeignKey = key == "MUL"
		col.UniqueKey = key == "UNI"
		col.AutoIncrement = strings.Contains(extra, "auto_increment")

		columns = append(columns, col)
	}

	return columns, nil
}

// getTableIndexes retrieves index information for a table
func (d *MySQL) getTableIndexes(dbName, tableName string) ([]Index, error) {
	query := `
		SELECT 
			INDEX_NAME,
			COLUMN_NAME,
			NON_UNIQUE
		FROM INFORMATION_SCHEMA.STATISTICS 
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX
	`

	rows, err := d.db.Query(query, dbName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexMap := make(map[string]*Index)

	for rows.Next() {
		var indexName, columnName string
		var nonUnique int

		err := rows.Scan(&indexName, &columnName, &nonUnique)
		if err != nil {
			return nil, err
		}

		// Skip PRIMARY key as it's handled in columns
		if indexName == "PRIMARY" {
			continue
		}

		if indexMap[indexName] == nil {
			indexMap[indexName] = &Index{
				Name:    indexName,
				Columns: []string{},
				Unique:  nonUnique == 0,
			}
		}

		indexMap[indexName].Columns = append(indexMap[indexName].Columns, columnName)
	}

	var indexes []Index
	for _, index := range indexMap {
		indexes = append(indexes, *index)
	}

	return indexes, nil
}

func Generate(dsn string, tables ...string) error {
	db, err := OpenDB(dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	if len(tables) == 0 {
		tables = []string{"*"}
	}

	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var table string
		err := rows.Scan(&table)
		if err != nil {
			return err
		}

		if slices.Contains(tables, table) || slices.Contains(tables, "*") {
			err := generateTableSchema(db, table)
			if err != nil {
				return fmt.Errorf("failed to generate schema for table %s: %w", table, err)
			}
		}
	}

	return nil
}

type ColumnInfo struct {
	Name     string
	Type     string
	Nullable bool
	Key      string
	Comment  string
}

func getTableColumns(db *sql.DB, tableName string) ([]ColumnInfo, error) {
	query := `SELECT COLUMN_NAME, DATA_TYPE, IS_NULLABLE, COLUMN_KEY, COLUMN_COMMENT 
			  FROM INFORMATION_SCHEMA.COLUMNS 
			  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`

	rows, err := db.Query(query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var isNullable string
		err := rows.Scan(&col.Name, &col.Type, &isNullable, &col.Key, &col.Comment)
		if err != nil {
			return nil, err
		}
		// Convert IS_NULLABLE from string to bool
		col.Nullable = isNullable == "YES"
		columns = append(columns, col)
	}

	return columns, nil
}

func generateTableSchema(db *sql.DB, tableName string) error {
	// Extract database connection details from DSN
	// For now, we'll use a simpler approach by querying the table directly
	columns, err := getTableColumns(db, tableName)
	if err != nil {
		return err
	}

	// Convert columns to JSON schema format
	schema := convertToJSONSchema(tableName, columns)

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return err
	}

	// Create schemas directory if it doesn't exist
	schemasDir := "data/schemas"
	err = os.MkdirAll(schemasDir, 0755)
	if err != nil {
		return err
	}

	// Write JSON file
	fileName := filepath.Join(schemasDir, tableName+".json")
	err = os.WriteFile(fileName, jsonData, 0644)
	if err != nil {
		return err
	}

	fmt.Printf("Generated schema: %s\n", fileName)
	return nil
}

func convertToJSONSchema(tableName string, columns []ColumnInfo) map[string]interface{} {
	fields := make([]map[string]interface{}, 0, len(columns))

	for _, col := range columns {
		field := map[string]interface{}{
			"type":     convertMySQLTypeToJSONType(col.Type),
			"name":     col.Name,
			"label":    strings.Title(col.Name),
			"optional": col.Nullable,
			"sortable": true,
			"renderer": getRendererForType(col.Type),
			"db":       map[string]interface{}{},
		}
		fields = append(fields, field)
	}

	// Determine label field (first non-id field or "id")
	labelField := "id"
	for _, col := range columns {
		if col.Name != "id" {
			labelField = col.Name
			break
		}
	}

	return map[string]interface{}{
		"name":              tableName,
		"namespace":         tableName + "s",
		"label_field":       labelField,
		"disable_timestamp": false,
		"fields":            fields,
	}
}

// binary
// blob
// varbinary

func convertMySQLTypeToJSONType(mysqlType string) string {

	mappings := map[string][]string{
		"int":    {"int", "bigint", "smallint", "mediumint", "tinyint"},
		"float":  {"float", "double", "decimal"},
		"string": {"varchar", "text", "char", "enum", "longtext", "mediumtext", "set"},
		"time":   {"datetime", "time", "timestamp"},
		"bool":   {"bool", "bit"},
		"json":   {"json"},
		"binary": {"binary", "blob", "varbinary", "longblob"},
	}

	mysqlType = strings.ToLower(mysqlType)

	for key, values := range mappings {
		if slices.Contains(values, mysqlType) {
			return key
		}
	}

	return "string"
}

func getRendererForType(mysqlType string) map[string]interface{} {
	mysqlType = strings.ToLower(mysqlType)

	switch {
	case strings.Contains(mysqlType, "int"), strings.Contains(mysqlType, "decimal"), strings.Contains(mysqlType, "float"), strings.Contains(mysqlType, "double"):
		return map[string]interface{}{
			"class": "number",
		}
	case strings.Contains(mysqlType, "date"), strings.Contains(mysqlType, "time"), strings.Contains(mysqlType, "timestamp"):
		return map[string]interface{}{
			"class": "date",
		}
	case strings.Contains(mysqlType, "bool"), strings.Contains(mysqlType, "bit"):
		return map[string]interface{}{
			"class": "checkbox",
		}
	default:
		return map[string]interface{}{
			"class": "input",
			"settings": map[string]interface{}{
				"hide_form_label": false,
			},
		}
	}
}

func OpenDB(dsn string) (*sql.DB, error) {

	split := strings.Split(dsn, "://")
	driver, uri := split[0], split[1]

	if driver == "mysql" {
		// uri = dsn
	}

	db, err := sql.Open(driver, uri)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// TABLE_CATALOG,varchar(64),NO
// TABLE_SCHEMA,varchar(64),NO
// TABLE_NAME,varchar(64),NO
// TABLE_TYPE,"enum('BASE TABLE','VIEW','SYSTEM VIEW')",NO
// ENGINE,varchar(64),YES
// VERSION,int,YES
// ROW_FORMAT,"enum('Fixed','Dynamic','Compressed','Redundant','Compact','Paged')",YES
// TABLE_ROWS,bigint unsigned,YES
// AVG_ROW_LENGTH,bigint unsigned,YES
// DATA_LENGTH,bigint unsigned,YES
// MAX_DATA_LENGTH,bigint unsigned,YES
// INDEX_LENGTH,bigint unsigned,YES
// DATA_FREE,bigint unsigned,YES
// AUTO_INCREMENT,bigint unsigned,YES
// CREATE_TIME,timestamp,NO
// UPDATE_TIME,datetime,YES
// CHECK_TIME,datetime,YES
// TABLE_COLLATION,varchar(64),YES
// CHECKSUM,bigint,YES
// CREATE_OPTIONS,varchar(256),YES
// TABLE_COMMENT,text,YES
