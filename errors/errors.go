package errors

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/pkg/errors"

	"github.com/cloudwego/hertz/pkg/app"
	hertzErrors "github.com/cloudwego/hertz/pkg/common/errors"
)

// StatusOf maps a coded error to the HTTP status it is reported with.
//
// One function decides this so that the pipeline's fallback abort and the
// ErrorHandler middleware cannot disagree. Two independent mappings would drift
// apart, and the symptom would be a status that changes depending on whether a
// middleware happened to be installed.
func StatusOf(err error) int {
	appErr := coded(err)
	if appErr == nil {
		return http.StatusInternalServerError
	}

	switch appErr.code {
	case CodeNotFound:
		return http.StatusNotFound
	case CodeAlreadyExists:
		return http.StatusConflict
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeBadRequest:
		return http.StatusBadRequest
	case CodeBadGateway:
		return http.StatusBadGateway
	case CodeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// ErrorHandler turns whatever errors a request recorded into the response the
// client sees: a body carrying the error's key, plus a stack trace when not
// running in production, under the status that key's code maps to.
//
// This is the single place an error response is shaped, so every failure
// reaches the client in the same form — a hook rejecting a request, an
// unmatched route, a backend that refused the connection. It has to be
// installed before the routes are registered, since it depends on running ahead
// of the handler and continuing after it.
//
// TODO: Add observability
func ErrorHandler(devMode bool) app.HandlerFunc {
	return func(c context.Context, rc *app.RequestContext) {
		rc.Next(c)
		if len(rc.Errors) == 0 {
			return
		}

		err := rc.Errors[len(rc.Errors)-1]
		var rrr *Error
		if !As(err, &rrr) {
			res := map[string]string{"KEY": internalKey}
			if devMode {
				res["VERBOSE"] = fmt.Sprintf("%+v", err.Err)
			}

			rc.AbortWithStatusJSON(http.StatusInternalServerError, res)

			return
		}

		// The last coded error is the client's answer. Hooks earlier in the
		// chain record why the request failed, and an OnError hook with an
		// opinion about how to report it records one afterwards, so taking the
		// most recent lets an error-formatting or fallback hook have the final
		// say. Errors carrying no code of ours take no part in the decision:
		// they are bookkeeping about the error handling itself, and letting
		// them win would hide the failure that actually brought us here.
		appErr := lastCodedError(rc.Errors)
		res := map[string]any{"KEY": appErr.key}
		if devMode {
			res["VERBOSE"] = fmt.Sprintf("%+v", appErr.error)
		}

		rc.AbortWithStatusJSON(StatusOf(appErr), res)
	}
}

// lastCodedError returns the most recently recorded coded error, or nil when
// the request recorded none.
func lastCodedError(errs []*hertzErrors.Error) *Error {
	for _, err := range slices.Backward(errs) {
		if appErr := coded(err); appErr != nil {
			return appErr
		}
	}

	return nil
}

// NotFound reports a resource that does not exist.
func NotFound(what string) error {
	return WrapCK(nil, CodeNotFound, fmt.Sprintf("%s not found", what))
}

// Unauthorized reports a request that carried no usable credentials.
func Unauthorized() error {
	return WrapCK(nil, CodeUnauthorized, "unauthorized")
}

// Forbidden reports a caller that is authenticated but not allowed.
func Forbidden() error {
	return WrapCK(nil, CodeForbidden, "forbidden")
}

// BadGateway reports a failure to reach the downstream service, or to complete
// the call once it was reached.
func BadGateway(err error) error {
	return WrapCK(err, CodeBadGateway, "bad gateway")
}

// Code classifies an error so that one place can decide how it is reported to
// the client.
//
// It is deliberately not an HTTP status. The code is what the code that failed
// actually knows ("this was not authorised"), while the status is a transport
// concern that follows from it; collapsing the two would put net/http in the
// business layer and make the same failure mean different things depending on
// who caught it.
type Code int

const (
	CodeInternal Code = iota
	CodeNotFound
	CodeAlreadyExists
	CodeUnauthorized
	CodeForbidden
	CodeBadRequest
	CodeBadGateway
)

// Error is an error carrying a code and a stable, machine-readable key.
//
// The key is what the client is told, so it must stay stable across releases:
// it is an identifier, not a message. The human-readable text lives in whatever
// the error wraps, which is why Error embeds error rather than replacing it —
// formatting a *Error delegates to the wrapped value, so a pkg/errors stack
// trace still prints under %+v.
type Error struct {
	error
	key string

	code Code
}

// internalKey is the identifier reported for failures the gateway has nothing
// more specific to say about.
const internalKey = "INTERNAL"

// New builds an error with no specific classification, defaulting to
// [CodeInternal]. Prefer WrapCK when the caller knows what kind of failure this
// is.
//
// text is a developer-facing message and deliberately does not become the key.
// The key is sent to the client, so deriving one from arbitrary text would
// publish whatever an internal message happened to say — a message naming a
// host, a query, or a credential would be reported to anyone who could make the
// request fail. The text is still readable through the error itself, which is
// what the stack trace in a development response is built from.
func New(text string) error {
	return &Error{
		error: errors.New(text),
		key:   internalKey,
		code:  CodeInternal,
	}
}

// As is [errors.As], re-exported so callers matching a *Error do not have to
// import two packages named errors.
func As(err error, target any) bool {
	return errors.As(err, target)
}

func HasCode(err error, code Code) bool {
	target := new(Error)
	if !errors.As(err, &target) {
		return false
	}

	return target.code == code
}

// Is reports whether err is the same kind of coded error as target: same code,
// same key. Anything that is not one of this package's errors falls through to
// [errors.Is], so ordinary sentinel comparisons keep working.
//
// It deliberately does not treat a bare new(Error) as a wildcard match. The
// zero value has an empty key, so comparing against it answers "is this one of
// ours?" with false for every real error — which is what silently turned the
// re-wrap guard in Wrap into a no-op, and made wrapping an Unauthorized()
// downgrade it to an internal 500.
func Is(err error, target error) bool {
	targetErr := new(Error)
	if !errors.As(target, &targetErr) {
		return errors.Is(err, target)
	}

	errErr := coded(err)
	if errErr == nil {
		return false
	}

	return errErr.code == targetErr.code && errErr.key == targetErr.key
}

// coded returns err's own *Error, or nil when err carries no code of ours. It
// is the one place the "is this ours?" question is answered.
func coded(err error) *Error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}

	return nil
}

