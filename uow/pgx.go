package uow

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/maadiii/hexa"
)

var ErrTransactionNotFound = errors.New("no transaction found in context")

func NewPgx[R any](pool PgxPool, constructor func(pgx.Tx) R) UoW[R] {
	return &uowPgx[R]{pool, constructor}
}

type uowPgx[R any] struct {
	pool        PgxPool
	constructor func(tx pgx.Tx) R
}

func (u *uowPgx[R]) Do(ctx *hexa.Context, fn func(*hexa.Context, R) error, opts ...*TxOptions) error {
	txOpts := ConvertPgxTxOptions(opts...)

	tx, err := u.pool.BeginTx(ctx.Context, txOpts)
	if err != nil {
		return err
	}

	defer rollbackOnPanic(func() error { return tx.Rollback(ctx) })

	if err := fn(ctx, u.constructor(tx)); err != nil {
		rollBackError := tx.Rollback(ctx.Context)
		if rollBackError != nil {
			return errors.Join(err, rollBackError)
		}

		return err
	}

	return tx.Commit(ctx)
}

func (u *uowPgx[R]) Begin(ctx *hexa.Context, opts ...*TxOptions) (c *hexa.Context, r R, err error) { //nolint
	txOpts := ConvertPgxTxOptions(opts...)

	tx, err := u.pool.BeginTx(ctx.Context, txOpts)
	if err != nil {
		return
	}

	ctx.Context = context.WithValue(ctx.Context, CtxTxKey{}, tx)

	return ctx, u.constructor(tx), nil
}

func (u *uowPgx[R]) Commit(ctx *hexa.Context) error {
	tx, ok := ctx.Value(CtxTxKey{}).(pgx.Tx)
	if !ok || tx == nil {
		return ErrTransactionNotFound
	}

	return tx.Commit(ctx.Context)
}

func (u *uowPgx[R]) Rollback(ctx *hexa.Context) error {
	tx, ok := ctx.Value(CtxTxKey{}).(pgx.Tx)
	if !ok || tx == nil {
		return ErrTransactionNotFound
	}

	return tx.Rollback(ctx.Context)
}

func (u *uowPgx[R]) SavePoint(ctx *hexa.Context, name string) error {
	tx, ok := ctx.Value(CtxTxKey{}).(pgx.Tx)
	if !ok || tx == nil {
		return ErrTransactionNotFound
	}

	_, err := tx.Exec(ctx.Context, savepointStatement(name))

	return err
}

func ConvertPgxTxOptions(opts ...*TxOptions) pgx.TxOptions {
	var pgxOpts pgx.TxOptions

	if len(opts) == 0 || opts[0] == nil {
		pgxOpts.AccessMode = pgx.ReadWrite

		return pgxOpts
	}

	switch opts[0].Isolation { //nolint: exhaustive
	case LevelRepeatableRead:
		pgxOpts.IsoLevel = pgx.RepeatableRead
	case LevelSerializable:
		pgxOpts.IsoLevel = pgx.Serializable
	default:
		pgxOpts.IsoLevel = pgx.ReadCommitted
	}

	pgxOpts.AccessMode = ChooseAccessMode(opts[0])

	return pgxOpts
}

func ChooseAccessMode(opts *TxOptions) pgx.TxAccessMode {
	if opts == nil {
		return pgx.ReadWrite
	}

	if opts.ReadOnly {
		return pgx.ReadOnly
	}

	return pgx.ReadWrite
}

type (
	CtxTxKey struct{}
	PgxPool  interface {
		BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
	}
)
