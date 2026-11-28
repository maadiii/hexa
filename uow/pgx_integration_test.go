//nolint:paralleltest // Tests share and truncate one users table.
package uow_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maadiii/uow"
)

func openPgxIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), realPostgresDSN(t))
	if err != nil {
		t.Fatalf("failed to open pgx pool: %v", err)
	}

	t.Cleanup(pool.Close)

	if _, err := pool.Exec(context.Background(), createUsersTableSQL); err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}

	if _, err := pool.Exec(context.Background(), truncateUsersSQL); err != nil {
		t.Fatalf("failed to truncate users table: %v", err)
	}

	return pool
}

func countPgxUsers(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()

	var count int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM users").Scan(&count); err != nil {
		t.Fatalf("failed to count users: %v", err)
	}

	return count
}

func TestPgxIntegrationDo_CommitsOnSuccess(t *testing.T) {
	pool := openPgxIntegrationPool(t)

	uow := uow.NewPgx(pool, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	err := uow.Do(context.Background(), func(ctx context.Context, repo PgxRepository) error {
		if err := repo.Users().Create("Alice"); err != nil {
			return err
		}

		return repo.Users().Create("Bob")
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if count := countPgxUsers(t, pool); count != 2 {
		t.Fatalf("expected 2 persisted users, got %d", count)
	}

	name, err := PgxRepository{pool}.Users().Get(1)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if name != "Alice" {
		t.Fatalf("expected first user to be Alice, got %q", name)
	}
}

func TestPgxIntegrationDo_RollsBackOnError(t *testing.T) {
	pool := openPgxIntegrationPool(t)

	uow := uow.NewPgx(pool, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	expectedErr := errors.New("force rollback")
	err := uow.Do(context.Background(), func(ctx context.Context, repo PgxRepository) error {
		if err := repo.Users().Create("RolledBack"); err != nil {
			return err
		}

		return expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}

	if count := countPgxUsers(t, pool); count != 0 {
		t.Fatalf("expected no persisted users after rollback, got %d", count)
	}
}

func TestPgxIntegrationBegin_SavePointAndCommit(t *testing.T) {
	pool := openPgxIntegrationPool(t)

	uow := uow.NewPgx(pool, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	ctx, repo, err := uow.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}

	if err := repo.Users().Create("BeforeSavePoint"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := uow.SavePoint(ctx, "sp1"); err != nil {
		t.Fatalf("SavePoint returned error: %v", err)
	}

	if err := repo.Users().Create("AfterSavePoint"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := uow.Commit(ctx); err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	if count := countPgxUsers(t, pool); count != 2 {
		t.Fatalf("expected 2 persisted users, got %d", count)
	}
}

func TestPgxIntegrationBegin_Rollback(t *testing.T) {
	pool := openPgxIntegrationPool(t)

	uow := uow.NewPgx(pool, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	ctx, repo, err := uow.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}

	if err := repo.Users().Create("Discarded"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := uow.Rollback(ctx); err != nil {
		t.Fatalf("Rollback returned error: %v", err)
	}

	if count := countPgxUsers(t, pool); count != 0 {
		t.Fatalf("expected no persisted users after rollback, got %d", count)
	}
}

func TestPgxIntegrationDo_IsolationReachesServer(t *testing.T) {
	pool := openPgxIntegrationPool(t)

	uow1 := uow.NewPgx(pool, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	var isolation string

	err := uow1.Do(context.Background(), func(ctx context.Context, repo PgxRepository) error {
		return repo.pool.QueryRow(ctx, "SHOW transaction_isolation").Scan(&isolation)
	}, &uow.TxOptions{Isolation: uow.LevelSerializable})
	if err != nil {
		t.Fatalf("expected no error with serializable isolation, got %v", err)
	}

	if isolation != serializableIsolation {
		t.Fatalf("expected the server to report serializable isolation, got %q", isolation)
	}
}

func TestPgxIntegrationDo_ReadOnlyRejectsWrites(t *testing.T) {
	pool := openPgxIntegrationPool(t)

	uow1 := uow.NewPgx(pool, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	err := uow1.Do(context.Background(), func(ctx context.Context, repo PgxRepository) error {
		return repo.Users().Create("NotAllowed")
	}, &uow.TxOptions{ReadOnly: true})
	if err == nil {
		t.Fatalf("expected a read-only transaction to reject the insert")
	}

	if count := countPgxUsers(t, pool); count != 0 {
		t.Fatalf("expected no persisted users in a read-only transaction, got %d", count)
	}
}

func TestPgxIntegrationDo_IsolationOption(t *testing.T) {
	pool := openPgxIntegrationPool(t)

	uow1 := uow.NewPgx(pool, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	err := uow1.Do(context.Background(), func(ctx context.Context, repo PgxRepository) error {
		return repo.Users().Create("Serializable")
	}, &uow.TxOptions{Isolation: uow.LevelSerializable})
	if err != nil {
		t.Fatalf("expected no error with serializable isolation, got %v", err)
	}

	if count := countPgxUsers(t, pool); count != 1 {
		t.Fatalf("expected 1 persisted user, got %d", count)
	}
}
