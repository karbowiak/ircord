package app

import (
	"context"
	"log/slog"
	"time"

	"ircord/backend/internal/channelguide"
	"ircord/backend/internal/storage"
)

// signupChannels are the channels a new account joins on the home network: the channel
// guide's channels for everyone, or the configured defaults while no guide is saved (#258).
// Optional guide channels are chosen in the welcome dialog instead, so nobody is joined to a
// channel and then parted again. An unreadable guide falls back to the defaults rather than
// failing the signup.
func signupChannels(store *storage.Store, defaults []string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	guide, _, _, err := channelguide.Load(ctx, store, defaults)
	if err != nil {
		slog.Error("Reading the channel guide for a new account failed; joining the default channels", "error", err)
		return defaults
	}
	return guide.EveryoneChannels()
}
