package user

import (
	"context"

	"github.com/Masterminds/squirrel"
	"hexa/internal/core/domain/entity"
)

func (d *DataAccess) GetByID(ctx context.Context, user *entity.User) (err error) {
	query, args, err := d.QueryBuilder().
		Select("id", "first_name", "last_name", "email").
		From("users").
		Where(squirrel.Eq{"id": user.ID}).
		ToSql()
	if err != nil {
		return err
	}

	err = d.Connection().GetContext(ctx, user, query, args...)

	return
}
