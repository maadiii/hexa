package port

import (
	"context"

	"hexa/internal/core/domain/entity"
	"hexa/internal/core/domain/model"
)

type UserService interface {
	Create(context.Context, *model.UserCreationRequest) (model.UserCreationResponse, error)
	Get(context.Context, *model.UserRetrivationRequest) (model.UserRetrivationResponse, error)
}

type UserDataAccess interface {
	Insert(context.Context, *entity.User) error
	GetByID(context.Context, *entity.User) error
}
