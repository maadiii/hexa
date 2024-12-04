package model

import (
	"hexa/internal/core/domain/entity"
	"hexa/utility/helper"
)

type UserCreationRequest struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

func (c UserCreationRequest) Map() *entity.User {
	return &entity.User{
		FirstName: &c.FirstName,
		LastName:  &c.LastName,
		Email:     &c.Email,
		Password:  &c.Password,
	}
}

type UserCreationResponse struct {
	ID int64 `json:"id"`
}

type UserRetrivationRequest struct {
	ID int64 `uri:"id"`
}

func (u UserRetrivationRequest) Map() *entity.User {
	return &entity.User{ID: &u.ID}
}

type UserRetrivationResponse struct {
	ID        int64  `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

func (u *UserRetrivationResponse) Map(user *entity.User) {
	u.ID = helper.FromPointer(user.ID)
	u.FirstName = helper.FromPointer(user.FirstName)
	u.LastName = helper.FromPointer(user.LastName)
	u.Email = helper.FromPointer(user.Email)
}
