package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/xcono/sqlrest/e2e/containers"
)

func TestPostgresContainer(t *testing.T) {
	// Skip this test when TestMain is active to avoid port conflicts
	if testContainers != nil {
		t.Skip("Skipping individual PostgreSQL test - TestMain containers are active")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Create a network first
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

	t.Log("Starting PostgreSQL container...")

	container, dsn, err := containers.SetupPostgres(ctx, network)
	if err != nil {
		t.Fatalf("Failed to setup PostgreSQL container: %v", err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := container.Terminate(cleanupCtx); err != nil {
			t.Logf("Warning: Failed to cleanup PostgreSQL container: %v", err)
		}
	}()

	t.Logf("PostgreSQL container started successfully")
	t.Logf("PostgreSQL DSN: %s", dsn)

	// Verify the container is running
	if container == nil {
		t.Fatal("PostgreSQL container is nil")
	}

	// Get container info
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get PostgreSQL host: %v", err)
	}
	t.Logf("PostgreSQL host: %s", host)

	mappedPort, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("Failed to get PostgreSQL mapped port: %v", err)
	}
	t.Logf("PostgreSQL mapped port: %s", mappedPort.Port())

	t.Log("PostgreSQL container test completed successfully")
}