// Wrap classifies an error that carries no code of its own as internal, keeping
// the original as the cause.
//
// An error that already carries a code is returned untouched. That is what
// stops the pipeline from flattening a deliberate rejection into a 500: a hook
// that returns Forbidden() means 403 all the way to the client, even though the
// pipeline is the one that recorded it.
func Wrap(err error) error {
	if err == nil || coded(err) != nil {
		return err
	}

	return WrapCK(err, CodeInternal, "internal")
}

// WrapCK builds an error carrying the given code and key, with err as its
// cause. A nil err is allowed: a hook that rejects a request outright has no
// underlying failure to attach and should not have to invent one.
//
// key serves as both the machine-readable identifier (upper-cased, spaces to
// underscores) and the human-readable prefix of the message, so the two cannot
// drift apart and describe different things.
func WrapCK(err error, code Code, key string) error {
	if err == nil {
		return &Error{
			error: errors.New(key),
			key:   makeErrorKey(key),
			code:  code,
		}
	}

	return &Error{
		error: errors.Wrap(err, key),
		key:   makeErrorKey(key),
		code:  code,
	}
}

// Format implements [fmt.Formatter] by delegating to the wrapped error when it
// can format itself, so %+v on a pkg/errors chain still prints its stack trace.
//
// It never panics: formatting runs inside log and error paths, where turning a
// writer failure into a panic would replace a reportable error with a crash.
func (e *Error) Format(s fmt.State, verb rune) {
	if formatter, ok := e.error.(fmt.Formatter); ok {
		formatter.Format(s, verb)

		return
	}

	_, _ = io.WriteString(s, e.Error())
}

// Unwrap returns the wrapped error, so errors.Is and errors.As see through a
// *Error to its cause.
func (e *Error) Unwrap() error {
	return e.error
}

func makeErrorKey(msg string) string {
	return strings.ToUpper(strings.ReplaceAll(msg, " ", "_"))
}
