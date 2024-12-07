package rest

import (
	"github.com/gin-gonic/gin"
	"hexa/internal/controller/rest/user"
	"hexa/internal/core/port"
)

type Params struct {
	UserService port.UserService
}

func Init(params *Params, router *gin.RouterGroup) {
	user.New(params.UserService).Route(router)
}
