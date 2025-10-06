package dbseed

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// SeedMySQL executes the MySQL migration file
func SeedMySQL(t *testing.T, dsn string) *sql.DB {
	// Use root user for database operations
	rootDSN := strings.Replace(dsn, "testuser:testpass", "root:rootpass", 1)
	db, err := sql.Open("mysql", rootDSN)
	if err != nil {
		t.Fatalf("Failed to connect to MySQL: %v", err)
	}

	// Disable foreign key checks temporarily
	_, err = db.Exec("SET FOREIGN_KEY_CHECKS = 0")
	if err != nil {
		t.Fatalf("Failed to disable foreign key checks: %v", err)
	}

	// Clean existing tables first
	cleanupMySQLTables(t, db)

	// Read and execute migration
	sqlBytes, err := os.ReadFile("migrations/my/simple_mysql.sql")
	if err != nil {
		t.Fatalf("Failed to read MySQL migration: %v", err)
	}

	// Execute migration using proper SQL statement parsing
	statements := parseSQLStatements(string(sqlBytes))
	t.Logf("Parsed %d SQL statements from migration file", len(statements))
	insertCount := 0
	for _, stmt := range statements {
		if strings.Contains(strings.ToUpper(stmt), "INSERT") {
			insertCount++
		}
	}
	t.Logf("Found %d INSERT statements", insertCount)
	for _, stmt := range statements {
		if stmt == "" {
			continue
		}
		// Log INSERT statements for debugging
		if strings.Contains(strings.ToUpper(stmt), "INSERT") {
			stmtPreview := stmt
			if len(stmt) > 100 {
				stmtPreview = stmt[:100] + "..."
			}
			t.Logf("Executing INSERT statement: %s", stmtPreview)
		}
		_, err = db.Exec(stmt)
		if err != nil {
			t.Fatalf("Failed to execute SQL statement: %v\nStatement: %s", err, stmt)
		}
	}

	// Re-enable foreign key checks
	_, err = db.Exec("SET FOREIGN_KEY_CHECKS = 1")
	if err != nil {
		t.Fatalf("Failed to re-enable foreign key checks: %v", err)
	}

	// Close root connection and return connection as testuser
	db.Close()

	testDB, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("Failed to connect to MySQL as testuser: %v", err)
	}

	return testDB
}

// parseSQLStatements properly parses SQL file into executable statements
func parseSQLStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	inMultiLineComment := false

	lines := strings.Split(sql, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Handle single-line comments
		if strings.HasPrefix(line, "--") {
			continue
		}

		// Handle multi-line comments
		if strings.Contains(line, "/*") {
			inMultiLineComment = true
		}
		if strings.Contains(line, "*/") {
			inMultiLineComment = false
			continue
		}
		if inMultiLineComment {
			continue
		}

		// Handle comment blocks (/* */)
		if strings.HasPrefix(line, "/**") || strings.HasPrefix(line, "/*") {
			continue
		}

		// Skip comment-only lines
		if strings.HasPrefix(line, "*") {
			continue
		}

		// Build current statement
		current.WriteString(line)
		current.WriteString(" ")

		// Check if line ends with semicolon
		if strings.HasSuffix(line, ";") {
			stmt := strings.TrimSpace(current.String())

			// Skip database creation/drop statements and USE statements
			if stmt != "" && len(stmt) > 10 &&
				!strings.Contains(strings.ToUpper(stmt), "DROP DATABASE") &&
				!strings.Contains(strings.ToUpper(stmt), "CREATE DATABASE") &&
				!strings.Contains(strings.ToUpper(stmt), "USE ") &&
				!strings.Contains(strings.ToUpper(stmt), "\\C ") {
				statements = append(statements, stmt)
			}
			current.Reset()
		}
	}

	// Handle any remaining statement without semicolon (for multi-line INSERTs)
	if current.Len() > 0 {
		stmt := strings.TrimSpace(current.String())
		if stmt != "" && len(stmt) > 10 &&
			!strings.Contains(strings.ToUpper(stmt), "DROP DATABASE") &&
			!strings.Contains(strings.ToUpper(stmt), "CREATE DATABASE") &&
			!strings.Contains(strings.ToUpper(stmt), "USE ") &&
			!strings.Contains(strings.ToUpper(stmt), "\\C ") {
			statements = append(statements, stmt)
		}
	}

	return statements
}

