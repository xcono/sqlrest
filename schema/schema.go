package schema

type (

	// Services isolates access to the database.
	Service struct {
		// Db is the database name.
		DSN string `yaml:"dsn" json:"dsn"`
		// Schemas is a named map used for defining schemas.
		Schemas Schemas `yaml:"schemas" json:"schemas,optional"`
	}

	// Schemas is a named map used for defining schemas
	Schemas map[string]Schema

	// Schema is a named map used for defining schema
	Schema struct {
		// Table name in the database.
		Table string `yaml:"table" json:"table,optional"`
		// Fields is a named map used for defining fields.
		Fields Fields `yaml:"fields" json:"fields,optional"`
	}

	// Fields is a named map used for defining fields
	Fields map[string]Field

	// Field is column with additional metadata
	Field struct {
		Column Column `json:"column,optional"`
		Widget Widget `yaml:"widget" json:"widget,optional"`
	}

	// Widget is column with additional metadata
	Widget struct {
		// Column name in the database.
		Column string `yaml:"column" json:"column,optional"`
		// Field name in the API.
		Name string `yaml:"name" json:"name,optional"`
		// If true, the field will be sorted.
		// By default all indexed fields are sorted.
		Sorted bool `yaml:"sorted" json:"sorted,optional"`
		// If true, the field will be filtered.
		// By default all indexed fields are filtered.
		Filter bool `yaml:"filter" json:"filter,optional"`
		// If true, the field will be hidden.
		// By default hidden columns named: pass, password, hash, token, secret.
		Hidden bool `yaml:"hidden" json:"hidden,optional"`
	}
)
