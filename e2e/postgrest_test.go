package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/xcono/sqlrest/e2e/containers"
)

func TestPostgRESTContainer(t *testing.T) {
	// Skip this test when TestMain is active to avoid port conflicts
	if testContainers != nil {
		t.Skip("Skipping individual PostgREST test - TestMain containers are active")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Create a network first with the same name as used in containers
	t.Log("Creating test network...")
	network, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{
			Name: "e2e-test-network",
		},
	})
	if err != nil {
		t.Fatalf("Failed to create network: %v", err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := network.Remove(cleanupCtx); err != nil {
			t.Logf("Warning: Failed to remove network: %v", err)
		}
	}()

	// Start PostgreSQL first with network
	t.Log("Starting PostgreSQL container...")
	postgresContainer, postgresDSN, err := containers.SetupPostgresWithNetwork(ctx, network)
	if err != nil {
		t.Fatalf("Failed to setup PostgreSQL container: %v", err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := postgresContainer.Terminate(cleanupCtx); err != nil {
			t.Logf("Warning: Failed to cleanup PostgreSQL container: %v", err)
		}
	}()

	t.Logf("PostgreSQL container started successfully")
	t.Logf("PostgreSQL DSN: %s", postgresDSN)

	// Start PostgREST with network
	t.Log("Starting PostgREST container...")
	postgrestContainer, postgrestURL, err := containers.SetupPostgRESTWithNetwork(ctx, network)
	if err != nil {
		t.Fatalf("Failed to setup PostgREST container: %v", err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := postgrestContainer.Terminate(cleanupCtx); err != nil {
			t.Logf("Warning: Failed to cleanup PostgREST container: %v", err)
		}
	}()

	t.Logf("PostgREST container started successfully")
	t.Logf("PostgREST URL: %s", postgrestURL)

	// Verify the containers are running
	if postgresContainer == nil {
		t.Fatal("PostgreSQL container is nil")
	}
	if postgrestContainer == nil {
		t.Fatal("PostgREST container is nil")
	}

	// Get container info
	host, err := postgrestContainer.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get PostgREST host: %v", err)
	}
	t.Logf("PostgREST host: %s", host)

	mappedPort, err := postgrestContainer.MappedPort(ctx, "3000")
	if err != nil {
		t.Fatalf("Failed to get PostgREST mapped port: %v", err)
	}
	t.Logf("PostgREST mapped port: %s", mappedPort.Port())

	t.Log("PostgREST container test completed successfully")
}
