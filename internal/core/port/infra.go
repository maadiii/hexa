package port

import (
	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
)

type Database interface {
	Connection() *sqlx.DB
	QueryBuilder() squirrel.StatementBuilderType
	Close() error
}

type Hasher interface {
	Hash(plain string) (hashed string, err error)
	Compare(plain, hashed string) error
}
