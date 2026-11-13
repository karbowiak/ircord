package api

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/url"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/irc"
)

func result[T any](value T, err error) (*Output[T], error) {
	if err != nil {
		return nil, commandFailure(err)
	}
	return &Output[T]{Body: value}, nil
}

// commandFailure translates IRC command failures at the HTTP boundary.
func commandFailure(err error) error {
	var service *irc.ServiceError
	if errors.As(err, &service) {
		return huma.NewError(service.Status, service.Detail)
	}
	switch {
	case err == nil:
		return nil
	case errors.Is(err, irc.ErrStorage):
		return huma.Error503ServiceUnavailable(err.Error())
	case errors.Is(err, irc.ErrUncertain), errors.Is(err, irc.ErrConflict), errors.Is(err, irc.ErrSavedLimit):
		return huma.Error409Conflict(err.Error())
	case errors.Is(err, irc.ErrNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, irc.ErrRateLimited):
		return huma.Error429TooManyRequests(err.Error())
	case errors.Is(err, irc.ErrForbidden), errors.Is(err, irc.ErrRedactRefused), errors.Is(err, irc.ErrEditForbidden),
		errors.Is(err, irc.ErrEditRefused), errors.Is(err, irc.ErrNotChannelOperator), errors.Is(err, irc.ErrAttractNeedsOperator):
		return huma.Error403Forbidden(err.Error())
	case errors.Is(err, irc.ErrEditUnchanged):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, irc.ErrAlreadyDeleted), errors.Is(err, irc.ErrRedactUnconfirmed), errors.Is(err, irc.ErrEditUnconfirmed),
		errors.Is(err, irc.ErrDisconnected), errors.Is(err, irc.ErrBusy):
		return huma.Error409Conflict(err.Error())
	case internalCause(err):
		// Its text can name files, addresses or database details; log it instead.
		slog.Error("Command failed", "error", err)
		return huma.Error500InternalServerError("Something went wrong. Please try again.")
	default:
		// The IRC layer's own errors are sentences written for the user.
		return huma.Error400BadRequest(err.Error())
	}
}

// internalCause reports an error that comes from the system rather than from the request:
// files, the network, the database or a deadline.
func internalCause(err error) bool {
	var path *fs.PathError
	var network net.Error
	var address *url.Error
	var database interface{ Code() int }
	return errors.As(err, &path) || errors.As(err, &network) || errors.As(err, &address) ||
		errors.As(err, &database) || errors.Is(err, sql.ErrConnDone) || errors.Is(err, sql.ErrTxDone) ||
		errors.Is(err, context.DeadlineExceeded)
}

// Huma echoes the offending input of a validation failure back in errors[].value. Request
// bodies carry passwords and other secrets, so body values are never echoed; the message and
// location still say what was wrong.
func init() {
	base := huma.NewError
	huma.NewError = func(status int, msg string, errs ...error) huma.StatusError {
		err := base(status, msg, errs...)
		if model, ok := err.(*huma.ErrorModel); ok {
			for _, detail := range model.Errors {
				if detail != nil && strings.HasPrefix(detail.Location, "body") {
					detail.Value = nil
				}
			}
		}
		return err
	}
}
