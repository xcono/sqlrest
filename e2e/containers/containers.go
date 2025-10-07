package containers

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestContainers holds all container references and connection details
type TestContainers struct {
	MySQLContainer     testcontainers.Container
	PostgresContainer  testcontainers.Container
	PostgRESTContainer testcontainers.Container
	Network            testcontainers.Network

	MySQLDSN     string
	PostgresDSN  string
	PostgRESTURL string
}

// SetupMySQL creates and starts a MySQL container with the test database
func SetupMySQL(ctx context.Context) (testcontainers.Container, string, error) {
	// Get the absolute path to the migration file
	migrationPath, err := filepath.Abs("migrations/my/simple_mysql.sql")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get migration file path: %w", err)
	}

	// Use non-standard port to avoid conflicts
	hostConfigModifier := func(hostConfig *container.HostConfig) {
		hostConfig.PortBindings = nat.PortMap{
			"3306/tcp": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: "13306", // Non-standard port
				},
			},
		}
	}

	container, err := mysql.RunContainer(ctx,
		testcontainers.WithImage("mysql:8.4"),
		mysql.WithDatabase("chinook_auto_increment"),
		mysql.WithUsername("testuser"),
		mysql.WithPassword("testpass"),
		mysql.WithScripts(migrationPath),
		testcontainers.WithWaitStrategy(
			wait.ForLog("ready for connections").
				WithOccurrence(1).
				WithStartupTimeout(60*time.Second),
		),
		testcontainers.WithHostConfigModifier(hostConfigModifier),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start MySQL container: %w", err)
	}

	// Get connection string using the mapped port
	mappedPort, err := container.MappedPort(ctx, "3306")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get MySQL mapped port: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get MySQL host: %w", err)
	}

	dsn := fmt.Sprintf("testuser:testpass@tcp(%s:%s)/chinook_auto_increment?parseTime=true&loc=Local", host, mappedPort.Port())

	return container, dsn, nil
}

// SetupPostgres creates and starts a PostgreSQL container with the test database
func SetupPostgres(ctx context.Context, network testcontainers.Network) (testcontainers.Container, string, error) {
	return setupPostgresInternal(ctx, network, false)
}

// SetupPostgresWithNetwork creates and starts a PostgreSQL container with network alias
func SetupPostgresWithNetwork(ctx context.Context, network testcontainers.Network) (testcontainers.Container, string, error) {
	return setupPostgresInternal(ctx, network, true)
}

// setupPostgresInternal is the internal implementation for PostgreSQL setup
func setupPostgresInternal(ctx context.Context, network testcontainers.Network, withNetworkAlias bool) (testcontainers.Container, string, error) {
	// Get the absolute path to the migration file
	migrationPath, err := filepath.Abs("migrations/pg/simple_postgres.sql")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get migration file path: %w", err)
	}

	// Use non-standard port to avoid conflicts
	hostConfigModifier := func(hostConfig *container.HostConfig) {
		hostConfig.PortBindings = nat.PortMap{
			"5432/tcp": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: "15432", // Non-standard port
				},
			},
		}
	}

	var container testcontainers.Container
	if withNetworkAlias {
		// Use GenericContainer with proper NetworkAliases configuration for PostgREST connectivity
		container, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        "postgres:17.5",
				ExposedPorts: []string{"5432/tcp"},
				Env: map[string]string{
					"POSTGRES_USER":     "postgres",
					"POSTGRES_PASSWORD": "postgres",
					"POSTGRES_DB":       "chinook_auto_increment",
				},
				Files: []testcontainers.ContainerFile{
					{
						HostFilePath:      migrationPath,
						ContainerFilePath: "/docker-entrypoint-initdb.d/init.sql",
						FileMode:          0644,
					},
				},
				Networks: []string{"e2e-test-network"},
				NetworkAliases: map[string][]string{
					"e2e-test-network": {"postgres"},
				},
				WaitingFor: wait.ForAll(
					wait.ForLog("database system is ready to accept connections").
						WithOccurrence(1).
						WithStartupTimeout(60*time.Second),
					wait.ForListeningPort("5432/tcp"),
				),
			},
			Started: true,
		})
	} else {
		// Standard setup without network alias
		container, err = postgres.Run(ctx, "postgres:17.5",
			postgres.WithDatabase("chinook_auto_increment"),
			postgres.WithUsername("postgres"),
			postgres.WithPassword("postgres"),
			postgres.WithInitScripts(migrationPath),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(1).
					WithStartupTimeout(60*time.Second),
			),
			testcontainers.WithHostConfigModifier(hostConfigModifier),
		)
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed to start PostgreSQL container: %w", err)
	}

	// Get connection string using the mapped port
	mappedPort, err := container.MappedPort(ctx, "5432")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get PostgreSQL mapped port: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get PostgreSQL host: %w", err)
	}

	dsn := fmt.Sprintf("postgres://postgres:postgres@%s:%s/chinook_auto_increment?sslmode=disable", host, mappedPort.Port())

	return container, dsn, nil
}

