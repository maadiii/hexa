package uow_test

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type PgxExecer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type PgxRepository struct {
	pool PgxExecer
}

func (p PgxRepository) Users() PgxUserRepo {
	return PgxUserRepo(p)
}

type PgxUserRepo struct {
	pool PgxExecer
}

func (p PgxUserRepo) Create(name string) error {
	_, err := p.pool.Exec(context.Background(), "INSERT INTO users (name) VALUES ($1)", name)

	return err
}

func (p PgxUserRepo) Get(id int) (string, error) {
	row := p.pool.QueryRow(context.Background(), "SELECT name FROM users WHERE id = $1", id)
	var name string
	if err := row.Scan(&name); err != nil {
		return name, err
	}

	return name, nil
}

type SqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type SqlRepository struct {
	tx SqlExecer
}

func (s SqlRepository) Users() SqlUserRepo {
	return SqlUserRepo(s)
}

type SqlUserRepo struct {
	tx SqlExecer
}

func (s SqlUserRepo) Create(name string) error {
	_, err := s.tx.ExecContext(context.Background(), "INSERT INTO users (name) VALUES (?)", name)

	return err
}

func (s SqlUserRepo) Get(id int) (string, error) {
	row := s.tx.QueryRowContext(context.Background(), "SELECT name FROM users WHERE id = ?", id)

	var name string
	if err := row.Scan(&name); err != nil {
		return name, err
	}

	return name, nil
}

type GormRepository struct {
	tx *gorm.DB
}

func (g GormRepository) Users() GormUserRepo {
	return GormUserRepo(g)
}

type GormUserRepo struct {
	tx *gorm.DB
}

func (g GormUserRepo) Create(name string) error {
	return g.tx.Exec("INSERT INTO users (name) VALUES ($1)", name).Error
}

func (g GormUserRepo) Get(id int) (string, error) {
	row := g.tx.Raw("SELECT name FROM users WHERE id = $1", id).Row()

	var name string
	if err := row.Scan(&name); err != nil {
		return name, err
	}

	return name, nil
}
