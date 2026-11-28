package uow_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	testpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	// postgresImage is the real database pulled by testcontainers for the integration tests.
	postgresImage = "postgres:16-alpine"

	createUsersTableSQL = `CREATE TABLE IF NOT EXISTS users (id bigserial PRIMARY KEY, name text NOT NULL UNIQUE)`

	truncateUsersSQL = "TRUNCATE users RESTART IDENTITY"

	// serializableIsolation is how Postgres reports SHOW transaction_isolation.
	serializableIsolation = "serializable"
)

var (
	postgresOnce sync.Once
	postgresDSN  string
	postgresErr  error
)

// realPostgresDSN starts a single Postgres container for the whole test run and
// returns its DSN. The container is reaped by testcontainers when the test
// process exits.
func realPostgresDSN(t *testing.T) string {
	t.Helper()

	testcontainers.SkipIfProviderIsNotHealthy(t)

	postgresOnce.Do(func() {
		ctx := context.Background()

		container, err := testpostgres.Run(
			ctx,
			postgresImage,
			testpostgres.WithDatabase("uow"),
			testpostgres.WithUsername("uow"),
			testpostgres.WithPassword("uow"),
			testpostgres.BasicWaitStrategies(),
		)
		if err != nil {
			postgresErr = fmt.Errorf("start postgres container: %w", err)

			return
		}

		postgresDSN, postgresErr = container.ConnectionString(ctx, "sslmode=disable")
	})

	if postgresErr != nil {
		t.Fatalf("postgres container: %v", postgresErr)
	}

	return postgresDSN
}

// resetUsers creates the users table if needed and empties it so every test
// starts from a known state.
func resetUsers(t *testing.T, db *sql.DB) {
	t.Helper()

	if _, err := db.ExecContext(context.Background(), createUsersTableSQL); err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}

	if _, err := db.ExecContext(context.Background(), truncateUsersSQL); err != nil {
		t.Fatalf("failed to truncate users table: %v", err)
	}
}

func countUsers(t *testing.T, db *sql.DB) int {
	t.Helper()

	var count int
	if err := db.QueryRowContext(context.Background(), "SELECT count(*) FROM users").Scan(&count); err != nil {
		t.Fatalf("failed to count users: %v", err)
	}

	return count
}
