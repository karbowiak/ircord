package app

import (
	"context"
	"strings"
	"testing"

	"ircord/backend/internal/storage"
)

// New accounts join the configured defaults until owners save a guide, then only the guide's
// channels for everyone; suggested and listed channels wait for the welcome dialog.
func TestSignupChannelsFollowTheGuide(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	defaults := []string{"#lobby", "#help"}
	if got := strings.Join(signupChannels(store, defaults), ","); got != "#lobby,#help" {
		t.Fatalf("before a guide: %s", got)
	}
	guide := `{"groups":[],"entries":[{"channel":"#news","state":"everyone"},{"channel":"#eve","state":"suggested"},{"channel":"#food","state":"listed"}]}`
	if _, err = store.SaveChannelGuide(context.Background(), []byte(guide), 0, "owner", nil); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(signupChannels(store, defaults), ","); got != "#news" {
		t.Fatalf("with a guide: %s", got)
	}
}
