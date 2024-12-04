package user

import (
	"context"

	"hexa/internal/core/domain/model"
)

func (s *Service) Get(ctx context.Context, model *model.UserRetrivationRequest) (out model.UserRetrivationResponse, err error) {
	user := model.Map()
	if err = s.users.GetByID(ctx, user); err != nil {
		return
	}

	out.Map(user)

	return
}
