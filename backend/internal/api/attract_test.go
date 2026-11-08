package api

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/irc"
)

func TestAttractErrorStatuses(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{irc.RateLimitError{Wait: 12 * time.Second, Scope: "in this channel"}, http.StatusTooManyRequests},
		{irc.ErrAttractNeedsOperator, http.StatusForbidden},
		{irc.ErrAttractNoMessage, http.StatusNotFound},
		{irc.ErrAttractUnsupported, http.StatusBadRequest},
	} {
		_, err := result(irc.Message{}, tc.err)
		var status huma.StatusError
		if !errors.As(err, &status) || status.GetStatus() != tc.status {
			t.Fatalf("%v: %v", tc.err, err)
		}
		if tc.status == http.StatusTooManyRequests && err.Error() != "You can use /attract in this channel again in 12 s." {
			t.Fatal(err.Error())
		}
		if tc.status == http.StatusNotFound && err.Error() != irc.ErrAttractNoMessage.Error() {
			t.Fatal(err.Error())
		}
	}
}
