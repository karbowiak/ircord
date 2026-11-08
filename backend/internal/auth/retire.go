package auth

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// Deleting an account retires its username for good (see storage.DeleteAccount), and its
// account on the associated IRC server is unregistered, which keeps the name reserved there
// too. A failed unregistration stays queued and RecoverCredentials retries it.

// unregister unregisters a retired username on the associated server, reporting whether
// that succeeded. Failures are logged, not returned: the IRCord account is already gone.
func (s *Service) unregister(ctx context.Context, username string) bool {
	s.unregisterMu.Lock()
	defer s.unregisterMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := s.options.Accounts.Unregister(ctx, username); err != nil {
		slog.Error("Unregistering a deleted account on the IRC server will retry", "account", username, "error", err)
		return false
	}
	if err := s.store.FinishUnregistration(username); err != nil {
		slog.Error("Recording an IRC server unregistration failed", "account", username, "error", err)
	}
	slog.Info("Unregistered a deleted account on the IRC server", "account", username)
	return true
}

func (s *Service) unregisterRetired(ctx context.Context) error {
	if s.options.Accounts == nil {
		return nil
	}
	names, err := s.store.PendingUnregistrations()
	if err != nil {
		return ErrUnavailable
	}
	failed := 0
	for _, name := range names {
		if ctx.Err() != nil {
			break
		}
		if !s.unregister(ctx, name) {
			failed++
		}
	}
	if failed > 0 {
		return errors.New("some deleted accounts are still registered on the IRC server")
	}
	return nil
}
