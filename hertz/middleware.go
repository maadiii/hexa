package hertz

import (
	"context"
	"crypto/rand"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/go-playground/validator/v10"
	"github.com/maadiii/hexa"
	"github.com/maadiii/hexa/errors"
	"go.uber.org/zap"
)

// SetContext use &hexa.Context{} as context.Context in all middleware and handler funcs if
// be called at first server.Use as middleware.
func SetContext() app.HandlerFunc {
	return func(c context.Context, rc *app.RequestContext) {
		c = &hexa.Context{Context: c}

		rc.Next(c)
	}
}

func SimpleRequestId() app.HandlerFunc { //nolint
	return func(c context.Context, rc *app.RequestContext) {
		reqId := rc.GetHeader("X-Request-Id")

		err := errors.WrapCK(nil, errors.CodeBadRequest, "invalid request id")
		if len(reqId) > 64 { //nolint
			_ = rc.AbortWithError(http.StatusBadRequest, err)

			return
		}

		if len(reqId) == 0 {
			reqId := rand.Text()
			rc.Header("X-Request-Id", reqId)
			ctx, _ := c.(*hexa.Context)
			ctx.RequestID = reqId

			rc.Next(c)

			return
		}

		for i := range reqId {
			ch := reqId[i]

			switch {
			case ch >= 'a' && ch <= 'z':
			case ch >= 'A' && ch <= 'Z':
			case ch >= '0' && ch <= '9':
			case ch == '-' || ch == '_':
			default:
				id := string(reqId)
				rc.Header("X-Request-Id", id)
				ctx, _ := c.(*hexa.Context)
				ctx.RequestID = id
				_ = rc.AbortWithError(http.StatusBadRequest, err)

				return
			}
		}

		id := string(reqId)
		rc.Header("X-Request-Id", id)
		ctx, _ := c.(*hexa.Context)
		ctx.RequestID = id

		rc.Next(ctx)
	}
}

func AccessLog(logger *zap.Logger) app.HandlerFunc {
	return func(c context.Context, rc *app.RequestContext) {
		rc.Next(c)

		if rc.Response.StatusCode() < http.StatusBadRequest {
			return
		}

		ctx, _ := c.(*hexa.Context)

		fields := []zap.Field{
			zap.String("method", string(rc.Method())),
			zap.String("path", string(rc.URI().Path())),
			zap.String("route", rc.FullPath()),
			zap.Int("status", rc.Response.StatusCode()),
			zap.String("request_id", ctx.RequestID),
			zap.String("user_id", ctx.Identity.UserID),
			zap.String("client_ip", rc.ClientIP()),
		}

		fields = append(fields, zap.Error(rc.Errors[len(rc.Errors)-1].Unwrap()))
		status := rc.Response.StatusCode()
		switch {
		case status >= http.StatusInternalServerError:
			logger.Error("api call failed", fields...)
		case status >= http.StatusBadRequest:
			logger.Warn("api call rejected", fields...)
		}
	}
}

type Validator struct {
	validator *validator.Validate
}

func SetValidator() config.Option {
	vd := &Validator{
		validator: validator.New(),
	}

	return server.WithCustomValidatorFunc(func(r *protocol.Request, i any) error {
		if err := vd.Validate(i); err != nil {
			return errors.WrapCK(err, errors.CodeBadRequest, "invalid request")
		}

		return nil
	})
}

func (v *Validator) Validate(i any) error {
	return v.validator.Struct(i)
}
