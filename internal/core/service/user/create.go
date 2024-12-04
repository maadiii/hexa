package user

import (
	"context"

	"hexa/internal/core/domain/model"
)

func (s *Service) Create(ctx context.Context, model *model.UserCreationRequest) (out model.UserCreationResponse, err error) {
	model.Password, err = s.hasher.Hash(model.Password)
	if err != nil {
		return
	}

	entity := model.Map()
	err = s.users.Insert(ctx, entity)
	out.ID = *entity.ID

	return
}
