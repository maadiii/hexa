package main

import (
	"hexa/internal/controller/rest"
	"hexa/internal/core/service/infra"
	userService "hexa/internal/core/service/user"
	userDA "hexa/internal/dataacces/pg/user"

	"github.com/gin-gonic/gin"
)

func main() {
	database := infra.NewDatabase()
	param := new(rest.Params)

	userDataAccess := userDA.NewDataAccess(database)
	param.UserService = userService.NewService(userDataAccess, infra.NewHasher())

	router := gin.Default()
	v1 := router.Group("/apiv1")
	rest.Init(param, v1)

	if err := router.Run(); err != nil {
		panic(err)
	}
}
