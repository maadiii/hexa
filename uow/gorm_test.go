package uow_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/maadiii/hexa"
	"github.com/maadiii/hexa/uow"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ============================================================================
// uow.Do Tests
// ============================================================================

func TestGormDo_CommitsOnSuccess(t *testing.T) {
	t.Parallel()

	mock, db := newGormMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").
		WithArgs("Alice").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository {
		return GormRepository{tx}
	})

	err := uow.Do(&hexa.Context{Context: t.Context()}, func(ctx *hexa.Context, repo GormRepository) error {
		return repo.Users().Create("Alice")
	})
	if err != nil {
		t.Fatalf("expected no error on success, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGormDo_RollsBackOnError(t *testing.T) {
	t.Parallel()

	mock, db := newGormMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").
		WithArgs("Bob").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectRollback()

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository {
		return GormRepository{tx}
	})

	err := uow.Do(&hexa.Context{Context: t.Context()}, func(ctx *hexa.Context, repo GormRepository) error {
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

func TestGormDo_RollsBackOnPanic(t *testing.T) { //nolint:dupl
	t.Parallel()

	mock, db := newGormMock(t)

	mock.ExpectBegin()
	mock.ExpectRollback()

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository {
		return GormRepository{tx}
	})

	var recovered any

	func() {
		defer func() { recovered = recover() }()

		_ = uow.Do(&hexa.Context{Context: t.Context()}, func(ctx *hexa.Context, repo GormRepository) error {
			panic("gorm panic")
		})
	}()

	if recovered != "gorm panic" {
		t.Fatalf("expected the panic to propagate, got %v", recovered)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGormDo_PropagatesErrorFromFunction(t *testing.T) {
	t.Parallel()

	mock, db := newGormMock(t)

	mock.ExpectBegin()
	mock.ExpectRollback()

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository {
		return GormRepository{tx}
	})

	expectedErr := errors.New("test error")
	err := uow.Do(&hexa.Context{Context: t.Context()}, func(ctx *hexa.Context, repo GormRepository) error {
		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGormDo_MultipleOperationsWithinTransaction(t *testing.T) { //nolint:dupl
	t.Parallel()

	mock, db := newGormMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("User1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO users").WithArgs("User2").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec("INSERT INTO users").WithArgs("User3").WillReturnResult(sqlmock.NewResult(3, 1))
	mock.ExpectCommit()

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository {
		return GormRepository{tx}
	})

	err := uow.Do(&hexa.Context{Context: t.Context()}, func(ctx *hexa.Context, repo GormRepository) error {
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

func TestGormDo_MultipleOperationsRollbackOnLastError(t *testing.T) {
	t.Parallel()

	mock, db := newGormMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("User1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO users").WithArgs("User2").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectRollback()

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	err := uow.Do(&hexa.Context{Context: t.Context()}, func(ctx *hexa.Context, repo GormRepository) error {
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

func TestGormDo_WithTxOptions(t *testing.T) {
	t.Parallel()

	mock, db := newGormMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("TxOpts").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	opts := &uow.TxOptions{
		Isolation: uow.LevelDefault,
		ReadOnly:  false,
	}

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	err := uow.Do(&hexa.Context{Context: t.Context()}, func(ctx *hexa.Context, repo GormRepository) error {
		return repo.Users().Create("TxOpts")
	}, opts)
	if err != nil {
		t.Fatalf("expected no error with TxOptions, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGormDo_QueryWithinTransaction(t *testing.T) {
	t.Parallel()

	mock, db := newGormMock(t)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT name FROM users WHERE id").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("ExistingUser"))
	mock.ExpectCommit()

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	err := uow.Do(&hexa.Context{Context: t.Context()}, func(ctx *hexa.Context, repo GormRepository) error {
		name, err := repo.Users().Get(1)
		if err != nil {
			return err
		}

		if name != "ExistingUser" {
			return errors.New("unexpected name: " + name)
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

func TestGormDo_ContextCancel(t *testing.T) {
	t.Parallel()

	_, db := newGormMock(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	err := uow.Do(&hexa.Context{Context: ctx}, func(ctx *hexa.Context, repo GormRepository) error {
		return repo.Users().Create("Cancelled")
	})

	// Should get context cancelled error before the transaction ever begins
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// ============================================================================
// uowGorm.Begin() Tests
// ============================================================================

func TestGormBegin_ReturnsContextAndRepoFactory(t *testing.T) {
	t.Parallel()

	mock, db := newGormMock(t)

	mock.ExpectBegin()

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	ctx, repo, err := uow.Begin(&hexa.Context{Context: t.Context()})
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}

	if ctx == nil {
		t.Fatalf("expected non-nil context, got nil")
	}

	if repo == (GormRepository{}) {
		t.Fatalf("expected non-empty repo factory")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGormBegin_ContextContainsTx(t *testing.T) {
	t.Parallel()

	mock, db := newGormMock(t)

	mock.ExpectBegin()

	uow1 := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	ctx, _, err := uow1.Begin(&hexa.Context{Context: t.Context()})
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}

	tx := ctx.Value(uow.CtxTxKey{})
	if tx == nil {
		t.Fatalf("expected transaction in context")
	}

	// Verify it's the correct type
	if _, ok := tx.(*gorm.DB); !ok {
		t.Fatalf("expected *gorm.DB in context, got %T", tx)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGormBegin_WithTxOptions(t *testing.T) {
	t.Parallel()

	mock, db := newGormMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("TxOptsUser").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	opts := &uow.TxOptions{
		Isolation: uow.LevelSerializable,
		ReadOnly:  false,
	}

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	ctx := &hexa.Context{Context: t.Context()}
	ctx, repo, err := uow.Begin(ctx, opts)
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

func TestGormBegin_ErrorHandling(t *testing.T) {
	t.Parallel()

	mock, db := newGormMock(t)

	// Expect BeginTx to return an error
	expectedErr := errors.New("begin transaction failed")
	mock.ExpectBegin().WillReturnError(expectedErr)

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	_, _, err := uow.Begin(&hexa.Context{Context: t.Context()})
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
// uowGorm.Commit() Tests
// ============================================================================

func TestGormCommit_CommitsTransaction(t *testing.T) { //nolint
	t.Parallel()

	mock, db := newGormMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("CommitTest").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	ctx, repo, err := uow.Begin(&hexa.Context{Context: t.Context()})
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

func TestGormCommit_ErrorWhenNoTransactionInContext(t *testing.T) {
	t.Parallel()

	_, db := newGormMock(t)

	uow1 := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	err := uow1.Commit(&hexa.Context{Context: t.Context()})
	if err == nil {
		t.Fatalf("expected error when no transaction in context, got nil")
	}

	if !errors.Is(err, uow.ErrTransactionNotFound) {
		if err.Error() != "no transaction found in context" { //nolint
			t.Fatalf("expected 'no transaction found in context' error, got %v", err)
		}
	}
}

func TestGormCommit_ErrorWhenTransactionIsNil(t *testing.T) {
	t.Parallel()

	_, db := newGormMock(t)

	uow1 := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	// Manually create context with nil transaction
	ctx := context.WithValue(t.Context(), uow.CtxTxKey{}, (*gorm.DB)(nil))

	err := uow1.Commit(&hexa.Context{Context: ctx})
	if err == nil {
		t.Fatalf("expected error when transaction is nil, got nil")
	}
}

// ============================================================================
// uowGorm.Rollback() Tests
// ============================================================================

func TestGormRollback_RollsBackTransaction(t *testing.T) { //nolint:dupl
	t.Parallel()

	mock, db := newGormMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("RollbackTest").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectRollback()

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	ctx, repo, err := uow.Begin(&hexa.Context{Context: t.Context()})
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

func TestGormRollback_ErrorWhenNoTransactionInContext(t *testing.T) {
	t.Parallel()

	_, db := newGormMock(t)

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	err := uow.Rollback(&hexa.Context{Context: t.Context()})
	if err == nil {
		t.Fatalf("expected error when no transaction in context, got nil")
	}

	if err.Error() != "no transaction found in context" {
		t.Fatalf("expected 'no transaction found in context' error, got %v", err)
	}
}

func TestGormRollback_ErrorWhenTransactionIsNil(t *testing.T) {
	t.Parallel()

	_, db := newGormMock(t)

	uow1 := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	ctx := context.WithValue(context.Background(), uow.CtxTxKey{}, (*gorm.DB)(nil))

	err := uow1.Rollback(&hexa.Context{Context: ctx})
	if err == nil {
		t.Fatalf("expected error when transaction is nil, got nil")
	}
}

// ============================================================================
// uowGorm.SavePoint() Tests
// ============================================================================

func TestGormSavePoint_CreatesSavePoint(t *testing.T) { //nolint:dupl
	t.Parallel()

	mock, db := newGormMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("User1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("SAVEPOINT \"sp1\"").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO users").WithArgs("User2").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	ctx, repo, err := uow.Begin(&hexa.Context{Context: t.Context()})
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

func TestGormSavePoint_ErrorWhenNoTransactionInContext(t *testing.T) {
	t.Parallel()

	_, db := newGormMock(t)

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	err := uow.SavePoint(&hexa.Context{Context: t.Context()}, "sp1")
	if err == nil {
		t.Fatalf("expected error when no transaction in context, got nil")
	}

	if err.Error() != "no transaction found in context" {
		t.Fatalf("expected 'no transaction found in context' error, got %v", err)
	}
}

func TestGormSavePoint_ErrorWhenTransactionIsNil(t *testing.T) {
	t.Parallel()

	_, db := newGormMock(t)

	uow1 := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	ctx := context.WithValue(context.Background(), uow.CtxTxKey{}, (*gorm.DB)(nil))

	err := uow1.SavePoint(&hexa.Context{Context: ctx}, "sp1")
	if err == nil {
		t.Fatalf("expected error when transaction is nil, got nil")
	}
}

func TestGormSavePoint_MultipleSavePoints(t *testing.T) { //nolint:dupl
	t.Parallel()

	mock, db := newGormMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("User1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("SAVEPOINT \"sp1\"").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO users").WithArgs("User2").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec("SAVEPOINT \"sp2\"").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO users").WithArgs("User3").WillReturnResult(sqlmock.NewResult(3, 1))
	mock.ExpectCommit()

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	ctx, repo, err := uow.Begin(&hexa.Context{Context: t.Context()})
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

func TestGormDo_JoinsRollbackError(t *testing.T) {
	t.Parallel()

	mock, db := newGormMock(t)

	errWork := errors.New("force rollback")
	errRollback := errors.New("rollback failed")

	mock.ExpectBegin()
	mock.ExpectRollback().WillReturnError(errRollback)

	uow1 := uow.NewGorm(db, func(tx *gorm.DB) GormRepository {
		return GormRepository{tx}
	})

	err := uow1.Do(&hexa.Context{Context: t.Context()}, func(*hexa.Context, GormRepository) error {
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

func newGormMock(t *testing.T) (sqlmock.Sqlmock, *gorm.DB) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}

	db, err := gorm.Open(
		postgres.New(postgres.Config{Conn: sqlDB}),
		&gorm.Config{
			DisableAutomaticPing:   true,
			SkipDefaultTransaction: true,
			Logger:                 logger.Default.LogMode(logger.Silent),
		},
	)
	if err != nil {
		t.Fatalf("failed to open gorm db: %v", err)
	}

	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return mock, db
}

// ============================================================================
// Integration Tests: Begin/Commit/Rollback Flow
// ============================================================================

func TestGormIntegration_BeginCommitRollbackFlow(t *testing.T) { //nolint:dupl
	t.Parallel()

	mock, db := newGormMock(t)

	// First transaction: successful
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("Alice").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// Second transaction: rollback
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("Bob").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectRollback()

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	// First transaction: successful
	ctx1, repo1, err := uow.Begin(&hexa.Context{Context: t.Context()})
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
	ctx2, repo2, err := uow.Begin(&hexa.Context{Context: t.Context()})
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

func TestGormIntegration_NestedSavePointAndRollback(t *testing.T) {
	t.Parallel()

	mock, db := newGormMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO users").WithArgs("User1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("SAVEPOINT \"sp1\"").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO users").WithArgs("User2").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec("SAVEPOINT \"sp2\"").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	ctx, repo, err := uow.Begin(&hexa.Context{Context: t.Context()})
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

func TestGormIntegration_DoVsManualBeginCommit_ProducesSameResult(t *testing.T) { //nolint
	t.Parallel()

	// Test with Do
	mock1, db1 := newGormMock(t)

	mock1.ExpectBegin()
	mock1.ExpectExec("INSERT INTO users").WithArgs("User1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock1.ExpectExec("INSERT INTO users").WithArgs("User2").WillReturnResult(sqlmock.NewResult(2, 1))
	mock1.ExpectCommit()

	uow1 := uow.NewGorm(db1, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	err1 := uow1.Do(&hexa.Context{Context: t.Context()}, func(ctx *hexa.Context, repo GormRepository) error {
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
	mock2, db2 := newGormMock(t)

	mock2.ExpectBegin()
	mock2.ExpectExec("INSERT INTO users").WithArgs("User1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock2.ExpectExec("INSERT INTO users").WithArgs("User2").WillReturnResult(sqlmock.NewResult(2, 1))
	mock2.ExpectCommit()

	uow2 := uow.NewGorm(db2, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	ctx, repo, err := uow2.Begin(&hexa.Context{Context: t.Context()})
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
// ConvertGormTxOptions Tests
// ============================================================================

func TestConvertGormTxOptions_NilOptions(t *testing.T) {
	t.Parallel()

	opts := uow.ConvertGormTxOptions(nil)

	if opts != nil {
		t.Fatalf("expected nil options for nil input, got %v", opts)
	}
}

func TestConvertGormTxOptions_NoOptions(t *testing.T) {
	t.Parallel()

	opts := uow.ConvertGormTxOptions()

	if opts != nil {
		t.Fatalf("expected nil options for empty input, got %v", opts)
	}
}

func TestConvertGormTxOptions_IsolationLevels(t *testing.T) { //nolint:dupl
	t.Parallel()

	testCases := []struct {
		name       string
		sqlLevel   uow.IsolationLevel
		expectedIs sql.IsolationLevel
	}{
		{"LevelDefault", uow.LevelDefault, sql.LevelDefault},
		{"LevelReadUncommitted", uow.LevelReadUncommitted, sql.LevelReadUncommitted},
		{"LevelReadCommitted", uow.LevelReadCommitted, sql.LevelReadCommitted},
		{"LevelWriteCommitted", uow.LevelWriteCommitted, sql.LevelWriteCommitted},
		{"LevelRepeatableRead", uow.LevelRepeatableRead, sql.LevelRepeatableRead},
		{"LevelSnapshot", uow.LevelSnapshot, sql.LevelSnapshot},
		{"LevelSerializable", uow.LevelSerializable, sql.LevelSerializable},
		{"LevelLinearizable", uow.LevelLinearizable, sql.LevelLinearizable},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts := uow.ConvertGormTxOptions(&uow.TxOptions{Isolation: tc.sqlLevel})

			if opts == nil {
				t.Fatalf("expected non-nil options for isolation %v", tc.sqlLevel)
			}

			if opts.Isolation != tc.expectedIs {
				t.Fatalf("expected %v, got %v", tc.expectedIs, opts.Isolation)
			}
		})
	}
}

func TestConvertGormTxOptions_DefaultIsolationLevel(t *testing.T) {
	t.Parallel()

	opts := uow.ConvertGormTxOptions(&uow.TxOptions{Isolation: uow.IsolationLevel(999)})

	if opts == nil {
		t.Fatalf("expected non-nil options, got nil")
	}

	if opts.Isolation != sql.IsolationLevel(999) {
		t.Fatalf("expected isolation level 999, got %v", opts.Isolation)
	}
}

func TestConvertGormTxOptions_ReadOnly(t *testing.T) { //nolint:dupl
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

			opts := uow.ConvertGormTxOptions(&uow.TxOptions{
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
