package uow

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/maadiii/hexa"
)

// UoW is a unit of work over a repository of type R.
type UoW[R any] interface {
	Do(ctx *hexa.Context, fn func(ctx *hexa.Context, repo R) error, opts ...*TxOptions) error
	Begin(ctx *hexa.Context, opts ...*TxOptions) (*hexa.Context, R, error)
	SavePoint(ctx *hexa.Context, name string) error
	Commit(ctx *hexa.Context) error
	Rollback(ctx *hexa.Context) error
}

// IsolationLevel is the transaction isolation level used in [TxOptions].
type IsolationLevel int

// Various isolation levels that drivers may support in [DB.BeginTx].
// If a driver does not support a given isolation level an error may be returned.
//
// See https://en.wikipedia.org/wiki/Isolation_(database_systems)#Isolation_levels.
const (
	LevelDefault IsolationLevel = iota
	LevelReadUncommitted
	LevelReadCommitted
	LevelWriteCommitted
	LevelRepeatableRead
	LevelSnapshot
	LevelSerializable
	LevelLinearizable
)

// String returns the name of the transaction isolation level.
func (i IsolationLevel) String() string {
	switch i {
	case LevelDefault:
		return "Default"
	case LevelReadUncommitted:
		return "Read Uncommitted"
	case LevelReadCommitted:
		return "Read Committed"
	case LevelWriteCommitted:
		return "Write Committed"
	case LevelRepeatableRead:
		return "Repeatable Read"
	case LevelSnapshot:
		return "Snapshot"
	case LevelSerializable:
		return "Serializable"
	case LevelLinearizable:
		return "Linearizable"
	default:
		return "IsolationLevel(" + strconv.Itoa(int(i)) + ")"
	}
}

var _ fmt.Stringer = LevelDefault

const savepoint = "SAVEPOINT"

// rollbackOnPanic rolls the transaction back when the surrounding function
// panics and re-panics so the caller still sees the original failure. It does
// nothing when the function returns normally, including after a failed commit.
func rollbackOnPanic(rollback func() error) {
	if p := recover(); p != nil {
		_ = rollback()
		panic(p)
	}
}

// savepointStatement builds a SAVEPOINT statement for name. A savepoint name
// cannot be passed as a query parameter, so it is quoted as a SQL identifier;
// that keeps names containing quotes, semicolons or comments from changing the
// meaning of the statement.
func savepointStatement(name string) string {
	return savepoint + " " + quoteIdentifier(name)
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// TxOptions holds the transaction options to be used in [DB.BeginTx].
type TxOptions struct {
	// Isolation is the transaction isolation level.
	// If zero, the driver or database's default level is used.
	Isolation IsolationLevel
	ReadOnly  bool
}
