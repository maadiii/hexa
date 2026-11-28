package uow

import (
	"context"
	"database/sql"
	"errors"

	"github.com/maadiii/hexa"
)

func NewSql[R any](db *sql.DB, constructor func(tx *sql.Tx) R) UoW[R] {
	return &uowsql[R]{db, constructor}
}

type uowsql[R any] struct {
	db          *sql.DB
	constructor func(tx *sql.Tx) R
}

func (u *uowsql[R]) Do(ctx *hexa.Context, fn func(*hexa.Context, R) error, opts ...*TxOptions) error {
	txOpts := ConvertSqlTxOptions(opts...)

	tx, err := u.db.BeginTx(ctx.Context, txOpts)
	if err != nil {
		return err
	}

	defer rollbackOnPanic(tx.Rollback)

	if err := fn(ctx, u.constructor(tx)); err != nil {
		rollbackError := tx.Rollback()
		if rollbackError != nil {
			return errors.Join(err, rollbackError)
		}

		return err
	}

	return tx.Commit()
}

func (u *uowsql[R]) Begin(ctx *hexa.Context, opts ...*TxOptions) (c *hexa.Context, r R, err error) { //nolint
	txOpts := ConvertSqlTxOptions(opts...)

	tx, err := u.db.BeginTx(ctx.Context, txOpts)
	if err != nil {
		return
	}

	ctx.Context = context.WithValue(ctx.Context, CtxTxKey{}, tx)

	return ctx, u.constructor(tx), nil
}

func (u *uowsql[R]) Commit(ctx *hexa.Context) error {
	tx, ok := ctx.Value(CtxTxKey{}).(*sql.Tx)
	if !ok || tx == nil {
		return ErrTransactionNotFound
	}

	return tx.Commit()
}

func (u *uowsql[R]) Rollback(ctx *hexa.Context) error {
	tx, ok := ctx.Value(CtxTxKey{}).(*sql.Tx)
	if !ok || tx == nil {
		return ErrTransactionNotFound
	}

	return tx.Rollback()
}

func (u *uowsql[R]) SavePoint(ctx *hexa.Context, name string) error {
	tx, ok := ctx.Value(CtxTxKey{}).(*sql.Tx)
	if !ok || tx == nil {
		return ErrTransactionNotFound
	}

	_, err := tx.Exec(savepointStatement(name))

	return err
}

func ConvertSqlTxOptions(opts ...*TxOptions) *sql.TxOptions {
	if len(opts) == 0 || opts[0] == nil {
		return nil
	}

	return &sql.TxOptions{
		Isolation: sql.IsolationLevel(opts[0].Isolation),
		ReadOnly:  opts[0].ReadOnly,
	}
}
