package hertz

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/maadiii/hexa"
)

// To use this method must add SetContext func as first middleware to hertz server
func H[
	IN any, OUT any,
	SVC func(*hexa.Context, *IN) (*OUT, error),
](srv SVC, statusCode int) app.HandlerFunc {
	return func(c context.Context, rc *app.RequestContext) {
		ctx, _ := c.(*hexa.Context)

		req, err := bind[IN](rc)
		if err != nil {
			return
		}

		out, err := callSvc(ctx, srv, req, rc)
		if err != nil {
			return
		}

		rc.JSON(statusCode, out)
	}
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
