package infra

import (
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"hexa/config"
	"hexa/internal/core/port"
)

type Database struct {
	db *sqlx.DB
}

func NewDatabase() port.Database {
	cfg := config.Database()
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Name,
	)

	conn, err := sqlx.Open(cfg.Driver, dsn)
	if err != nil {
		panic(err)
	}

	if err := conn.Ping(); err != nil {
		panic(err)
	}

	return &Database{conn}
}

func (d *Database) Connection() *sqlx.DB {
	return d.db
}

func (d *Database) QueryBuilder() squirrel.StatementBuilderType {
	return squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
}

func (d *Database) Close() error {
	return d.db.Close()
}
