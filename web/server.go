package web

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/xcono/legs/schema"
	"github.com/xcono/legs/web/database"
	"github.com/xcono/legs/web/handlers"
)

func StartServer(c schema.Config) {
	// Get the first service configuration
	var serviceConfig schema.Service
	for _, service := range c.Services {
		serviceConfig = service
		break
	}

	// Open database connection
	db, err := schema.OpenDB(serviceConfig.DSN)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create database executor and router
	dbExecutor := database.NewExecutor(db)
	router := handlers.NewRouter(dbExecutor)

	// Create HTTP server
	mux := http.NewServeMux()

	// Add CORS middleware
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept")
		w.Header().Set("Content-Type", "application/json")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Route requests to PostgREST handler
		// Extract table name from path (e.g., /users -> users, /posts -> posts)
		path := strings.Trim(r.URL.Path, "/")
		if path == "" {
			// Root path - return API info
			handleRoot(w, r)
			return
		}

		// Check if this is a table request (no additional path segments)
		pathParts := strings.Split(path, "/")
		if len(pathParts) == 1 {
			// This is a table request - route to handler
			router.HandleTable(w, r)
			return
		}

		// For now, handle additional path segments as table requests
		// In the future, this could be extended for more complex routing
		router.HandleTable(w, r)
	})

	port := ":3002"
	log.Printf("Starting server on port %s", port)
	log.Printf("PostgREST API available at http://localhost%s", port)
	log.Printf("Supported endpoints:")
	log.Printf("  GET    /{table}     - Select data from table")
	log.Printf("  POST   /{table}     - Insert data into table")
	log.Printf("  PATCH  /{table}     - Update data in table")
	log.Printf("  DELETE /{table}     - Delete data from table")
	log.Printf("  OPTIONS /{table}    - CORS preflight")
	log.Fatal(http.ListenAndServe(port, mux))
}

// handleRoot handles requests to the root path and returns API information
func handleRoot(w http.ResponseWriter, r *http.Request) {
	apiInfo := map[string]interface{}{
		"name":        "PostgREST Server",
		"version":     "1.0.0",
		"description": "A PostgREST-compatible API server built in Go",
		"endpoints": map[string]string{
			"GET":    "Select data from table",
			"POST":   "Insert data into table",
			"PATCH":  "Update data in table",
			"DELETE": "Delete data from table",
		},
		"usage": map[string]string{
			"select": "GET /{table}?select=col1,col2&filter=eq.value",
			"insert": "POST /{table} with JSON body",
			"update": "PATCH /{table}?filter=eq.value with JSON body",
			"delete": "DELETE /{table}?filter=eq.value",
		},
		"operators": []string{
			"eq", "neq", "gt", "gte", "lt", "lte",
			"like", "ilike", "in", "is", "not",
		},
		"logical_operators": []string{"and", "or"},
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiInfo)
}
