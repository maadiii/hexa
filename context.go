package hexa

import (
	"context"

	"go.uber.org/zap"
)

type Context struct {
	context.Context

	RequestID string

	Identity Identity
	Logger   *zap.Logger
}

type (
	Role        []string
	Permissions []string
)

type Identity struct {
	UserID      string
	Roles       Role
	Permissions Permissions
}