// createTestSchema creates a simplified test schema
func createTestSchema(t *testing.T, db *sql.DB) {
	// Create tables with test data
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS Album (
		AlbumId INT NOT NULL AUTO_INCREMENT,
		Title VARCHAR(160) NOT NULL,
		ArtistId INT NOT NULL,
		PRIMARY KEY (AlbumId)
	)`

	_, err := db.Exec(createTableSQL)
	if err != nil {
		t.Fatalf("Failed to create Album table: %v", err)
	}

	createTableSQL = `
	CREATE TABLE IF NOT EXISTS Artist (
		ArtistId INT NOT NULL AUTO_INCREMENT,
		Name VARCHAR(120),
		PRIMARY KEY (ArtistId)
	)`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		t.Fatalf("Failed to create Artist table: %v", err)
	}

	createTableSQL = `
	CREATE TABLE IF NOT EXISTS Genre (
		GenreId INT NOT NULL AUTO_INCREMENT,
		Name VARCHAR(120),
		PRIMARY KEY (GenreId)
	)`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		t.Fatalf("Failed to create Genre table: %v", err)
	}

	createTableSQL = `
	CREATE TABLE IF NOT EXISTS Customer (
		CustomerId INT NOT NULL AUTO_INCREMENT,
		FirstName VARCHAR(40) NOT NULL,
		LastName VARCHAR(20) NOT NULL,
		Country VARCHAR(40),
		PRIMARY KEY (CustomerId)
	)`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		t.Fatalf("Failed to create Customer table: %v", err)
	}

	createTableSQL = `
	CREATE TABLE IF NOT EXISTS Track (
		TrackId INT NOT NULL AUTO_INCREMENT,
		Name VARCHAR(200) NOT NULL,
		AlbumId INT,
		GenreId INT,
		PRIMARY KEY (TrackId)
	)`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		t.Fatalf("Failed to create Track table: %v", err)
	}

	// Insert test data
	insertData := `
	INSERT INTO Artist (ArtistId, Name) VALUES 
	(1, 'AC/DC'), (2, 'Accept'), (3, 'Aerosmith'), (4, 'Alanis Morissette'), (5, 'Alice In Chains')`

	_, err = db.Exec(insertData)
	if err != nil {
		t.Fatalf("Failed to insert Artist data: %v", err)
	}

	insertData = `
	INSERT INTO Album (AlbumId, Title, ArtistId) VALUES 
	(1, 'For Those About To Rock We Salute You', 1),
	(2, 'Balls to the Wall', 2),
	(3, 'Restless and Wild', 2),
	(4, 'Let There Be Rock', 1),
	(5, 'Big Ones', 3)`

	_, err = db.Exec(insertData)
	if err != nil {
		t.Fatalf("Failed to insert Album data: %v", err)
	}

	insertData = `
	INSERT INTO Genre (GenreId, Name) VALUES 
	(1, 'Rock'), (2, 'Jazz'), (3, 'Metal'), (4, 'Alternative & Punk'), (5, 'Rock And Roll')`

	_, err = db.Exec(insertData)
	if err != nil {
		t.Fatalf("Failed to insert Genre data: %v", err)
	}

	insertData = `
	INSERT INTO Customer (CustomerId, FirstName, LastName, Country) VALUES 
	(1, 'Luís', 'Gonçalves', 'Brazil'),
	(2, 'Leonie', 'Köhler', 'Germany'),
	(3, 'François', 'Tremblay', 'Canada'),
	(4, 'Bjørn', 'Hansen', 'Norway'),
	(5, 'František', 'Wichterlová', 'Czech Republic'),
	(6, 'Helena', 'Holý', 'Czech Republic'),
	(7, 'Astrid', 'Gruber', 'Austria'),
	(8, 'Daan', 'Peeters', 'Belgium'),
	(9, 'Kara', 'Nielsen', 'Denmark'),
	(10, 'Eduardo', 'Martins', 'Brazil'),
	(11, 'Alexandre', 'Rocha', 'Brazil'),
	(12, 'Roberto', 'Almeida', 'Brazil'),
	(13, 'Fernanda', 'Ramos', 'Brazil'),
	(14, 'Mark', 'Philips', 'Canada'),
	(15, 'Jennifer', 'Peterson', 'Canada'),
	(16, 'Frank', 'Harris', 'USA'),
	(17, 'Jack', 'Smith', 'USA'),
	(18, 'Michelle', 'Brooks', 'USA'),
	(19, 'Tim', 'Goyer', 'USA'),
	(20, 'Dan', 'Miller', 'USA')`

	_, err = db.Exec(insertData)
	if err != nil {
		t.Fatalf("Failed to insert Customer data: %v", err)
	}

	insertData = `
	INSERT INTO Track (TrackId, Name, AlbumId, GenreId) VALUES 
	(1, 'For Those About To Rock (We Salute You)', 1, 1),
	(2, 'Balls to the Wall', 2, 3),
	(3, 'Fast As a Shark', 3, 3),
	(4, 'Restless and Wild', 3, 3),
	(5, 'Princess of the Dawn', 3, 3),
	(6, 'Put The Finger On You', 1, 1),
	(7, 'Let''s Get It Up', 1, 1),
	(8, 'Inject The Venom', 1, 1),
	(9, 'Snowballed', 1, 1),
	(10, 'Evil Walks', 1, 1)`

	_, err = db.Exec(insertData)
	if err != nil {
		t.Fatalf("Failed to insert Track data: %v", err)
	}
}

// SeedPostgres executes the PostgreSQL migration file
func SeedPostgres(t *testing.T, dsn string) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	// Disable foreign key checks temporarily (PostgreSQL approach)
	_, err = db.Exec("SET session_replication_role = replica")
	if err != nil {
		t.Fatalf("Failed to disable foreign key checks: %v", err)
	}

	// Clean existing tables first
	cleanupPostgresTables(t, db)

	// Read and execute migration
	sqlBytes, err := os.ReadFile("migrations/pg/simple_postgres.sql")
	if err != nil {
		t.Fatalf("Failed to read PostgreSQL migration: %v", err)
	}

	// Execute migration using proper SQL statement parsing
	statements := parseSQLStatements(string(sqlBytes))
	t.Logf("Parsed %d SQL statements from migration file", len(statements))
	insertCount := 0
	for _, stmt := range statements {
		if strings.Contains(strings.ToUpper(stmt), "INSERT") {
			insertCount++
		}
	}
	t.Logf("Found %d INSERT statements", insertCount)
	for _, stmt := range statements {
		if stmt == "" {
			continue
		}
		// Log INSERT statements for debugging
		if strings.Contains(strings.ToUpper(stmt), "INSERT") {
			stmtPreview := stmt
			if len(stmt) > 100 {
				stmtPreview = stmt[:100] + "..."
			}
			t.Logf("Executing INSERT statement: %s", stmtPreview)
		}
		_, err = db.Exec(stmt)
		if err != nil {
			t.Fatalf("Failed to execute SQL statement: %v\nStatement: %s", err, stmt)
		}
	}

	// Re-enable foreign key checks
	_, err = db.Exec("SET session_replication_role = DEFAULT")
	if err != nil {
		t.Fatalf("Failed to re-enable foreign key checks: %v", err)
	}
}

// createPostgresTestSchema creates a simplified test schema for PostgreSQL
func createPostgresTestSchema(t *testing.T, db *sql.DB) {
	// Create tables with test data
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS album (
		album_id SERIAL PRIMARY KEY,
		title VARCHAR(160) NOT NULL,
		artist_id INTEGER NOT NULL
	)`

	_, err := db.Exec(createTableSQL)
	if err != nil {
		t.Fatalf("Failed to create album table: %v", err)
	}

	createTableSQL = `
	CREATE TABLE IF NOT EXISTS artist (
		artist_id SERIAL PRIMARY KEY,
		name VARCHAR(120)
	)`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		t.Fatalf("Failed to create artist table: %v", err)
	}

	createTableSQL = `
	CREATE TABLE IF NOT EXISTS genre (
		genre_id SERIAL PRIMARY KEY,
		name VARCHAR(120)
	)`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		t.Fatalf("Failed to create genre table: %v", err)
	}

	createTableSQL = `
	CREATE TABLE IF NOT EXISTS customer (
		customer_id SERIAL PRIMARY KEY,
		first_name VARCHAR(40) NOT NULL,
		last_name VARCHAR(20) NOT NULL,
		country VARCHAR(40)
	)`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		t.Fatalf("Failed to create customer table: %v", err)
	}

	createTableSQL = `
	CREATE TABLE IF NOT EXISTS track (
		track_id SERIAL PRIMARY KEY,
		name VARCHAR(200) NOT NULL,
		album_id INTEGER,
		genre_id INTEGER
	)`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		t.Fatalf("Failed to create track table: %v", err)
	}

	// Insert test data
	insertData := `
	INSERT INTO artist (artist_id, name) VALUES 
	(1, 'AC/DC'), (2, 'Accept'), (3, 'Aerosmith'), (4, 'Alanis Morissette'), (5, 'Alice In Chains')`

	_, err = db.Exec(insertData)
	if err != nil {
		t.Fatalf("Failed to insert artist data: %v", err)
	}

	insertData = `
	INSERT INTO album (album_id, title, artist_id) VALUES 
	(1, 'For Those About To Rock We Salute You', 1),
	(2, 'Balls to the Wall', 2),
	(3, 'Restless and Wild', 2),
	(4, 'Let There Be Rock', 1),
	(5, 'Big Ones', 3)`

	_, err = db.Exec(insertData)
	if err != nil {
		t.Fatalf("Failed to insert album data: %v", err)
	}

	insertData = `
	INSERT INTO genre (genre_id, name) VALUES 
	(1, 'Rock'), (2, 'Jazz'), (3, 'Metal'), (4, 'Alternative & Punk'), (5, 'Rock And Roll')`

	_, err = db.Exec(insertData)
	if err != nil {
		t.Fatalf("Failed to insert genre data: %v", err)
	}

	insertData = `
	INSERT INTO customer (customer_id, first_name, last_name, country) VALUES 
	(1, 'Luís', 'Gonçalves', 'Brazil'),
	(2, 'Leonie', 'Köhler', 'Germany'),
	(3, 'François', 'Tremblay', 'Canada'),
	(4, 'Bjørn', 'Hansen', 'Norway'),
	(5, 'František', 'Wichterlová', 'Czech Republic'),
	(6, 'Helena', 'Holý', 'Czech Republic'),
	(7, 'Astrid', 'Gruber', 'Austria'),
	(8, 'Daan', 'Peeters', 'Belgium'),
	(9, 'Kara', 'Nielsen', 'Denmark'),
	(10, 'Eduardo', 'Martins', 'Brazil'),
	(11, 'Alexandre', 'Rocha', 'Brazil'),
	(12, 'Roberto', 'Almeida', 'Brazil'),
	(13, 'Fernanda', 'Ramos', 'Brazil'),
	(14, 'Mark', 'Philips', 'Canada'),
	(15, 'Jennifer', 'Peterson', 'Canada'),
	(16, 'Frank', 'Harris', 'USA'),
	(17, 'Jack', 'Smith', 'USA'),
	(18, 'Michelle', 'Brooks', 'USA'),
	(19, 'Tim', 'Goyer', 'USA'),
	(20, 'Dan', 'Miller', 'USA')`

	_, err = db.Exec(insertData)
	if err != nil {
		t.Fatalf("Failed to insert customer data: %v", err)
	}

	insertData = `
	INSERT INTO track (track_id, name, album_id, genre_id) VALUES 
	(1, 'For Those About To Rock (We Salute You)', 1, 1),
	(2, 'Balls to the Wall', 2, 3),
	(3, 'Fast As a Shark', 3, 3),
	(4, 'Restless and Wild', 3, 3),
	(5, 'Princess of the Dawn', 3, 3),
	(6, 'Put The Finger On You', 1, 1),
	(7, 'Let''s Get It Up', 1, 1),
	(8, 'Inject The Venom', 1, 1),
	(9, 'Snowballed', 1, 1),
	(10, 'Evil Walks', 1, 1)`

	_, err = db.Exec(insertData)
	if err != nil {
		t.Fatalf("Failed to insert track data: %v", err)
	}
}