// SetupPostgREST creates and starts a PostgREST container connected to PostgreSQL
func SetupPostgREST(ctx context.Context, network testcontainers.Network) (testcontainers.Container, string, error) {
	return setupPostgRESTInternal(ctx, network, false)
}

// SetupPostgRESTWithNetwork creates and starts a PostgREST container with proper networking
func SetupPostgRESTWithNetwork(ctx context.Context, network testcontainers.Network) (testcontainers.Container, string, error) {
	return setupPostgRESTInternal(ctx, network, true)
}

// setupPostgRESTInternal is the internal implementation for PostgREST setup
func setupPostgRESTInternal(ctx context.Context, network testcontainers.Network, withNetwork bool) (testcontainers.Container, string, error) {
	var container testcontainers.Container
	var err error

	if withNetwork {
		// Use network with proper service discovery - connect to PostgreSQL via network alias
		dbURI := "postgres://postgres:postgres@postgres:5432/chinook_auto_increment"

		container, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        "postgrest/postgrest:v12.2.3",
				ExposedPorts: []string{"3000/tcp"},
				Env: map[string]string{
					"PGRST_DB_URI":                   dbURI,
					"PGRST_DB_ANON_ROLE":             "postgres",
					"PGRST_DB_SCHEMAS":               "public",
					"PGRST_OPENAPI_SERVER_PROXY_URI": "http://localhost:3000",
				},
				Networks: []string{"e2e-test-network"},
				NetworkAliases: map[string][]string{
					"e2e-test-network": {"postgrest"},
				},
				WaitingFor: wait.ForHTTP("/").
					WithPort("3000/tcp").
					WithStartupTimeout(60 * time.Second),
			},
			Started: true,
		})
	} else {
		// Standard setup without network
		dbURI := "postgres://postgres:postgres@postgres:5432/chinook_auto_increment"

		container, err = testcontainers.Run(ctx, "postgrest/postgrest:v12.2.3",
			testcontainers.WithExposedPorts("3000/tcp"),
			testcontainers.WithEnv(map[string]string{
				"PGRST_DB_URI":                   dbURI,
				"PGRST_DB_ANON_ROLE":             "postgres",
				"PGRST_DB_SCHEMAS":               "public",
				"PGRST_OPENAPI_SERVER_PROXY_URI": "http://localhost:13000",
			}),
			testcontainers.WithWaitStrategy(
				wait.ForHTTP("/").
					WithPort("3000/tcp").
					WithStartupTimeout(60*time.Second),
			),
		)
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed to start PostgREST container: %w", err)
	}

	// Get the mapped port for PostgREST
	mappedPort, err := container.MappedPort(ctx, "3000")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get PostgREST mapped port: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get PostgREST host: %w", err)
	}

	postgrestURL := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())

	return container, postgrestURL, nil
}

// SetupAllContainers creates and starts all required containers for testing
func SetupAllContainers(ctx context.Context) (*TestContainers, error) {
	tc := &TestContainers{}

	// Create a custom network for PostgreSQL and PostgREST communication
	network, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{
			Name: "e2e-test-network",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create network: %w", err)
	}
	tc.Network = network

	// Setup MySQL
	mysqlContainer, mysqlDSN, err := SetupMySQL(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to setup MySQL: %w", err)
	}
	tc.MySQLContainer = mysqlContainer
	tc.MySQLDSN = mysqlDSN

	// Setup PostgreSQL with network aliases for PostgREST connectivity
	postgresContainer, postgresDSN, err := SetupPostgresWithNetwork(ctx, network)
	if err != nil {
		return nil, fmt.Errorf("failed to setup PostgreSQL: %w", err)
	}
	tc.PostgresContainer = postgresContainer
	tc.PostgresDSN = postgresDSN

	// Setup PostgREST with network for PostgreSQL connectivity
	postgrestContainer, postgrestURL, err := SetupPostgRESTWithNetwork(ctx, network)
	if err != nil {
		return nil, fmt.Errorf("failed to setup PostgREST: %w", err)
	}
	tc.PostgRESTContainer = postgrestContainer
	tc.PostgRESTURL = postgrestURL

	return tc, nil
}

// Cleanup terminates all containers and network
func (tc *TestContainers) Cleanup(ctx context.Context) error {
	var lastErr error

	if tc.PostgRESTContainer != nil {
		if err := tc.PostgRESTContainer.Terminate(ctx); err != nil {
			lastErr = fmt.Errorf("failed to terminate PostgREST container: %w", err)
		}
	}

	if tc.PostgresContainer != nil {
		if err := tc.PostgresContainer.Terminate(ctx); err != nil {
			lastErr = fmt.Errorf("failed to terminate PostgreSQL container: %w", err)
		}
	}

	if tc.MySQLContainer != nil {
		if err := tc.MySQLContainer.Terminate(ctx); err != nil {
			lastErr = fmt.Errorf("failed to terminate MySQL container: %w", err)
		}
	}

	if tc.Network != nil {
		if err := tc.Network.Remove(ctx); err != nil {
			lastErr = fmt.Errorf("failed to remove network: %w", err)
		}
	}

	return lastErr
}
