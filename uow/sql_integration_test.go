//nolint:paralleltest // Tests share and truncate one users table.
package uow_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/maadiii/uow"
)

type pgRepository struct {
	tx *sql.Tx
}

func (p pgRepository) Users() pgUserRepo {
	return pgUserRepo(p)
}

type pgUserRepo struct {
	tx *sql.Tx
}

func (p pgUserRepo) Create(ctx context.Context, name string) error {
	_, err := p.tx.ExecContext(ctx, "INSERT INTO users (name) VALUES ($1)", name)

	return err
}

func openSqlIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("pgx", realPostgresDSN(t))
	if err != nil {
		t.Fatalf("failed to open sql db: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	resetUsers(t, db)

	return db
}

func TestSqlIntegrationDo_CommitsOnSuccess(t *testing.T) {
	db := openSqlIntegrationDB(t)

	uow := uow.NewSql(db, func(tx *sql.Tx) pgRepository { return pgRepository{tx} })

	err := uow.Do(context.Background(), func(ctx context.Context, repo pgRepository) error {
		if err := repo.Users().Create(ctx, "Alice"); err != nil {
			return err
		}

		return repo.Users().Create(ctx, "Bob")
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if count := countUsers(t, db); count != 2 {
		t.Fatalf("expected 2 persisted users, got %d", count)
	}
}

func TestSqlIntegrationDo_RollsBackOnError(t *testing.T) {
	db := openSqlIntegrationDB(t)

	uow := uow.NewSql(db, func(tx *sql.Tx) pgRepository { return pgRepository{tx} })

	expectedErr := errors.New("force rollback")
	err := uow.Do(context.Background(), func(ctx context.Context, repo pgRepository) error {
		if err := repo.Users().Create(ctx, "RolledBack"); err != nil {
			return err
		}

		return expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}

	if count := countUsers(t, db); count != 0 {
		t.Fatalf("expected no persisted users after rollback, got %d", count)
	}
}

func TestSqlIntegrationBegin_SavePointAndCommit(t *testing.T) {
	db := openSqlIntegrationDB(t)

	uow := uow.NewSql(db, func(tx *sql.Tx) pgRepository { return pgRepository{tx} })

	ctx, repo, err := uow.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}

	if err := repo.Users().Create(ctx, "BeforeSavePoint"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := uow.SavePoint(ctx, "sp1"); err != nil {
		t.Fatalf("SavePoint returned error: %v", err)
	}

	if err := repo.Users().Create(ctx, "AfterSavePoint"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := uow.Commit(ctx); err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	if count := countUsers(t, db); count != 2 {
		t.Fatalf("expected 2 persisted users, got %d", count)
	}
}

func TestSqlIntegrationBegin_SavePointWithQuotedName(t *testing.T) {
	db := openSqlIntegrationDB(t)

	uow1 := uow.NewSql(db, func(tx *sql.Tx) pgRepository { return pgRepository{tx} })

	ctx, repo, err := uow1.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}

	if err := repo.Users().Create(ctx, "BeforeSavePoint"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// A savepoint name may contain characters that are significant in SQL as
	// long as it is quoted as an identifier.
	if err := uow1.SavePoint(ctx, `sp "odd"; --`); err != nil {
		t.Fatalf("SavePoint with a quoted name returned error: %v", err)
	}

	if err := uow1.Commit(ctx); err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	if count := countUsers(t, db); count != 1 {
		t.Fatalf("expected 1 persisted user, got %d", count)
	}
}

func TestSqlIntegrationBegin_Rollback(t *testing.T) {
	db := openSqlIntegrationDB(t)

	uow := uow.NewSql(db, func(tx *sql.Tx) pgRepository { return pgRepository{tx} })

	ctx, repo, err := uow.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}

	if err := repo.Users().Create(ctx, "Discarded"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := uow.Rollback(ctx); err != nil {
		t.Fatalf("Rollback returned error: %v", err)
	}

	if count := countUsers(t, db); count != 0 {
		t.Fatalf("expected no persisted users after rollback, got %d", count)
	}
}

func TestSqlIntegrationDo_ReadOnlyRejectsWrites(t *testing.T) {
	db := openSqlIntegrationDB(t)

	uow1 := uow.NewSql(db, func(tx *sql.Tx) pgRepository { return pgRepository{tx} })

	err := uow1.Do(context.Background(), func(ctx context.Context, repo pgRepository) error {
		return repo.Users().Create(ctx, "NotAllowed")
	}, &uow.TxOptions{ReadOnly: true})
	if err == nil {
		t.Fatalf("expected a read-only transaction to reject the insert")
	}

	if count := countUsers(t, db); count != 0 {
		t.Fatalf("expected no persisted users in a read-only transaction, got %d", count)
	}
}

func TestSqlIntegrationDo_IsolationOption(t *testing.T) {
	db := openSqlIntegrationDB(t)

	uow1 := uow.NewSql(db, func(tx *sql.Tx) pgRepository { return pgRepository{tx} })

	var isolation string

	err := uow1.Do(context.Background(), func(ctx context.Context, repo pgRepository) error {
		if err := repo.Users().Create(ctx, "Serializable"); err != nil {
			return err
		}

		return repo.tx.QueryRowContext(ctx, "SHOW transaction_isolation").Scan(&isolation)
	}, &uow.TxOptions{Isolation: uow.LevelSerializable})
	if err != nil {
		t.Fatalf("expected no error with serializable isolation, got %v", err)
	}

	if isolation != serializableIsolation {
		t.Fatalf("expected the server to report serializable isolation, got %q", isolation)
	}

	if count := countUsers(t, db); count != 1 {
		t.Fatalf("expected 1 persisted user, got %d", count)
	}
}
