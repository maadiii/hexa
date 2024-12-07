package user

import (
	"github.com/gin-gonic/gin"
	"hexa/internal/core/port"
)

type Controller struct {
	users port.UserService
}

func New(users port.UserService) *Controller {
	return &Controller{users}
}

func (c Controller) Route(routerGroup *gin.RouterGroup) {
	users := routerGroup.Group("/users")
	users.POST("", c.Register)
	users.GET("/:id", c.Get)
}
