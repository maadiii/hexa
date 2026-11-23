package hertz

import (
	"context"
	"crypto/rand"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/go-playground/validator/v10"
	"github.com/hertz-contrib/cors"
	"github.com/maadiii/gogate/pkg/errors"
	"github.com/maadiii/hexa"
	"go.uber.org/fx"
)

func New(lc fx.Lifecycle, cfg Config, opts ...config.Option) *server.Hertz {
	vd := newValidator()
	opts = append(
		opts,
		server.WithCustomValidatorFunc(func(r *protocol.Request, i any) error {
			if err := vd.Validate(i); err != nil {
				return errors.WrapCK(err, errors.CodeBadRequest, "validation error")
			}

			return nil
		}),
	)

	s := server.Default(opts...)

	addCors(s, cfg.CORS)
	s.Use(errors.ErrorHandler(cfg.DevMode))
	s.HEAD("/healthz", func(_ context.Context, rc *app.RequestContext) {
		rc.Status(http.StatusOK)
	})
	s.GET("/healthz", func(_ context.Context, rc *app.RequestContext) {
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
	SVC func(*hexa.Context, *IN) (*OUT, error),
](srv SVC, statusCode int) app.HandlerFunc {
	return func(c context.Context, rc *app.RequestContext) {
		req, err := bind[IN](rc)
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

func addCors(s *server.Hertz, corsList []string) {
	s.Use(cors.New(cors.Config{
		AllowOrigins:     corsList,
		AllowMethods:     allowMethods,
		AllowHeaders:     allowHeaders,
		ExposeHeaders:    exposeHeaders,
		AllowCredentials: true,
		MaxAge:           12 * time.Hour, //nolint:mnd
	}))
}

func callSvc[
	IN any, OUT any,
	SRV func(*hexa.Context, *IN) (*OUT, error),
](ctx *hexa.Context, srv SRV, req *IN, rc *app.RequestContext) (*OUT, error) {
	out, err := srv(ctx, req)
	if err != nil {
		_ = rc.Error(err)
		rc.Abort()
	}

	return out, err
}

func bind[T any](rc *app.RequestContext) (*T, error) {
	var body T

	err := rc.BindAndValidate(&body)
	if err != nil {
		_ = rc.Error(err)
		rc.Abort()

		return nil, err
	}

	return &body, nil
}

func appContext(c context.Context, rc *app.RequestContext) *hexa.Context {
	requsetID := rand.Text()
	rc.Header("Request-Id", requsetID)

	ctx := &hexa.Context{
		Context:   c,
		RequestID: requsetID,
	}

	identity, _ := rc.Get(identityKey)
	if identity != nil {
		ctx.Identity = identity.(*hexa.Identity) //nolint
	}

	return ctx
}

type Config struct {
	DevMode bool
	CORS    []string
}

type Validator struct {
	validator *validator.Validate
}

func newValidator() *Validator {
	return &Validator{
		validator: validator.New(),
	}
}

func (v *Validator) Validate(i any) error {
	return v.validator.Struct(i)
}

const (
	identityKey = "identity"
)

var (
	allowMethods  = []string{"PUT", "PATCH", "GET", "POST", "DELETE", "HEAD", "OPTIONS"}
	allowHeaders  = []string{"Origin", "Content-Type", "Authorization", "Token"}
	exposeHeaders = []string{"Content-Length", "Request-Id"}
)
