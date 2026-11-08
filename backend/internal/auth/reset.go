package auth

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"ircord/backend/internal/storage"
)

// ErrNoAccount means an operator named a username that has no live IRCord account.
var ErrNoAccount = errors.New("no IRCord account has that username")

// Operator password reset outcomes, as recorded in the audit log and returned to the operator.
const (
	ResetCompleted = "completed"
	ResetPending   = "pending"
	ResetFailed    = "failed"
)

// PasswordReset is an operator's new password for an existing account. Its length is checked
// by PasswordProblem rather than the schema, so a rejection is readable and audited; the
// request body limit bounds it.
type PasswordReset struct {
	Password string `json:"password" minLength:"1" writeOnly:"true"`
}

// PasswordResetResult reports an operator reset. It never contains credential material.
type PasswordResetResult struct {
	Username        string `json:"username" doc:"The account's username, as stored."`
	Status          string `json:"status" enum:"completed,pending" doc:"completed: IRCord, the associated IRC server and the saved IRC login all use the new password. pending: the reset is saved and the backend keeps retrying the remaining steps; sign-in stays blocked until they finish."`
	SessionsRevoked int    `json:"sessionsRevoked" minimum:"0" doc:"Signed-in IRCord sessions that this reset signed out."`
	Associated      bool   `json:"associated" doc:"Whether the account also has an account on the associated IRC server that the reset changes."`
	Detail          string `json:"detail,omitempty" doc:"Why a reset is still pending."`
}

// ResetPassword sets a new password for an existing account without its current password.
// It reuses the durable password operation: the intent is saved encrypted first, then the
// associated IRC server, the local hash and the saved IRC login are brought to the new value.
// Every session is signed out before any upstream call, so prior sessions end even when the
// rest has to be retried. A reset that can't finish yet is reported as pending, never completed.
func (s *Service) ResetPassword(ctx context.Context, username string, in PasswordReset) (PasswordResetResult, error) {
	username = strings.TrimSpace(username)
	if reason := PasswordProblem(in.Password); reason != "" {
		s.audit(username, ResetFailed)
		return PasswordResetResult{}, InvalidError{reason}
	}
	lock := s.credentialLock(username)
	lock.Lock()
	defer lock.Unlock()
	pending, err := s.loadOperation(username)
	if err != nil {
		s.audit(username, ResetFailed)
		return PasswordResetResult{}, ErrUnavailable
	}
	// A pending password change is superseded: the reset rewrites the same idempotent intent.
	// An unfinished signup has to finish first, because the account may not exist yet.
	if pending != nil && pending.Kind != "password" {
		if err := s.finishOperation(ctx, pending); err != nil {
			s.audit(username, ResetFailed)
			return PasswordResetResult{}, err
		}
	}
	account, _, err := s.store.AccountByUsername(username)
	if errors.Is(err, storage.ErrNotFound) {
		s.audit(username, ResetFailed)
		return PasswordResetResult{}, ErrNoAccount
	} else if err != nil {
		s.audit(username, ResetFailed)
		return PasswordResetResult{}, ErrUnavailable
	}
	hash, err := hashPassword(in.Password)
	if err != nil {
		s.audit(account.Username, ResetFailed)
		return PasswordResetResult{}, ErrUnavailable
	}
	op := &credentialOperation{Kind: "password", Owner: account.ID, Username: account.Username, Hash: hash, Password: in.Password, Reset: true}
	if err := s.saveOperation(op); err != nil {
		s.audit(account.Username, ResetFailed)
		return PasswordResetResult{}, ErrUnavailable
	}
	result := PasswordResetResult{Username: account.Username, Status: ResetCompleted, Associated: s.options.Accounts != nil}
	if result.SessionsRevoked, err = s.revokeAll(account.ID); err != nil {
		// The saved intent still revokes every session when it finishes.
		result.Status, result.Detail = ResetPending, "Signing out existing sessions failed; the backend retries the reset."
		s.audit(account.Username, ResetPending)
		return result, nil
	}
	// finishOperation audits completion, and dropping an intent that can never finish.
	err = s.finishOperation(ctx, op)
	var invalid InvalidError
	switch {
	case err == nil:
		return result, nil
	case errors.As(err, &invalid), errors.Is(err, ErrNoUpstreamAccount), errors.Is(err, ErrUpstreamRefused):
		// The associated server gave a definitive answer and the intent was dropped: the
		// account keeps its current password.
		return PasswordResetResult{}, err
	case errors.Is(err, ErrNetworkUnavailable):
		result.Detail = "The associated IRC server couldn’t be reached; the backend keeps retrying."
	default:
		result.Detail = "Saving the new password didn’t finish; the backend keeps retrying."
	}
	result.Status = ResetPending
	s.audit(account.Username, ResetPending)
	return result, nil
}

// revokeAll signs out every session of an account and closes its live connections.
func (s *Service) revokeAll(owner string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sessions, err := s.store.Sessions(owner)
	if err != nil {
		return 0, err
	}
	revoked := 0
	defer s.devices.notify(owner)
	for _, session := range sessions {
		if !session.Revoked {
			if err := s.store.RevokeSession(owner, session.ID); err != nil {
				// Never leave a revoked session's connections open, whatever was reported.
				if !s.store.SessionActive(owner, session.ID) {
					s.cancelSession(session.ID)
					s.sessionEnded(owner, session.ID)
				}
				return revoked, err
			}
			revoked++
		}
		s.cancelSession(session.ID)
		s.sessionEnded(owner, session.ID)
	}
	return revoked, nil
}

// audit records an operator reset's target, time and outcome. It never records credentials.
func (s *Service) audit(target, outcome string) {
	slog.Info("Operator password reset", "target", target, "outcome", outcome)
	if err := s.store.RecordAdminEvent("password-reset", target, outcome); err != nil {
		slog.Error("Recording an operator audit event failed", "action", "password-reset", "error", err)
	}
}
