package uow_test

import (
	"testing"

	"github.com/maadiii/hexa/uow"
)

const (
	defaultLevelName      = "Default"
	serializableLevelName = "Serializable"
)

func TestIsolationLevelString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		level uow.IsolationLevel
		want  string
	}{
		{name: "default", level: uow.LevelDefault, want: defaultLevelName},
		{name: "read uncommitted", level: uow.LevelReadUncommitted, want: "Read Uncommitted"},
		{name: "read committed", level: uow.LevelReadCommitted, want: "Read Committed"},
		{name: "write committed", level: uow.LevelWriteCommitted, want: "Write Committed"},
		{name: "repeatable read", level: uow.LevelRepeatableRead, want: "Repeatable Read"},
		{name: "snapshot", level: uow.LevelSnapshot, want: "Snapshot"},
		{name: "serializable", level: uow.LevelSerializable, want: serializableLevelName},
		{name: "linearizable", level: uow.LevelLinearizable, want: "Linearizable"},
		{name: "unknown", level: uow.IsolationLevel(999), want: "IsolationLevel(999)"},
		{name: "negative", level: uow.IsolationLevel(-1), want: "IsolationLevel(-1)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.level.String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
