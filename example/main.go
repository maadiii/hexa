package main

import (
	"net/http"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/maadiii/hexa"
	"github.com/maadiii/hexa/hertz"
)

func main() {
	h := New()

	h.POST("/:id", hertz.H(service, http.StatusOK))

	h.Spin()
}

func New() *server.Hertz {
	s := server.Default(
		server.WithHostPorts(":8000"),
		hertz.SetValidator(),
		server.WithMaxRequestBodySize(1<<20), //nolint
	)

	return s
}

type Request struct {
	ID     string `path:"id"`
	Name   string `json:"name"`
	Family string `json:"family"`
}

type Response struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Family string `json:"family"`
	Nation string `json:"nation"`
}

func service(ctx *hexa.Context, rq Request) (rs Response, err error) {
	rs = Response{
		rq.ID, rq.Name, rq.Family, "national",
	}

	return rs, nil
}
