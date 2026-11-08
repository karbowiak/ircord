package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"ircord/backend/internal/ergo"
	"ircord/backend/internal/storage"
)

type credentialOperation struct {
	Kind, Owner, Username, Hash, Password, DisplayName, KeepSession string
	// Reset marks an operator reset, so finishing it later is audited too.
	Reset bool `json:",omitempty"`
}

// The backend holds an exclusive data-directory lock. Fixed stripes serialize the
// complete credential workflow without an unbounded map of attacker-chosen names.
func (s *Service) credentialLock(username string) *sync.Mutex {
	var h uint32
	for _, b := range []byte(strings.ToLower(username)) {
		h = h*31 + uint32(b)
	}
	return &s.credentialLocks[h%uint32(len(s.credentialLocks))]
}
func (s *Service) saveOperation(op *credentialOperation) error {
	raw, err := json.Marshal(op)
	if err != nil {
		return err
	}
	return s.store.SaveCredentialOperation(strings.ToLower(op.Username), raw)
}
func (s *Service) loadOperation(username string) (*credentialOperation, error) {
	ops, err := s.store.CredentialOperations()
	if err != nil {
		return nil, err
	}
	raw := ops[strings.ToLower(username)]
	if raw == nil {
		return nil, nil
	}
	var op credentialOperation
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, err
	}
	return &op, nil
}

// Every step is idempotent. The intent is committed before touching Ergo and is
// removed only after local credentials, session revocation and saved SASL agree.
func (s *Service) finishOperation(ctx context.Context, op *credentialOperation) error {
	switch op.Kind {
	case "signup":
		if s.options.Accounts != nil {
			err := s.options.Accounts.Register(ctx, op.Username, op.Password)
			if errors.Is(err, ergo.ErrAccountExists) {
				// A lost success response is safe to recover only after proving possession
				// of the upstream password. Never reset an existing account's password here.
				valid, checkErr := s.options.Accounts.CheckPassword(ctx, op.Username, op.Password)
				if checkErr != nil {
					return ErrNetworkUnavailable
				}
				if valid {
					err = nil
				}
			}
			if err != nil {
				if errors.Is(err, ergo.ErrAccountExists) || errors.Is(err, ergo.ErrAccountRefused) || errors.Is(err, ergo.ErrInvalidPassphrase) {
					_ = s.store.DeleteCredentialOperation(op.Username)
				}
				return accountsError(err)
			}
		}
		exists, err := s.store.AccountExistsByID(op.Owner)
		if err != nil {
			return ErrUnavailable
		}
		if !exists {
			if _, err := s.store.CreateUser(op.Owner, op.Username, op.Hash); err != nil {
				if errors.Is(err, storage.ErrUsernameTaken) {
					return ErrUsernameTaken
				}
				return ErrUnavailable
			}
		}
		if s.options.Provision != nil {
			if err := s.options.Provision(op.Owner, op.Username, op.Password); err != nil {
				return ErrUnavailable
			}
		}
		if op.DisplayName != "" && s.options.SetDisplayName != nil {
			if err := s.options.SetDisplayName(op.Owner, op.DisplayName); err != nil {
				return ErrUnavailable
			}
		}
	case "password":
		exists, err := s.store.AccountExistsByID(op.Owner)
		if err != nil {
			return ErrUnavailable
		}
		if !exists {
			return s.dropOperation(op)
		}
		if s.options.Accounts != nil {
			if err := s.options.Accounts.SetPassword(ctx, op.Username, op.Password); err != nil {
				// A definitive answer won't change on retry: drop the intent so the account
				// keeps its current password instead of staying blocked behind it forever.
				if errors.Is(err, ergo.ErrInvalidPassphrase) || errors.Is(err, ergo.ErrNotFound) || errors.Is(err, ergo.ErrRefused) {
					if dropErr := s.dropOperation(op); dropErr != nil {
						return ErrUnavailable
					}
				}
				return accountsError(err)
			}
		}
		s.mu.Lock()
		sessions, err := s.store.Sessions(op.Owner)
		if err == nil {
			err = s.store.SetPassword(op.Owner, op.Hash, op.KeepSession)
		}
		// Cut every revoked session's connections, even if the change reported an error.
		for _, session := range sessions {
			if session.ID != op.KeepSession && (err == nil || !s.store.SessionActive(op.Owner, session.ID)) {
				s.cancelSession(session.ID)
			}
		}
		s.devices.notify(op.Owner)
		s.mu.Unlock()
		if err != nil {
			return ErrUnavailable
		}
		if s.options.PasswordChanged != nil {
			if err := s.options.PasswordChanged(op.Owner, op.Password); err != nil {
				return ErrUnavailable
			}
		}
	default:
		return ErrUnavailable
	}
	if err := s.store.DeleteCredentialOperation(op.Username); err != nil {
		return ErrUnavailable
	}
	if op.Reset {
		s.audit(op.Username, ResetCompleted)
	}
	return nil
}

// dropOperation abandons an intent that can never finish. An abandoned reset is audited.
func (s *Service) dropOperation(op *credentialOperation) error {
	if err := s.store.DeleteCredentialOperation(op.Username); err != nil {
		return err
	}
	if op.Reset {
		s.audit(op.Username, ResetFailed)
	}
	return nil
}

// RecoverCredentials retries durable operations at startup and during maintenance.
// An unavailable upstream leaves the encrypted intent available for a later retry.
func (s *Service) RecoverCredentials(ctx context.Context) error {
	ops, err := s.store.CredentialOperations()
	if err != nil {
		return ErrUnavailable
	}
	var result error
	for username := range ops {
		lock := s.credentialLock(username)
		lock.Lock()
		op, err := s.loadOperation(username)
		if err == nil && op != nil {
			err = s.finishOperation(ctx, op)
		}
		lock.Unlock()
		result = errors.Join(result, err)
	}
	return errors.Join(result, s.unregisterRetired(ctx))
}
