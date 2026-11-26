package main

import (
	"net/http"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/maadiii/hexa"
	"github.com/maadiii/hexa/hertz"
	"github.com/maadiii/hexa/logging"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
)

func main() {
	fx.New(
		fx.Provide(
			fx.Annotate(
				func() logging.Options {
					return logging.Options{
						Service: "example-service",
						Version: "0.0.1",
						Level:   zap.DebugLevel,
						Format:  "json",
					}
				},
				fx.ResultTags(`name:"logOpt"`),
			),
		),
		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log}
		}),
		fx.Provide(func() hertz.Config {
			return hertz.Config{
				DevMode: false,
				CORS:    []string{"http://localhost:3000"},
			}
		}),
		fx.Provide(
			fx.Annotate(
				func() []config.Option {
					return []config.Option{
						server.WithHostPorts(":8000"),
					}
				},
				fx.ResultTags(`name:"opts"`),
			),
		),
		fx.Provide(
			fx.Annotate(
				hertz.New,
				fx.ParamTags(``, ``, `name:"logOpt"`, `name:"opts"`),
			),
		),
		fx.Invoke(func(h *server.Hertz) {
			h.GET("/:name", hertz.H(Service, http.StatusOK))
		}),
	).Run()
}

func Service(ctx *hexa.Context, rq *Request) (*Response, error) {
	ctx.Logger.Info("HELLOOOOOO", zap.String("name", rq.Name), zap.String("request id", ctx.RequestID))

	return &Response{ID: "ID", Name: rq.Name}, nil
}

type Request struct {
	Name string `path:"name" validate:"required"`
}

type Response struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
