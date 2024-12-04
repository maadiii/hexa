package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"hexa/internal/core/domain/model"
)

func (c Controller) Get(ctx *gin.Context) {
	var req model.UserRetrivationRequest
	if err := ctx.Bind(&req); err != nil {
		ctx.AbortWithStatus(http.StatusBadRequest)

		return
	}

	res, err := c.users.Get(ctx, &req)
	if err != nil {
		// TODO: use approciate status and error type to client
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, err)

		return
	}

	ctx.JSON(http.StatusCreated, res)
}
