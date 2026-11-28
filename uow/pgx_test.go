package uow_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/maadiii/uow"
	"github.com/pashagolub/pgxmock/v5"
)

// ============================================================================
// uow.Do Tests
// ============================================================================

func TestPgxDo_CommitsOnSuccess(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	mock.ExpectBeginTx(pgx.TxOptions{AccessMode: pgx.ReadWrite})
	mock.ExpectExec("INSERT INTO users").
		WithArgs("Alice").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	uow := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository {
		return PgxRepository{tx}
	})

	err = uow.Do(context.Background(), func(ctx context.Context, repo PgxRepository) error {
		return repo.Users().Create("Alice")
	})
	if err != nil {
		t.Fatalf("expected no error on success, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPgxDo_RollsBackOnPanic(t *testing.T) { //nolint:dupl
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	mock.ExpectBeginTx(pgx.TxOptions{AccessMode: pgx.ReadWrite})
	mock.ExpectRollback()

	uow := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository {
		return PgxRepository{tx}
	})

	var recovered any

	func() {
		defer func() { recovered = recover() }()

		_ = uow.Do(context.Background(), func(ctx context.Context, repo PgxRepository) error {
			panic("pgx panic")
		})
	}()

	if recovered != "pgx panic" {
		t.Fatalf("expected the panic to propagate, got %v", recovered)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPgxDo_RollsBackOnError(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	mock.ExpectBeginTx(pgx.TxOptions{AccessMode: pgx.ReadWrite})
	mock.ExpectExec("INSERT INTO users").
		WithArgs("Bob").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectRollback()

	uow := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository {
		return PgxRepository{tx}
	})

	err = uow.Do(context.Background(), func(ctx context.Context, repo PgxRepository) error {
		if err = repo.Users().Create("Bob"); err != nil {
			return err
		}

		return errors.New("force rollback")
	})

	if err == nil {
		t.Fatalf("expected error to propagate, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPgxDo_PropagatesErrorFromFunction(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	mock.ExpectBeginTx(pgx.TxOptions{AccessMode: pgx.ReadWrite})
	mock.ExpectRollback()

	uow := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository {
		return PgxRepository{tx}
	})

	expectedErr := errors.New("test error")
	err = uow.Do(context.Background(), func(ctx context.Context, repo PgxRepository) error {
		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPgxDo_MultipleOperationsWithinTransaction(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	mock.ExpectBeginTx(pgx.TxOptions{AccessMode: pgx.ReadWrite})
	mock.ExpectExec("INSERT INTO users").WithArgs("User1").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO users").WithArgs("User2").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO users").WithArgs("User3").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	uow := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository {
		return PgxRepository{tx}
	})

	err = uow.Do(context.Background(), func(ctx context.Context, repo PgxRepository) error {
		if err = repo.Users().Create("User1"); err != nil {
			return err
		}

		if err = repo.Users().Create("User2"); err != nil {
			return err
		}

		if err = repo.Users().Create("User3"); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPgxDo_MultipleOperationsRollbackOnLastError(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	mock.ExpectBeginTx(pgx.TxOptions{AccessMode: pgx.ReadWrite})
	mock.ExpectExec("INSERT INTO users").WithArgs("User1").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO users").WithArgs("User2").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectRollback()

	uow := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	err = uow.Do(context.Background(), func(ctx context.Context, repo PgxRepository) error {
		if err = repo.Users().Create("User1"); err != nil {
			return err
		}

		if err = repo.Users().Create("User2"); err != nil {
			return err
		}

		return errors.New("error on third operation")
	})

	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPgxDo_WithTxOptions(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	mock.ExpectBeginTx(pgx.TxOptions{
		IsoLevel:   pgx.ReadCommitted,
		AccessMode: pgx.ReadWrite,
	})
	mock.ExpectExec("INSERT INTO users").WithArgs("TxOpts").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()
	opts := &uow.TxOptions{
		Isolation: uow.LevelDefault,
		ReadOnly:  false,
	}

	uow := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	err = uow.Do(context.Background(), func(ctx context.Context, repo PgxRepository) error {
		return repo.Users().Create("TxOpts")
	}, opts)
	if err != nil {
		t.Fatalf("expected no error with TxOptions, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPgxDo_WithDifferentIsolationLevels(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		isolation   uow.IsolationLevel
		expectedIso pgx.TxIsoLevel
	}{
		{defaultLevelName, uow.LevelDefault, pgx.ReadCommitted},
		{"ReadUncommitted", uow.LevelReadUncommitted, pgx.ReadCommitted},
		{"ReadCommitted", uow.LevelReadCommitted, pgx.ReadCommitted},
		{"RepeatableRead", uow.LevelRepeatableRead, pgx.RepeatableRead},
		{serializableLevelName, uow.LevelSerializable, pgx.Serializable},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatalf("failed to create mock pool: %v", err)
			}
			defer mock.Close()

			mock.ExpectBeginTx(pgx.TxOptions{
				IsoLevel:   tc.expectedIso,
				AccessMode: pgx.ReadWrite,
			})
			mock.ExpectExec("INSERT INTO users").
				WithArgs(fmt.Sprintf("User%d", i)).
				WillReturnResult(pgxmock.NewResult("INSERT", 1))
			mock.ExpectCommit()
			opts := &uow.TxOptions{
				Isolation: tc.isolation,
				ReadOnly:  false,
			}

			uow := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

			err = uow.Do(context.Background(), func(ctx context.Context, repo PgxRepository) error {
				return repo.Users().Create(fmt.Sprintf("User%d", i))
			}, opts)
			if err != nil {
				t.Fatalf("expected no error with isolation %v, got %v", tc.isolation, err)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestPgxDo_WithReadOnlyTransaction(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	// Read operation should succeed
	mock.ExpectBeginTx(pgx.TxOptions{
		IsoLevel:   pgx.ReadCommitted,
		AccessMode: pgx.ReadOnly,
	})
	mock.ExpectQuery("SELECT name FROM users WHERE id").
		WithArgs(1).
		WillReturnRows(pgxmock.NewRows([]string{"name"}).AddRow("ExistingUser"))
	mock.ExpectCommit()

	opts := &uow.TxOptions{
		ReadOnly: true,
	}
	uow1 := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	// Try to read in read-only transaction - should succeed
	err = uow1.Do(context.Background(), func(ctx context.Context, repo PgxRepository) error {
		name, err := repo.Users().Get(1)
		if err != nil {
			return err
		}
		if name != "ExistingUser" {
			return fmt.Errorf("expected 'ExistingUser', got %q", name)
		}

		return nil
	}, opts)
	if err != nil {
		t.Fatalf("expected no error in read-only transaction, got %v", err)
	}

	if err = mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}

	// Write operation should fail
	mock2, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock2.Close()

	mock2.ExpectBeginTx(pgx.TxOptions{
		IsoLevel:   pgx.ReadCommitted,
		AccessMode: pgx.ReadOnly,
	})
	mock2.ExpectExec("INSERT INTO users").
		WithArgs("NewUser").
		WillReturnError(errors.New("cannot execute INSERT in a read-only transaction"))
	mock2.ExpectRollback()

	uow2 := uow.NewPgx(mock2, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	err = uow2.Do(context.Background(), func(ctx context.Context, repo PgxRepository) error {
		return repo.Users().Create("NewUser")
	}, opts)

	if err == nil {
		t.Fatalf("expected error when writing in read-only transaction, got nil")
	}

	if err := mock2.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPgxDo_ContextCancel(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	mock.ExpectBeginTx(pgx.TxOptions{AccessMode: pgx.ReadWrite}).
		WillReturnError(context.Canceled)

	uow := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	err = uow.Do(ctx, func(ctx context.Context, repo PgxRepository) error {
		return repo.Users().Create("Cancelled")
	})

	// Should get context cancelled error
	if err == nil {
		t.Fatalf("expected error for cancelled context")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ============================================================================
// uowPgx.Begin() Tests
// ============================================================================

func TestPgxBegin_ReturnsContextAndRepoFactory(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	mock.ExpectBeginTx(pgx.TxOptions{AccessMode: pgx.ReadWrite})

	uow := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	ctx, repo, err := uow.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}

	if ctx == nil {
		t.Fatalf("expected non-nil context, got nil")
	}

	if repo == (PgxRepository{}) {
		t.Fatalf("expected non-empty repo factory")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPgxBegin_ContextContainsTx(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	mock.ExpectBeginTx(pgx.TxOptions{AccessMode: pgx.ReadWrite})

	uow1 := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	ctx, _, err := uow1.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}

	tx := ctx.Value(uow.CtxTxKey{})
	if tx == nil {
		t.Fatalf("expected transaction in context")
	}

	// Verify it's the correct type
	if _, ok := tx.(pgx.Tx); !ok {
		t.Fatalf("expected pgx.Tx in context, got %T", tx)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPgxBegin_WithTxOptions(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	mock.ExpectBeginTx(pgx.TxOptions{
		IsoLevel:   pgx.Serializable,
		AccessMode: pgx.ReadWrite,
	})
	mock.ExpectExec("INSERT INTO users").WithArgs("TxOptsUser").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()
	opts := &uow.TxOptions{
		Isolation: uow.LevelSerializable,
		ReadOnly:  false,
	}

	uow := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	ctx, repo, err := uow.Begin(context.Background(), opts)
	if err != nil {
		t.Fatalf("Begin with TxOptions returned error: %v", err)
	}

	if err := repo.Users().Create("TxOptsUser"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := uow.Commit(ctx); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ============================================================================
// uowPgx.Commit() Tests
// ============================================================================

func TestPgxCommit_CommitsTransaction(t *testing.T) { //nolint
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	mock.ExpectBeginTx(pgx.TxOptions{AccessMode: pgx.ReadWrite})
	mock.ExpectExec("INSERT INTO users").WithArgs("CommitTest").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	uow := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	ctx, repo, err := uow.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}

	if err := repo.Users().Create("CommitTest"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := uow.Commit(ctx); err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPgxCommit_ErrorWhenNoTransactionInContext(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	uow1 := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	err = uow1.Commit(context.Background())
	if err == nil {
		t.Fatalf("expected error when no transaction in context, got nil")
	}

	if !errors.Is(err, uow.ErrTransactionNotFound) {
		if err.Error() != "no transaction found in context" { //nolint
			t.Fatalf("expected 'no transaction found in context' error, got %v", err)
		}
	}
}

func TestPgxCommit_ErrorWhenTransactionIsNil(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	uow1 := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	// Manually create context with nil transaction
	ctx := context.WithValue(context.Background(), uow.CtxTxKey{}, pgx.Tx(nil))

	err = uow1.Commit(ctx)
	if err == nil {
		t.Fatalf("expected error when transaction is nil, got nil")
	}
}

// ============================================================================
// uowPgx.Rollback() Tests
// ============================================================================

func TestPgxRollback_RollsBackTransaction(t *testing.T) { //nolint:dupl
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	mock.ExpectBeginTx(pgx.TxOptions{AccessMode: pgx.ReadWrite})
	mock.ExpectExec("INSERT INTO users").WithArgs("RollbackTest").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectRollback()

	uow := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	ctx, repo, err := uow.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}

	if err := repo.Users().Create("RollbackTest"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := uow.Rollback(ctx); err != nil {
		t.Fatalf("Rollback returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPgxRollback_ErrorWhenNoTransactionInContext(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	uow := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	err = uow.Rollback(context.Background())
	if err == nil {
		t.Fatalf("expected error when no transaction in context, got nil")
	}

	if err.Error() != "no transaction found in context" {
		t.Fatalf("expected 'no transaction found in context' error, got %v", err)
	}
}

func TestPgxRollback_ErrorWhenTransactionIsNil(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	uow1 := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	ctx := context.WithValue(context.Background(), uow.CtxTxKey{}, pgx.Tx(nil))

	err = uow1.Rollback(ctx)
	if err == nil {
		t.Fatalf("expected error when transaction is nil, got nil")
	}
}

// ============================================================================
// uowPgx.SavePoint() Tests
// ============================================================================

func TestPgxSavePoint_CreatesSavePoint(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	mock.ExpectBeginTx(pgx.TxOptions{AccessMode: pgx.ReadWrite})
	mock.ExpectExec("INSERT INTO users").WithArgs("User1").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("SAVEPOINT \"sp1\"").WillReturnResult(pgxmock.NewResult("SAVEPOINT", 0))
	mock.ExpectExec("INSERT INTO users").WithArgs("User2").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	uow := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	ctx, repo, err := uow.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}

	if err := repo.Users().Create("User1"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := uow.SavePoint(ctx, "sp1"); err != nil {
		t.Fatalf("SavePoint returned error: %v", err)
	}

	// Continue with more operations
	if err := repo.Users().Create("User2"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := uow.Commit(ctx); err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPgxSavePoint_ErrorWhenNoTransactionInContext(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	uow := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	err = uow.SavePoint(context.Background(), "sp1")
	if err == nil {
		t.Fatalf("expected error when no transaction in context, got nil")
	}

	if err.Error() != "no transaction found in context" {
		t.Fatalf("expected 'no transaction found in context' error, got %v", err)
	}
}

func TestPgxSavePoint_ErrorWhenTransactionIsNil(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	uow1 := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	ctx := context.WithValue(context.Background(), uow.CtxTxKey{}, pgx.Tx(nil))

	err = uow1.SavePoint(ctx, "sp1")
	if err == nil {
		t.Fatalf("expected error when transaction is nil, got nil")
	}
}

func TestPgxSavePoint_MultipleSavePoints(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	mock.ExpectBeginTx(pgx.TxOptions{AccessMode: pgx.ReadWrite})
	mock.ExpectExec("INSERT INTO users").WithArgs("User1").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("SAVEPOINT \"sp1\"").WillReturnResult(pgxmock.NewResult("SAVEPOINT", 0))
	mock.ExpectExec("INSERT INTO users").WithArgs("User2").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("SAVEPOINT \"sp2\"").WillReturnResult(pgxmock.NewResult("SAVEPOINT", 0))
	mock.ExpectExec("INSERT INTO users").WithArgs("User3").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	uow := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	ctx, repo, err := uow.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}

	if err := repo.Users().Create("User1"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := uow.SavePoint(ctx, "sp1"); err != nil {
		t.Fatalf("SavePoint sp1 returned error: %v", err)
	}

	if err := repo.Users().Create("User2"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := uow.SavePoint(ctx, "sp2"); err != nil {
		t.Fatalf("SavePoint sp2 returned error: %v", err)
	}

	if err := repo.Users().Create("User3"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := uow.Commit(ctx); err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ============================================================================
// Integration Tests: Begin/Commit/Rollback Flow
// ============================================================================

func TestPgxIntegration_BeginCommitRollbackFlow(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	// First transaction: successful
	mock.ExpectBeginTx(pgx.TxOptions{AccessMode: pgx.ReadWrite})
	mock.ExpectExec("INSERT INTO users").WithArgs("Alice").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	// Second transaction: rollback
	mock.ExpectBeginTx(pgx.TxOptions{AccessMode: pgx.ReadWrite})
	mock.ExpectExec("INSERT INTO users").WithArgs("Bob").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectRollback()

	uow := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	// First transaction: successful
	ctx1, repo1, err := uow.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}

	if err = repo1.Users().Create("Alice"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err = uow.Commit(ctx1); err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	// Second transaction: rollback
	ctx2, repo2, err := uow.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}

	if err := repo2.Users().Create("Bob"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := uow.Rollback(ctx2); err != nil {
		t.Fatalf("Rollback returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPgxIntegration_NestedSavePointAndRollback(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	mock.ExpectBeginTx(pgx.TxOptions{AccessMode: pgx.ReadWrite})
	mock.ExpectExec("INSERT INTO users").WithArgs("User1").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("SAVEPOINT \"sp1\"").WillReturnResult(pgxmock.NewResult("SAVEPOINT", 0))
	mock.ExpectExec("INSERT INTO users").WithArgs("User2").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("SAVEPOINT \"sp2\"").WillReturnResult(pgxmock.NewResult("SAVEPOINT", 0))
	mock.ExpectCommit()

	uow := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	ctx, repo, err := uow.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}

	// Create first user
	if err := repo.Users().Create("User1"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Create savepoint
	if err := uow.SavePoint(ctx, "sp1"); err != nil {
		t.Fatalf("SavePoint returned error: %v", err)
	}

	// Create second user
	if err := repo.Users().Create("User2"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Create another savepoint
	if err := uow.SavePoint(ctx, "sp2"); err != nil {
		t.Fatalf("SavePoint returned error: %v", err)
	}

	if err := uow.Commit(ctx); err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPgxIntegration_DoVsManualBeginCommit_ProducesSameResult(t *testing.T) { //nolint
	t.Parallel()

	// Test with Do
	mock1, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock1.Close()

	mock1.ExpectBeginTx(pgx.TxOptions{AccessMode: pgx.ReadWrite})
	mock1.ExpectExec("INSERT INTO users").WithArgs("User1").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock1.ExpectExec("INSERT INTO users").WithArgs("User2").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock1.ExpectCommit()

	uow1 := uow.NewPgx(mock1, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	err1 := uow1.Do(context.Background(), func(ctx context.Context, repo PgxRepository) error {
		if err = repo.Users().Create("User1"); err != nil {
			return err
		}
		if err := repo.Users().Create("User2"); err != nil {
			return err
		}

		return nil
	})

	if err1 != nil {
		t.Fatalf("Do returned error: %v", err1)
	}

	if err = mock1.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}

	// Test with Begin/Commit
	mock2, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock2.Close()

	mock2.ExpectBeginTx(pgx.TxOptions{AccessMode: pgx.ReadWrite})
	mock2.ExpectExec("INSERT INTO users").WithArgs("User1").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock2.ExpectExec("INSERT INTO users").WithArgs("User2").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock2.ExpectCommit()

	uow2 := uow.NewPgx(mock2, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	ctx, repo, err := uow2.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}

	if err := repo.Users().Create("User1"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := repo.Users().Create("User2"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := uow2.Commit(ctx); err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	if err := mock2.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// ============================================================================
// convertPgxTxOptions Tests
// ============================================================================

func TestConvertPgxTxOptions_NilOptions(t *testing.T) {
	t.Parallel()

	opts := uow.ConvertPgxTxOptions(nil)

	if opts.IsoLevel != "" {
		t.Fatalf("expected empty IsoLevel for nil options, got %v", opts.IsoLevel)
	}

	if opts.AccessMode != pgx.ReadWrite {
		t.Fatalf("expected ReadWrite AccessMode for nil options, got %v", opts.AccessMode)
	}
}

func TestConvertPgxTxOptions_IsolationLevels(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		sqlLevel uow.IsolationLevel
		pgxLevel pgx.TxIsoLevel
	}{
		{uow.LevelDefault, pgx.ReadCommitted},
		{uow.LevelReadUncommitted, pgx.ReadCommitted}, // Postgres doesn't support dirty reads
		{uow.LevelReadCommitted, pgx.ReadCommitted},
		{uow.LevelRepeatableRead, pgx.RepeatableRead},
		{uow.LevelSerializable, pgx.Serializable},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("SQL_%v_to_PGX_%v", tc.sqlLevel, tc.pgxLevel), func(t *testing.T) {
			t.Parallel()

			sqlOpts := &uow.TxOptions{Isolation: tc.sqlLevel}
			pgxOpts := uow.ConvertPgxTxOptions(sqlOpts)

			if pgxOpts.IsoLevel != tc.pgxLevel {
				t.Fatalf("expected %v, got %v", tc.pgxLevel, pgxOpts.IsoLevel)
			}
		})
	}
}

func TestConvertPgxTxOptions_AccessMode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		readOnly   bool
		accessMode pgx.TxAccessMode
	}{
		{"ReadWrite", false, pgx.ReadWrite},
		{"ReadOnly", true, pgx.ReadOnly},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sqlOpts := &uow.TxOptions{ReadOnly: tc.readOnly}
			pgxOpts := uow.ConvertPgxTxOptions(sqlOpts)

			if pgxOpts.AccessMode != tc.accessMode {
				t.Fatalf("expected %v, got %v", tc.accessMode, pgxOpts.AccessMode)
			}
		})
	}
}

func TestChooseAccessMode_NilOptions(t *testing.T) {
	t.Parallel()

	mode := uow.ChooseAccessMode(nil)

	if mode != pgx.ReadWrite {
		t.Fatalf("expected ReadWrite for nil options, got %v", mode)
	}
}

func TestChooseAccessMode_ReadOnly(t *testing.T) {
	t.Parallel()

	opts := &uow.TxOptions{ReadOnly: true}
	mode := uow.ChooseAccessMode(opts)

	if mode != pgx.ReadOnly {
		t.Fatalf("expected ReadOnly, got %v", mode)
	}
}

func TestChooseAccessMode_ReadWrite(t *testing.T) {
	t.Parallel()

	opts := &uow.TxOptions{ReadOnly: false}
	mode := uow.ChooseAccessMode(opts)

	if mode != pgx.ReadWrite {
		t.Fatalf("expected ReadWrite, got %v", mode)
	}
}

func TestPgxBegin_ErrorHandling(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	// Expect BeginTx to return an error
	expectedErr := errors.New("begin transaction failed")
	mock.ExpectBeginTx(pgx.TxOptions{AccessMode: pgx.ReadWrite}).WillReturnError(expectedErr)

	uow := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository { return PgxRepository{tx} })

	_, _, err = uow.Begin(context.Background())
	if err == nil {
		t.Fatalf("expected error from Begin, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestConvertPgxTxOptions_DefaultIsolationLevel(t *testing.T) {
	t.Parallel()

	// Test with an unknown/invalid isolation level to hit the default case
	opts := &uow.TxOptions{
		Isolation: uow.IsolationLevel(999), // Invalid isolation level
		ReadOnly:  false,
	}

	pgxOpts := uow.ConvertPgxTxOptions(opts)

	// Default should be ReadCommitted
	if pgxOpts.IsoLevel != pgx.ReadCommitted {
		t.Fatalf("expected default isolation level ReadCommitted, got %v", pgxOpts.IsoLevel)
	}

	if pgxOpts.AccessMode != pgx.ReadWrite {
		t.Fatalf("expected access mode ReadWrite, got %v", pgxOpts.AccessMode)
	}
}

func TestPgxDo_JoinsRollbackError(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	errWork := errors.New("force rollback")
	errRollback := errors.New("rollback failed")

	mock.ExpectBeginTx(pgx.TxOptions{AccessMode: pgx.ReadWrite})
	mock.ExpectRollback().WillReturnError(errRollback)

	uow1 := uow.NewPgx(mock, func(tx pgx.Tx) PgxRepository {
		return PgxRepository{tx}
	})

	err = uow1.Do(context.Background(), func(context.Context, PgxRepository) error {
		return errWork
	})

	if !errors.Is(err, errWork) {
		t.Errorf("expected the work error in the result, got %v", err)
	}

	if !errors.Is(err, errRollback) {
		t.Errorf("expected the rollback error in the result, got %v", err)
	}
}
