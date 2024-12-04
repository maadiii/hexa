package user_test

import (
	"os"
	"testing"

	"github.com/Masterminds/squirrel"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"hexa/internal/core/port"
	"hexa/internal/dataacces/pg/user"
)

var dataAccess port.UserDataAccess
var db *sqlx.DB

func TestMain(m *testing.M) {
	tearup()
	os.Exit(m.Run())
}

func tearup() {
	sqlx, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}
	if err = sqlx.Ping(); err != nil {
		panic(err)
	}

	dbDriver, err := sqlite3.WithInstance(sqlx.DB, &sqlite3.Config{})
	if err != nil {
		panic(err)
	}
	source, err := (new(file.File)).Open("file://../../../../script/migration")
	if err != nil {
		panic(err)
	}

	m, err := migrate.NewWithInstance(":memory:", source, "test", dbDriver)
	if err != nil {
		panic(err)
	}
	if err := m.Up(); err != nil {
		panic(err)
	}

	dataAccess = user.NewDataAccess(&mockedDataBase{sqlx})
	db = sqlx
}

type mockedDataBase struct {
	db *sqlx.DB
}

func (m *mockedDataBase) Connection() *sqlx.DB {
	return m.db
}

func (m *mockedDataBase) QueryBuilder() squirrel.StatementBuilderType {
	return squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
}

func (m *mockedDataBase) Close() error {
	return m.db.Close()
}
