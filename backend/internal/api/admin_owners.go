package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/admin"
	"ircord/backend/internal/storage"
)

// The operator's trusted path to server ownership (#165): granting an owner whose account was
// created after IRCORD_OWNERS named it, and recovering when no owner is left. Bearer-token only.
func (s *Server) registerOwnerAdmin() {
	huma.Register(s.API, huma.Operation{OperationID: "admin-owners", Method: http.MethodGet, Path: admin.Prefix + "/owners", Summary: "List server owners"}, func(ctx context.Context, _ *struct{}) (*Output[Items[storage.AdminUser]], error) {
		if s.options.Store == nil {
			return nil, huma.Error501NotImplemented("this backend has no IRCord accounts")
		}
		owners := []storage.AdminUser{}
		for offset := 0; ; offset += 100 {
			page, more, err := s.options.Store.AdminUsers(ctx, "", offset, 100)
			if err != nil {
				return nil, huma.Error503ServiceUnavailable("account storage is unavailable")
			}
			for _, u := range page {
				if u.Owner {
					owners = append(owners, u)
				}
			}
			// Owners sort first, so the first page without one ends the list.
			if !more || len(page) == 0 || !page[len(page)-1].Owner {
				break
			}
		}
		return result(Items[storage.AdminUser]{owners}, nil)
	})
	huma.Register(s.API, huma.Operation{OperationID: "admin-grant-owner", Method: http.MethodPut, Path: admin.Prefix + "/owners/{username}", Summary: "Make an existing account a server owner"}, func(ctx context.Context, in *adminUsernameInput) (*Output[storage.AdminUser], error) {
		user, err := s.operatorTarget(ctx, in.Username)
		if err != nil {
			return nil, err
		}
		if err = s.options.Store.GrantOwner(user.ID, storage.OwnerFromOperator, "operator"); err != nil {
			_ = s.options.Store.RecordAdminEvent("owner-grant", user.Username, "failed")
			return nil, huma.Error503ServiceUnavailable("account storage is unavailable")
		}
		_ = s.options.Store.RecordAdminEvent("owner-grant", user.Username, "granted")
		return result(s.options.Store.AdminUser(ctx, user.ID))
	})
	huma.Register(s.API, huma.Operation{OperationID: "admin-revoke-owner", Method: http.MethodDelete, Path: admin.Prefix + "/owners/{username}", Summary: "Remove a server owner"}, func(ctx context.Context, in *adminUsernameInput) (*Output[storage.AdminUser], error) {
		user, err := s.operatorTarget(ctx, in.Username)
		if err != nil {
			return nil, err
		}
		if err = s.options.Store.RevokeOwner(user.ID, false); errors.Is(err, storage.ErrOwnerFromEnvironment) {
			return nil, huma.Error409Conflict("This owner is set in IRCORD_OWNERS. Remove them there and restart the backend.")
		} else if errors.Is(err, storage.ErrLastOwner) {
			return nil, huma.Error409Conflict("This is the last owner. Grant another owner first.")
		} else if err != nil {
			return nil, huma.Error503ServiceUnavailable("account storage is unavailable")
		}
		_ = s.options.Store.RecordAdminEvent("owner-revoke", user.Username, "revoked")
		return result(s.options.Store.AdminUser(ctx, user.ID))
	})
}

type adminUsernameInput struct {
	Username string `path:"username" minLength:"1" maxLength:"64" pattern:"^[^\\s\\x00-\\x1f\\x7f]+$"`
}

func (s *Server) operatorTarget(ctx context.Context, username string) (storage.AdminUser, error) {
	if s.options.Store == nil {
		return storage.AdminUser{}, huma.Error501NotImplemented("this backend has no IRCord accounts")
	}
	user, err := s.options.Store.AdminUser(ctx, username)
	if errors.Is(err, storage.ErrNotFound) || err == nil && !equalFoldASCII(user.Username, username) {
		return user, huma.Error404NotFound("no IRCord account has that username")
	}
	if err != nil {
		return user, huma.Error503ServiceUnavailable("account storage is unavailable")
	}
	return user, nil
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}
