package uow

import (
	"context"
	"database/sql"
	"errors"

	"github.com/maadiii/hexa"
	"gorm.io/gorm"
)

func NewGorm[R any](db *gorm.DB, constructor func(tx *gorm.DB) R) UoW[R] {
	return &uowGorm[R]{db, constructor}
}

type uowGorm[R any] struct {
	db          *gorm.DB
	constructor func(tx *gorm.DB) R
}

func (u *uowGorm[R]) Do(ctx *hexa.Context, fn func(*hexa.Context, R) error, opts ...*TxOptions) error {
	txOpts := ConvertGormTxOptions(opts...)

	tx := u.db.WithContext(ctx.Context).Begin(txOpts)
	if tx.Error != nil {
		return tx.Error
	}

	defer rollbackOnPanic(func() error { return tx.Rollback().Error })

	if err := fn(ctx, u.constructor(tx)); err != nil {
		rollbackError := tx.Rollback().Error
		if rollbackError != nil {
			return errors.Join(err, rollbackError)
		}

		return err
	}

	return tx.Commit().Error
}

func (u *uowGorm[R]) Begin(ctx *hexa.Context, opts ...*TxOptions) (c *hexa.Context, r R, err error) { //nolint
	txOpts := ConvertGormTxOptions(opts...)

	tx := u.db.WithContext(ctx).Begin(txOpts)
	if tx.Error != nil {
		return c, r, tx.Error
	}

	ctx.Context = context.WithValue(ctx.Context, CtxTxKey{}, tx)

	return ctx, u.constructor(tx), nil
}

func (u *uowGorm[R]) Commit(ctx *hexa.Context) error {
	tx, ok := ctx.Value(CtxTxKey{}).(*gorm.DB)
	if !ok || tx == nil {
		return ErrTransactionNotFound
	}

	return tx.Commit().Error
}

func (u *uowGorm[R]) Rollback(ctx *hexa.Context) error {
	tx, ok := ctx.Value(CtxTxKey{}).(*gorm.DB)
	if !ok || tx == nil {
		return ErrTransactionNotFound
	}

	return tx.Rollback().Error
}

func (u *uowGorm[R]) SavePoint(ctx *hexa.Context, name string) error {
	tx, ok := ctx.Value(CtxTxKey{}).(*gorm.DB)
	if !ok || tx == nil {
		return ErrTransactionNotFound
	}

	return tx.Exec(savepointStatement(name)).Error
}

func ConvertGormTxOptions(opts ...*TxOptions) *sql.TxOptions {
	return ConvertSqlTxOptions(opts...)
}
