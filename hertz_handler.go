package hexa

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/hertz-contrib/cors"
	"github.com/maadiii/gogate/pkg/errors"
	"go.uber.org/fx"
)

func NewHertz(lc fx.Lifecycle, cfg Config) *server.Hertz {
	vd := newValidator()
	s := server.Default(
		server.WithHostPorts(fmt.Sprintf(":%d", cfg.Port)),
		server.WithCustomValidatorFunc(func(r *protocol.Request, i any) error {
			if err := vd.Validate(i); err != nil {
				return errors.WrapCK(err, errors.CodeBadRequest, "validation error")
			}

			return nil
		}),
	)

	addCorsToHertz(s, cfg.CORS)
	s.Use(ErrorHandler(cfg.DevMode))
	s.HEAD("/health", func(_ context.Context, rc *app.RequestContext) {
		rc.Status(http.StatusOK)
	})
	s.GET("/health", func(_ context.Context, rc *app.RequestContext) {
		rc.Status(http.StatusOK)
	})

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				s.Spin()
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			return s.Shutdown(ctx)
		},
	})

	return s
}

func H[
	IN any, OUT any,
	SVC func(*Context, *IN) (*OUT, error),
](srv SVC, statusCode int) app.HandlerFunc {
	return func(c context.Context, rc *app.RequestContext) {
		req, err := hBind[IN](rc)
		if err != nil {
			return
		}

		ctx := appContext(c, rc)

		out, err := callSvc(ctx, srv, req, rc)
		if err != nil {
			return
		}

		rc.JSON(statusCode, out)
	}
}

func addCorsToHertz(s *server.Hertz, corsList []string) {
	s.Use(cors.New(cors.Config{
		AllowOrigins:     corsList,
		AllowMethods:     []string{"PUT", "PATCH", "GET", "POST", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Token"},
		ExposeHeaders:    []string{"Content-Length", "Request-Id"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour, //nolint:mnd
	}))
}

func callSvc[
	IN any, OUT any,
	SRV func(*Context, *IN) (*OUT, error),
](ctx *Context, srv SRV, req *IN, rc *app.RequestContext) (*OUT, error) {
	out, err := srv(ctx, req)
	if err != nil {
		_ = rc.Error(err)
		rc.Abort()
	}

	return out, err
}

func hBind[T any](rc *app.RequestContext) (*T, error) {
	var body T

	err := rc.BindAndValidate(&body)
	if err != nil {
		_ = rc.Error(err)
		rc.Abort()

		return nil, err
	}

	return &body, nil
}

func appContext(c context.Context, rc *app.RequestContext) *Context {
	requsetID := rand.Text()
	rc.Header("Request-Id", requsetID)

	ctx := &Context{
		Context:   c,
		RequestID: requsetID,
	}

	identity, _ := rc.Get(identityKey)
	if identity != nil {
		ctx.Identity = identity.(*Identity) //nolint
	}

	return ctx
}

const (
	identityKey = "identity"
)
