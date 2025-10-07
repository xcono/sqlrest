package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/xcono/sqlrest/e2e/containers"
)

func TestMySQLContainer(t *testing.T) {
	// Skip this test when TestMain is active to avoid port conflicts
	if testContainers != nil {
		t.Skip("Skipping individual MySQL test - TestMain containers are active")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	t.Log("Starting MySQL container...")

	container, dsn, err := containers.SetupMySQL(ctx)
	if err != nil {
		t.Fatalf("Failed to setup MySQL container: %v", err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := container.Terminate(cleanupCtx); err != nil {
			t.Logf("Warning: Failed to cleanup MySQL container: %v", err)
		}
	}()

	t.Logf("MySQL container started successfully")
	t.Logf("MySQL DSN: %s", dsn)

	// Verify the container is running
	if container == nil {
		t.Fatal("MySQL container is nil")
	}

	// Get container info
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get MySQL host: %v", err)
	}
	t.Logf("MySQL host: %s", host)

	mappedPort, err := container.MappedPort(ctx, "3306")
	if err != nil {
		t.Fatalf("Failed to get MySQL mapped port: %v", err)
	}
	t.Logf("MySQL mapped port: %s", mappedPort.Port())

	t.Log("MySQL container test completed successfully")
}
