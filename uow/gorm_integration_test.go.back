//nolint:paralleltest // Tests share and truncate one users table.
package uow_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/maadiii/uow"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openGormIntegrationDB(t *testing.T) (*gorm.DB, *sql.DB) {
	t.Helper()

	db, err := gorm.Open(gormpostgres.Open(realPostgresDSN(t)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open gorm db: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get underlying sql db: %v", err)
	}

	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	resetUsers(t, sqlDB)

	return db, sqlDB
}

func TestGormIntegrationDo_CommitsOnSuccess(t *testing.T) {
	db, sqlDB := openGormIntegrationDB(t)

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	err := uow.Do(context.Background(), func(ctx context.Context, repo GormRepository) error {
		if err := repo.Users().Create("Alice"); err != nil {
			return err
		}

		return repo.Users().Create("Bob")
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if count := countUsers(t, sqlDB); count != 2 {
		t.Fatalf("expected 2 persisted users, got %d", count)
	}
}

func TestGormIntegrationDo_RollsBackOnError(t *testing.T) {
	db, sqlDB := openGormIntegrationDB(t)

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	expectedErr := errors.New("force rollback")
	err := uow.Do(context.Background(), func(ctx context.Context, repo GormRepository) error {
		if err := repo.Users().Create("RolledBack"); err != nil {
			return err
		}

		return expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}

	if count := countUsers(t, sqlDB); count != 0 {
		t.Fatalf("expected no persisted users after rollback, got %d", count)
	}
}

func TestGormIntegrationBegin_SavePointAndCommit(t *testing.T) {
	db, sqlDB := openGormIntegrationDB(t)

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

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

	if count := countUsers(t, sqlDB); count != 2 {
		t.Fatalf("expected 2 persisted users, got %d", count)
	}
}

func TestGormIntegrationBegin_Rollback(t *testing.T) {
	db, sqlDB := openGormIntegrationDB(t)

	uow := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

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

	if count := countUsers(t, sqlDB); count != 0 {
		t.Fatalf("expected no persisted users after rollback, got %d", count)
	}
}

func TestGormIntegrationDo_ReadOnlyRejectsWrites(t *testing.T) {
	db, sqlDB := openGormIntegrationDB(t)

	uow1 := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	err := uow1.Do(context.Background(), func(ctx context.Context, repo GormRepository) error {
		return repo.Users().Create("NotAllowed")
	}, &uow.TxOptions{ReadOnly: true})
	if err == nil {
		t.Fatalf("expected a read-only transaction to reject the insert")
	}

	if count := countUsers(t, sqlDB); count != 0 {
		t.Fatalf("expected no persisted users in a read-only transaction, got %d", count)
	}
}

func TestGormIntegrationDo_IsolationOption(t *testing.T) {
	db, sqlDB := openGormIntegrationDB(t)

	uow1 := uow.NewGorm(db, func(tx *gorm.DB) GormRepository { return GormRepository{tx} })

	var isolation string

	err := uow1.Do(context.Background(), func(ctx context.Context, repo GormRepository) error {
		if err := repo.Users().Create("Serializable"); err != nil {
			return err
		}

		return repo.tx.Raw("SHOW transaction_isolation").Row().Scan(&isolation)
	}, &uow.TxOptions{Isolation: uow.LevelSerializable})
	if err != nil {
		t.Fatalf("expected no error with serializable isolation, got %v", err)
	}

	if isolation != serializableIsolation {
		t.Fatalf("expected the server to report serializable isolation, got %q", isolation)
	}

	if count := countUsers(t, sqlDB); count != 1 {
		t.Fatalf("expected 1 persisted user, got %d", count)
	}
}
