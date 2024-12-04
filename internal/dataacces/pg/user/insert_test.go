package user_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"hexa/internal/core/domain/entity"
	"hexa/utility/helper"
)

func TestInsert(t *testing.T) {
	ctx := context.Background()
	user := &entity.User{
		ID:        helper.ToPointer(int64(1)),
		FirstName: helper.ToPointer("Test First Name"),
		LastName:  helper.ToPointer("Test Last Name"),
		Email:     helper.ToPointer("Test Email"),
		Password:  helper.ToPointer("Test Password"),
	}

	err := dataAccess.Insert(ctx, user)
	assert.NoError(t, err) //nolint

	var inserted entity.User
	err = db.GetContext(ctx, &inserted, "select * from users where id = 1")
	assert.NoError(t, err) //nolint
	assert.NotNil(t, inserted)
	assert.Equal(t, "Test First Name", *inserted.FirstName)
	assert.Equal(t, "Test Last Name", *inserted.LastName)
	assert.Equal(t, "Test Email", *inserted.Email)
	assert.Equal(t, "Test Password", *inserted.Password)
}
