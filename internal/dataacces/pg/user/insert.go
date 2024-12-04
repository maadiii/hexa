package user

import (
	"context"

	"hexa/internal/core/domain/entity"
)

func (d *DataAccess) Insert(ctx context.Context, user *entity.User) error {
	query, args, err := d.QueryBuilder().
		Insert(TableName).
		Columns(FieldId, FieldFirstName, FieldLastName, FieldEmail, FieldPassword).
		Values(user.ID, user.FirstName, user.LastName, user.Email, user.Password).
		ToSql()
	if err != nil {
		return err
	}

	_, err = d.Connection().ExecContext(ctx, query, args...)

	return err
}
