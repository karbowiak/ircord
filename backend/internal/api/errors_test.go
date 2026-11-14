package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/irc"
)

func TestCommandFailuresKeepHTTPStatusAndMessage(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{irc.ErrStorage, http.StatusServiceUnavailable},
		{irc.ErrUncertain, http.StatusConflict},
		{irc.ErrConflict, http.StatusConflict},
		{irc.ErrSavedLimit, http.StatusConflict},
		{irc.ErrNotFound, http.StatusNotFound},
		{irc.ErrRateLimited, http.StatusTooManyRequests},
		{irc.ErrForbidden, http.StatusForbidden},
		{irc.ErrRedactRefused, http.StatusForbidden},
		{irc.ErrEditForbidden, http.StatusForbidden},
		{irc.ErrEditRefused, http.StatusForbidden},
		{irc.ErrNotChannelOperator, http.StatusForbidden},
		{irc.ErrAttractNeedsOperator, http.StatusForbidden},
		{irc.ErrEditUnchanged, http.StatusUnprocessableEntity},
		{irc.ErrAlreadyDeleted, http.StatusConflict},
		{irc.ErrRedactUnconfirmed, http.StatusConflict},
		{irc.ErrEditUnconfirmed, http.StatusConflict},
		{irc.ErrDisconnected, http.StatusConflict},
		{irc.ErrBusy, http.StatusConflict},
		{irc.ErrUnsupported, http.StatusBadRequest},
		{errors.New("invalid input"), http.StatusBadRequest},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			wrapped := fmt.Errorf("command failed: %w", tc.err)
			body, err := result("ignored", wrapped)
			if body != nil {
				t.Fatal("failed command returned a response body")
			}
			_, emptyErr := empty(wrapped)
			for _, mapped := range []error{err, emptyErr} {
				var status huma.StatusError
				if !errors.As(mapped, &status) || status.GetStatus() != tc.status || mapped.Error() != wrapped.Error() {
					t.Fatalf("got %v; want HTTP %d with the original message", mapped, tc.status)
				}
			}
		})
	}
	if body, err := result("accepted", nil); err != nil || body == nil || body.Body != "accepted" {
		t.Fatal("successful command lost its response")
	}
	if body, err := empty(nil); err != nil || body != nil {
		t.Fatal("empty success must return no body")
	}
}

func TestCommandFailureProblemTypes(t *testing.T) {
	for _, tc := range []struct {
		err     error
		status  int
		typeURI string
	}{
		{irc.ErrClientIDConflict, http.StatusConflict, problemOperationIDConflict},
		{irc.ErrUncertain, http.StatusConflict, problemDeliveryUncertain},
		{irc.ErrBusy, http.StatusConflict, problemOperationBusy},
		{irc.ErrDisconnected, http.StatusConflict, problemConnectionUnavailable},
		{irc.ErrConflict, http.StatusConflict, problemResourceConflict},
		{irc.ErrPurgeRunning, http.StatusConflict, problemResourceConflict},
	} {
		mapped := commandFailure(tc.err)
		status, ok := mapped.(huma.StatusError)
		model, typed := mapped.(*huma.ErrorModel)
		if !ok || !typed || status.GetStatus() != tc.status || model.Type != tc.typeURI || model.Detail != tc.err.Error() {
			t.Fatalf("%v mapped to %#v; want %d %s", tc.err, mapped, tc.status, tc.typeURI)
		}
	}
}

func TestCommandFailureCancellationIsSafe(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{context.Canceled, http.StatusServiceUnavailable},
		{context.DeadlineExceeded, http.StatusInternalServerError},
	} {
		mapped := commandFailure(fmt.Errorf("internal cancellation detail: %w", tc.err))
		status, ok := mapped.(huma.StatusError)
		if !ok || status.GetStatus() != tc.status || strings.Contains(mapped.Error(), "internal cancellation") {
			t.Fatalf("%v mapped unsafely to %v", tc.err, mapped)
		}
		if errors.Is(tc.err, context.Canceled) && mapped.Error() != "The request was canceled." {
			t.Fatalf("cancellation returned unexpected detail %q", mapped.Error())
		}
	}
}
