package hexa

import "context"

type Context struct {
	context.Context

	RequestID string

	Identity *Identity
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
