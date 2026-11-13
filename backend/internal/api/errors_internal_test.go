package api

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
)

// Errors from files, the network or the database must not reach the client with their text;
// the IRC layer's own messages still do.
func TestCommandFailureHidesInternalErrorText(t *testing.T) {
	internal := []error{
		fmt.Errorf("save: %w", &fs.PathError{Op: "open", Path: "/data/ircord.db", Err: fs.ErrPermission}),
		fmt.Errorf("upload: %w", &net.OpError{Op: "dial", Net: "tcp", Addr: &net.TCPAddr{IP: net.IPv4(10, 0, 0, 5), Port: 9000}, Err: errors.New("connection refused")}),
	}
	for _, err := range internal {
		status, ok := commandFailure(err).(huma.StatusError)
		if !ok || status.GetStatus() != 500 || strings.Contains(status.Error(), "10.0.0.5") || strings.Contains(status.Error(), "/data") {
			t.Fatalf("%v answered as %v", err, status)
		}
	}
	user := errors.New("this channel is moderated; voice or operator privileges are required")
	status, ok := commandFailure(user).(huma.StatusError)
	if !ok || status.GetStatus() != 400 || !strings.Contains(status.Error(), "moderated") {
		t.Fatalf("user-facing error answered as %v", status)
	}
}
