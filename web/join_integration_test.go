package web

import (
    "encoding/json"
    "net/http"
    "testing"
)

// TestPostgRESTJoinScanning verifies nested object scanning for JOIN embeds
func TestPostgRESTJoinScanning(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    // Create related tables for JOIN testing
    _, err := db.Exec(`
        CREATE TABLE posts (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            users_id INTEGER,
            title TEXT
        );
    `)
    if err != nil {
        t.Fatalf("Failed to create posts table: %v", err)
    }
    _, err = db.Exec(`
        CREATE TABLE comments (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            posts_id INTEGER,
            text TEXT
        );
    `)
    if err != nil {
        t.Fatalf("Failed to create comments table: %v", err)
    }

    // Insert one post and one comment related to Alice (users.id = 1)
    _, err = db.Exec("INSERT INTO posts (users_id, title) VALUES (1, 'Hello World')")
    if err != nil {
        t.Fatalf("Failed to insert post: %v", err)
    }
    _, err = db.Exec("INSERT INTO comments (posts_id, text) VALUES (1, 'Nice post!')")
    if err != nil {
        t.Fatalf("Failed to insert comment: %v", err)
    }

    server := createTestServer(t, db)
    defer server.Close()

    // Query with nested embeds and filter to a single user to avoid duplication
    resp, err := http.Get(server.URL + "/users?select=id,name,posts!left(id,title,comments(text))&name=eq.Alice")
    if err != nil {
        t.Fatalf("Failed to make request: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        t.Fatalf("Expected status 200, got %d", resp.StatusCode)
    }

    // Decode into a generic array of maps
    var result []map[string]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        t.Fatalf("Failed to decode response: %v", err)
    }

    if len(result) != 1 {
        t.Fatalf("Expected 1 result for Alice, got %d", len(result))
    }

    row := result[0]
    if row["name"] != "Alice" {
        t.Fatalf("Expected Alice, got %v", row["name"])
    }

    // Validate nested object structure built by scanner
    postsVal, ok := row["posts"]
    if !ok || postsVal == nil {
        t.Fatalf("Expected nested 'posts' object, missing")
    }
    postsObj, ok := postsVal.(map[string]interface{})
    if !ok {
        t.Fatalf("Expected nested 'posts' object as map, got: %T", postsVal)
    }
    if postsObj["id"] == nil || postsObj["title"] != "Hello World" {
        t.Fatalf("Unexpected posts object: %+v", postsObj)
    }

    commentsVal, ok := postsObj["comments"]
    if !ok || commentsVal == nil {
        t.Fatalf("Expected nested 'comments' object, missing")
    }
    commentsObj, ok := commentsVal.(map[string]interface{})
    if !ok {
        t.Fatalf("Expected nested 'comments' object as map, got: %T", commentsVal)
    }
    if commentsObj["text"] != "Nice post!" {
        t.Fatalf("Unexpected comments object: %+v", commentsObj)
    }
}
