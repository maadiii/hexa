package user

import (
	"hexa/internal/core/port"
)

const (
	TableName      = "users"
	FieldId        = "id"
	FieldFirstName = "first_name"
	FieldLastName  = "last_name"
	FieldEmail     = "email"
	FieldPassword  = "password"
)

type DataAccess struct {
	port.Database
}

func NewDataAccess(db port.Database) port.UserDataAccess {
	return &DataAccess{db}
}
