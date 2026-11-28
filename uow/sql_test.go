package uow_test

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/maadiii/uow"
)

// ============================================================================
// uow.Do Tests
// ============================================================================

func TestSqlDo_CommitsOnSuccess(t *testing.T) {
	t.Parallel()

	mock, db := newSqlMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").
		WithArgs("Alice").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	uow := uow.NewSql(db, func(tx *sql.Tx) SqlRepository {
		return SqlRepository{tx}
	})

	err := uow.Do(context.Background(), func(ctx context.Context, repo SqlRepository) error {
		return repo.Users().Create("Alice")
	})
	if err != nil {
		t.Fatalf("expected no error on success, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSqlDo_RollsBackOnError(t *testing.T) {
	t.Parallel()

	mock, db := newSqlMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").
		WithArgs("Bob").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectRollback()

	uow := uow.NewSql(db, func(tx *sql.Tx) SqlRepository {
		return SqlRepository{tx}
	})

	err := uow.Do(context.Background(), func(ctx context.Context, repo SqlRepository) error {
		if err := repo.Users().Create("Bob"); err != nil {
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

func TestSqlDo_RollsBackOnPanic(t *testing.T) { //nolint:dupl
	t.Parallel()

	mock, db := newSqlMock(t)

	mock.ExpectBegin()
	mock.ExpectRollback()

	uow := uow.NewSql(db, func(tx *sql.Tx) SqlRepository {
		return SqlRepository{tx}
	})

	var recovered any

	func() {
		defer func() { recovered = recover() }()

		_ = uow.Do(context.Background(), func(ctx context.Context, repo SqlRepository) error {
			panic("sql panic")
		})
	}()

	if recovered != "sql panic" {
		t.Fatalf("expected the panic to propagate, got %v", recovered)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSqlDo_PropagatesErrorFromFunction(t *testing.T) {
	t.Parallel()

	mock, db := newSqlMock(t)

	mock.ExpectBegin()
	mock.ExpectRollback()

	uow := uow.NewSql(db, func(tx *sql.Tx) SqlRepository {
		return SqlRepository{tx}
	})

	expectedErr := errors.New("test error")
	err := uow.Do(context.Background(), func(ctx context.Context, repo SqlRepository) error {
		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSqlDo_MultipleOperationsWithinTransaction(t *testing.T) { //nolint:dupl
	t.Parallel()

	mock, db := newSqlMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("User1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO users").WithArgs("User2").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec("INSERT INTO users").WithArgs("User3").WillReturnResult(sqlmock.NewResult(3, 1))
	mock.ExpectCommit()

	uow := uow.NewSql(db, func(tx *sql.Tx) SqlRepository {
		return SqlRepository{tx}
	})

	err := uow.Do(context.Background(), func(ctx context.Context, repo SqlRepository) error {
		for _, name := range []string{"User1", "User2", "User3"} {
			if err := repo.Users().Create(name); err != nil {
				return err
			}
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

func TestSqlDo_MultipleOperationsRollbackOnLastError(t *testing.T) {
	t.Parallel()

	mock, db := newSqlMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("User1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO users").WithArgs("User2").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectRollback()

	uow := uow.NewSql(db, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

	err := uow.Do(context.Background(), func(ctx context.Context, repo SqlRepository) error {
		if err := repo.Users().Create("User1"); err != nil {
			return err
		}

		if err := repo.Users().Create("User2"); err != nil {
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

func TestSqlDo_WithTxOptions(t *testing.T) {
	t.Parallel()

	mock, db := newSqlMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("TxOpts").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	opts := &uow.TxOptions{
		Isolation: uow.LevelDefault,
		ReadOnly:  false,
	}

	uow := uow.NewSql(db, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

	err := uow.Do(context.Background(), func(ctx context.Context, repo SqlRepository) error {
		return repo.Users().Create("TxOpts")
	}, opts)
	if err != nil {
		t.Fatalf("expected no error with TxOptions, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSqlDo_ContextCancel(t *testing.T) {
	t.Parallel()

	_, db := newSqlMock(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	uow := uow.NewSql(db, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

	err := uow.Do(ctx, func(ctx context.Context, repo SqlRepository) error {
		return repo.Users().Create("Cancelled")
	})

	// Should get context cancelled error before the transaction ever begins
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// ============================================================================
// uowsql.Begin() Tests
// ============================================================================

func TestSqlBegin_ReturnsContextAndRepoFactory(t *testing.T) {
	t.Parallel()

	mock, db := newSqlMock(t)

	mock.ExpectBegin()

	uow := uow.NewSql(db, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

	ctx, repo, err := uow.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}

	if ctx == nil {
		t.Fatalf("expected non-nil context, got nil")
	}

	if repo == (SqlRepository{}) {
		t.Fatalf("expected non-empty repo factory")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSqlBegin_ContextContainsTx(t *testing.T) {
	t.Parallel()

	mock, db := newSqlMock(t)

	mock.ExpectBegin()

	uow1 := uow.NewSql(db, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

	ctx, _, err := uow1.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}

	tx := ctx.Value(uow.CtxTxKey{})
	if tx == nil {
		t.Fatalf("expected transaction in context")
	}

	// Verify it's the correct type
	if _, ok := tx.(*sql.Tx); !ok {
		t.Fatalf("expected *sql.Tx in context, got %T", tx)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSqlBegin_WithTxOptions(t *testing.T) {
	t.Parallel()

	mock, db := newSqlMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("TxOptsUser").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	opts := &uow.TxOptions{
		Isolation: uow.LevelSerializable,
		ReadOnly:  false,
	}

	uow := uow.NewSql(db, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

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

func TestSqlBegin_ErrorHandling(t *testing.T) {
	t.Parallel()

	mock, db := newSqlMock(t)

	// Expect BeginTx to return an error
	expectedErr := errors.New("begin transaction failed")
	mock.ExpectBegin().WillReturnError(expectedErr)

	uow := uow.NewSql(db, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

	_, _, err := uow.Begin(context.Background())
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

// ============================================================================
// uowsql.Commit() Tests
// ============================================================================

func TestSqlCommit_CommitsTransaction(t *testing.T) { //nolint
	t.Parallel()

	mock, db := newSqlMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("CommitTest").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	uow := uow.NewSql(db, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

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

func TestSqlCommit_ErrorWhenNoTransactionInContext(t *testing.T) {
	t.Parallel()

	_, db := newSqlMock(t)

	uow1 := uow.NewSql(db, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

	err := uow1.Commit(context.Background())
	if err == nil {
		t.Fatalf("expected error when no transaction in context, got nil")
	}

	if !errors.Is(err, uow.ErrTransactionNotFound) {
		if err.Error() != "no transaction found in context" { //nolint
			t.Fatalf("expected 'no transaction found in context' error, got %v", err)
		}
	}
}

func TestSqlCommit_ErrorWhenTransactionIsNil(t *testing.T) {
	t.Parallel()

	_, db := newSqlMock(t)

	uow1 := uow.NewSql(db, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

	// Manually create context with nil transaction
	ctx := context.WithValue(context.Background(), uow.CtxTxKey{}, (*sql.Tx)(nil))

	err := uow1.Commit(ctx)
	if err == nil {
		t.Fatalf("expected error when transaction is nil, got nil")
	}
}

// ============================================================================
// uowsql.Rollback() Tests
// ============================================================================

func TestSqlRollback_RollsBackTransaction(t *testing.T) { //nolint:dupl
	t.Parallel()

	mock, db := newSqlMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("RollbackTest").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectRollback()

	uow := uow.NewSql(db, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

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

func TestSqlRollback_ErrorWhenNoTransactionInContext(t *testing.T) {
	t.Parallel()

	_, db := newSqlMock(t)

	uow := uow.NewSql(db, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

	err := uow.Rollback(context.Background())
	if err == nil {
		t.Fatalf("expected error when no transaction in context, got nil")
	}

	if err.Error() != "no transaction found in context" {
		t.Fatalf("expected 'no transaction found in context' error, got %v", err)
	}
}

func TestSqlRollback_ErrorWhenTransactionIsNil(t *testing.T) {
	t.Parallel()

	_, db := newSqlMock(t)

	uow1 := uow.NewSql(db, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

	ctx := context.WithValue(context.Background(), uow.CtxTxKey{}, (*sql.Tx)(nil))

	err := uow1.Rollback(ctx)
	if err == nil {
		t.Fatalf("expected error when transaction is nil, got nil")
	}
}

// ============================================================================
// uowsql.SavePoint() Tests
// ============================================================================

func TestSqlSavePoint_CreatesSavePoint(t *testing.T) { //nolint:dupl
	t.Parallel()

	mock, db := newSqlMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("User1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("SAVEPOINT \"sp1\"").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO users").WithArgs("User2").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	uow := uow.NewSql(db, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

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

func TestSqlSavePoint_QuotesName(t *testing.T) {
	t.Parallel()

	mock, db := newSqlMock(t)

	// A name with quotes and a statement terminator must stay inside the
	// identifier instead of ending the statement.
	name := `sp1"; DROP TABLE users; --`

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`SAVEPOINT "sp1""; DROP TABLE users; --"`)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	uow := uow.NewSql(db, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

	ctx, _, err := uow.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}

	if err := uow.SavePoint(ctx, name); err != nil {
		t.Fatalf("SavePoint returned error: %v", err)
	}

	if err := uow.Commit(ctx); err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSqlSavePoint_ErrorWhenNoTransactionInContext(t *testing.T) {
	t.Parallel()

	_, db := newSqlMock(t)

	uow := uow.NewSql(db, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

	err := uow.SavePoint(context.Background(), "sp1")
	if err == nil {
		t.Fatalf("expected error when no transaction in context, got nil")
	}

	if err.Error() != "no transaction found in context" {
		t.Fatalf("expected 'no transaction found in context' error, got %v", err)
	}
}

func TestSqlSavePoint_ErrorWhenTransactionIsNil(t *testing.T) {
	t.Parallel()

	_, db := newSqlMock(t)

	uow1 := uow.NewSql(db, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

	ctx := context.WithValue(context.Background(), uow.CtxTxKey{}, (*sql.Tx)(nil))

	err := uow1.SavePoint(ctx, "sp1")
	if err == nil {
		t.Fatalf("expected error when transaction is nil, got nil")
	}
}

func TestSqlSavePoint_MultipleSavePoints(t *testing.T) { //nolint:dupl
	t.Parallel()

	mock, db := newSqlMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("User1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("SAVEPOINT \"sp1\"").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO users").WithArgs("User2").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec("SAVEPOINT \"sp2\"").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO users").WithArgs("User3").WillReturnResult(sqlmock.NewResult(3, 1))
	mock.ExpectCommit()

	uow := uow.NewSql(db, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

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

func TestSqlIntegration_BeginCommitRollbackFlow(t *testing.T) { //nolint:dupl
	t.Parallel()

	mock, db := newSqlMock(t)

	// First transaction: successful
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("Alice").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// Second transaction: rollback
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("Bob").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectRollback()

	uow := uow.NewSql(db, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

	// First transaction: successful
	ctx1, repo1, err := uow.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}

	if err := repo1.Users().Create("Alice"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := uow.Commit(ctx1); err != nil {
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

func TestSqlIntegration_NestedSavePointAndRollback(t *testing.T) {
	t.Parallel()

	mock, db := newSqlMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("User1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("SAVEPOINT \"sp1\"").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO users").WithArgs("User2").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec("SAVEPOINT \"sp2\"").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	uow := uow.NewSql(db, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

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

func TestSqlIntegration_DoVsManualBeginCommit_ProducesSameResult(t *testing.T) { //nolint
	t.Parallel()

	// Test with Do
	mock1, db1 := newSqlMock(t)

	mock1.ExpectBegin()
	mock1.ExpectExec("INSERT INTO users").WithArgs("User1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock1.ExpectExec("INSERT INTO users").WithArgs("User2").WillReturnResult(sqlmock.NewResult(2, 1))
	mock1.ExpectCommit()

	uow1 := uow.NewSql(db1, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

	err1 := uow1.Do(context.Background(), func(ctx context.Context, repo SqlRepository) error {
		if err := repo.Users().Create("User1"); err != nil {
			return err
		}

		return repo.Users().Create("User2")
	})

	if err1 != nil {
		t.Fatalf("Do returned error: %v", err1)
	}

	if err := mock1.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}

	// Test with Begin/Commit
	mock2, db2 := newSqlMock(t)

	mock2.ExpectBegin()
	mock2.ExpectExec("INSERT INTO users").WithArgs("User1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock2.ExpectExec("INSERT INTO users").WithArgs("User2").WillReturnResult(sqlmock.NewResult(2, 1))
	mock2.ExpectCommit()

	uow2 := uow.NewSql(db2, func(tx *sql.Tx) SqlRepository { return SqlRepository{tx} })

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
// ConvertSqlTxOptions Tests
// ============================================================================

func TestConvertSqlTxOptions_NilOptions(t *testing.T) {
	t.Parallel()

	opts := uow.ConvertSqlTxOptions(nil)

	if opts != nil {
		t.Fatalf("expected nil options for nil input, got %v", opts)
	}
}

func TestConvertSqlTxOptions_NoOptions(t *testing.T) {
	t.Parallel()

	opts := uow.ConvertSqlTxOptions()

	if opts != nil {
		t.Fatalf("expected nil options for empty input, got %v", opts)
	}
}

func TestConvertSqlTxOptions_IsolationLevels(t *testing.T) { //nolint:dupl
	t.Parallel()

	testCases := []struct {
		name       string
		sqlLevel   uow.IsolationLevel
		expectedIs sql.IsolationLevel
	}{
		{defaultLevelName, uow.LevelDefault, sql.LevelDefault},
		{"ReadUncommitted", uow.LevelReadUncommitted, sql.LevelReadUncommitted},
		{"ReadCommitted", uow.LevelReadCommitted, sql.LevelReadCommitted},
		{"WriteCommitted", uow.LevelWriteCommitted, sql.LevelWriteCommitted},
		{"RepeatableRead", uow.LevelRepeatableRead, sql.LevelRepeatableRead},
		{"Snapshot", uow.LevelSnapshot, sql.LevelSnapshot},
		{serializableLevelName, uow.LevelSerializable, sql.LevelSerializable},
		{"Linearizable", uow.LevelLinearizable, sql.LevelLinearizable},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts := uow.ConvertSqlTxOptions(&uow.TxOptions{Isolation: tc.sqlLevel})

			if opts == nil {
				t.Fatalf("expected non-nil options for isolation %v", tc.sqlLevel)
			}

			if opts.Isolation != tc.expectedIs {
				t.Fatalf("expected %v, got %v", tc.expectedIs, opts.Isolation)
			}
		})
	}
}

func TestConvertSqlTxOptions_DefaultIsolationLevel(t *testing.T) {
	t.Parallel()

	// Test with an unknown/invalid isolation level
	opts := uow.ConvertSqlTxOptions(&uow.TxOptions{
		Isolation: uow.IsolationLevel(999),
	})

	if opts == nil {
		t.Fatalf("expected non-nil options, got nil")
	}

	// Isolation levels are passed through as-is to the driver
	if opts.Isolation != sql.IsolationLevel(999) {
		t.Fatalf("expected isolation level 999, got %v", opts.Isolation)
	}
}

func TestConvertSqlTxOptions_ReadOnly(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		readOnly bool
	}{
		{"read-only transaction", true},
		{"read-write transaction", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts := uow.ConvertSqlTxOptions(&uow.TxOptions{
				Isolation: uow.LevelSerializable,
				ReadOnly:  tc.readOnly,
			})
			if opts == nil {
				t.Fatalf("expected non-nil options, got nil")
			}

			if opts.ReadOnly != tc.readOnly {
				t.Fatalf("expected ReadOnly %v, got %v", tc.readOnly, opts.ReadOnly)
			}

			if opts.Isolation != sql.LevelSerializable {
				t.Fatalf("expected %v, got %v", sql.LevelSerializable, opts.Isolation)
			}
		})
	}
}

func TestSqlDo_JoinsRollbackError(t *testing.T) {
	t.Parallel()

	mock, db := newSqlMock(t)

	errWork := errors.New("force rollback")
	errRollback := errors.New("rollback failed")

	mock.ExpectBegin()
	mock.ExpectRollback().WillReturnError(errRollback)

	uow1 := uow.NewSql(db, func(tx *sql.Tx) SqlRepository {
		return SqlRepository{tx}
	})

	err := uow1.Do(context.Background(), func(context.Context, SqlRepository) error {
		return errWork
	})

	if !errors.Is(err, errWork) {
		t.Errorf("expected the work error in the result, got %v", err)
	}

	if !errors.Is(err, errRollback) {
		t.Errorf("expected the rollback error in the result, got %v", err)
	}
}

// ============================================================================
// Helpers
// ============================================================================

func newSqlMock(t *testing.T) (sqlmock.Sqlmock, *sql.DB) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	return mock, db
}