// cleanupMySQLTables drops all existing tables
func cleanupMySQLTables(t *testing.T, db *sql.DB) {
	// Tables in reverse order to handle foreign key constraints
	tables := []string{"playlist_track", "track", "album", "artist", "genre"}

	for _, table := range tables {
		_, err := db.Exec("DROP TABLE IF EXISTS " + table)
		if err != nil {
			t.Fatalf("Failed to drop table %s: %v", table, err)
		}
	}
}

// cleanupPostgresTables drops all existing tables
func cleanupPostgresTables(t *testing.T, db *sql.DB) {
	// Tables in reverse order to handle foreign key constraints
	tables := []string{"playlist_track", "track", "album", "artist", "genre"}

	for _, table := range tables {
		_, err := db.Exec("DROP TABLE IF EXISTS " + table + " CASCADE")
		if err != nil {
			t.Fatalf("Failed to drop table %s: %v", table, err)
		}
	}
}

// CleanMySQL truncates all tables
func CleanMySQL(db *sql.DB) error {
	// Tables in reverse order to handle foreign key constraints
	tables := []string{"PlaylistTrack", "InvoiceLine", "Track", "Playlist",
		"Invoice", "Customer", "Employee", "Album", "Artist",
		"MediaType", "Genre"}

	for _, table := range tables {
		_, err := db.Exec("DELETE FROM " + table)
		if err != nil {
			return err
		}
	}
	return nil
}

// CleanPostgres truncates all tables
func CleanPostgres(db *sql.DB) error {
	// Tables in reverse order to handle foreign key constraints
	tables := []string{"playlist_track", "invoice_line", "track", "playlist",
		"invoice", "customer", "employee", "album", "artist",
		"media_type", "genre"}

	for _, table := range tables {
		_, err := db.Exec("DELETE FROM " + table)
		if err != nil {
			return err
		}
	}
	return nil
}
