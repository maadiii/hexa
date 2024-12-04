package user

import (
	"hexa/internal/core/port"
)

type Service struct {
	users  port.UserDataAccess
	hasher port.Hasher
}

func NewService(users port.UserDataAccess, hasher port.Hasher) port.UserService {
	return &Service{users, hasher}
}
