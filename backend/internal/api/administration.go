package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/admin"
	"ircord/backend/internal/auth"
	"ircord/backend/internal/ergo"
	"ircord/backend/internal/storage"
)

// The Administration panel (#165): server owners use their ordinary signed-in session, and
// every request checks ownership in the database, so a revocation takes effect immediately.
// This is separate from /api/v1/admin, the operator bearer-token API, which never takes a
// browser session.
const administrationPrefix = "/api/v1/administration"

type AdministrationAccess struct {
	Owner bool `json:"owner"`
}
type AdminUserPage struct {
	Items []storage.AdminUser `json:"items" nullable:"false"`
	More  bool                `json:"more"`
}

// IdentityMatch is one autocomplete result for choosing an account: an IRCord account, and the
// nickname it's using on the associated server when one was looked up. Only an eligible match
// carries a user to submit.
type IdentityMatch struct {
	Nickname string             `json:"nickname,omitempty"`
	Account  string             `json:"account,omitempty"`
	User     *storage.AdminUser `json:"user,omitempty"`
	Eligible bool               `json:"eligible"`
	Note     string             `json:"note,omitempty"`
}
type OwnerGrant struct {
	Account string `json:"account" minLength:"1" maxLength:"100" doc:"The IRCord account ID to grant, as returned by the user list or identity lookup"`
	Reason  string `json:"reason,omitempty" maxLength:"500"`
}

// requireOwner returns the signed-in server owner, or an error for everyone else.
func (s *Server) requireOwner(ctx context.Context) (auth.Principal, error) {
	if s.options.Auth == nil || s.options.Store == nil {
		return auth.Principal{}, huma.Error404NotFound("this backend has no IRCord accounts")
	}
	p := principal(ctx)
	owner, err := s.options.Store.IsOwner(p.Account.ID)
	if err != nil {
		return p, huma.Error503ServiceUnavailable("account storage is unavailable")
	}
	if !owner {
		return p, huma.Error403Forbidden("Only server owners can use Administration.")
	}
	return p, nil
}

func (s *Server) audit(actor auth.Principal, action, target, reason, outcome string) {
	_ = s.options.Store.RecordOwnerEvent(storage.OwnerEvent{Actor: actor.Account.Username, Action: action, Target: target, Reason: strings.TrimSpace(reason), Outcome: outcome})
}

func (s *Server) registerAdministration() {
	s.registerDatabaseAdministration()
	huma.Register(s.API, huma.Operation{OperationID: "administration-access", Method: http.MethodGet, Path: administrationPrefix + "/access", Summary: "Whether the signed-in account can use Administration"}, func(ctx context.Context, _ *struct{}) (*Output[AdministrationAccess], error) {
		if s.options.Auth == nil || s.options.Store == nil {
			return result(AdministrationAccess{}, nil)
		}
		owner, err := s.options.Store.IsOwner(principal(ctx).Account.ID)
		if err != nil {
			return nil, huma.Error503ServiceUnavailable("account storage is unavailable")
		}
		return result(AdministrationAccess{Owner: owner}, nil)
	})
	huma.Register(s.API, huma.Operation{OperationID: "administration-status", Method: http.MethodGet, Path: administrationPrefix + "/status", Summary: "Server status for owners"}, func(ctx context.Context, _ *struct{}) (*Output[admin.Status], error) {
		if _, err := s.requireOwner(ctx); err != nil {
			return nil, err
		}
		return result(s.adminStatus(ctx), nil)
	})
	huma.Register(s.API, huma.Operation{OperationID: "administration-users", Method: http.MethodGet, Path: administrationPrefix + "/users", Summary: "List IRCord accounts for owners"}, func(ctx context.Context, in *struct {
		Query  string `query:"query" maxLength:"64"`
		Offset int    `query:"offset" minimum:"0" maximum:"100000"`
		Limit  int    `query:"limit" minimum:"1" maximum:"100" default:"50"`
	}) (*Output[AdminUserPage], error) {
		if _, err := s.requireOwner(ctx); err != nil {
			return nil, err
		}
		items, more, err := s.options.Store.AdminUsers(ctx, strings.TrimSpace(in.Query), in.Offset, in.Limit)
		return result(AdminUserPage{items, more}, err)
	})
	huma.Register(s.API, huma.Operation{
		OperationID: "administration-identities", Method: http.MethodGet, Path: administrationPrefix + "/identities",
		Summary:     "Find an account by username or by the nickname it uses",
		Description: "Matches IRCord usernames, and with an associated server also looks the text up as a nickname to find the account behind it. Nicknames that aren't logged in, or whose account has no IRCord account, are returned as ineligible with an explanation.",
	}, func(ctx context.Context, in *struct {
		Query string `query:"query" minLength:"1" maxLength:"64"`
	}) (*Output[Items[IdentityMatch]], error) {
		if _, err := s.requireOwner(ctx); err != nil {
			return nil, err
		}
		matches, err := s.identities(ctx, strings.TrimSpace(in.Query))
		return result(Items[IdentityMatch]{matches}, err)
	})
	huma.Register(s.API, huma.Operation{OperationID: "administration-grant-owner", Method: http.MethodPost, Path: administrationPrefix + "/owners", Summary: "Make an account a server owner", MaxBodyBytes: 4096}, func(ctx context.Context, in *struct{ Body OwnerGrant }) (*Output[storage.AdminUser], error) {
		actor, err := s.requireOwner(ctx)
		if err != nil {
			return nil, err
		}
		target, err := s.options.Store.AdminUser(ctx, in.Body.Account)
		if errors.Is(err, storage.ErrNotFound) || err == nil && target.ID != in.Body.Account {
			return nil, huma.Error404NotFound("That account doesn't exist.")
		}
		if err != nil {
			return nil, huma.Error503ServiceUnavailable("account storage is unavailable")
		}
		if err = s.options.Store.GrantOwner(target.ID, storage.OwnerFromOwner, actor.Account.Username); err != nil {
			s.audit(actor, "owner-grant", target.Username, in.Body.Reason, "failed")
			return nil, huma.Error503ServiceUnavailable("account storage is unavailable")
		}
		s.audit(actor, "owner-grant", target.Username, in.Body.Reason, "granted")
		return result(s.options.Store.AdminUser(ctx, target.ID))
	})
	huma.Register(s.API, huma.Operation{OperationID: "administration-revoke-owner", Method: http.MethodDelete, Path: administrationPrefix + "/owners/{account}", Summary: "Remove a server owner"}, func(ctx context.Context, in *struct {
		Account string `path:"account" minLength:"1" maxLength:"100"`
		Reason  string `query:"reason" maxLength:"500"`
	}) (*Output[storage.AdminUser], error) {
		actor, err := s.requireOwner(ctx)
		if err != nil {
			return nil, err
		}
		target, err := s.options.Store.AdminUser(ctx, in.Account)
		if errors.Is(err, storage.ErrNotFound) || err == nil && target.ID != in.Account {
			return nil, huma.Error404NotFound("That account doesn't exist.")
		}
		if err != nil {
			return nil, huma.Error503ServiceUnavailable("account storage is unavailable")
		}
		switch err = s.options.Store.RevokeOwner(target.ID, false); {
		case errors.Is(err, storage.ErrLastOwner):
			s.audit(actor, "owner-revoke", target.Username, in.Reason, "refused: last owner")
			return nil, huma.Error409Conflict("The server needs at least one owner. Make someone else an owner first.")
		case errors.Is(err, storage.ErrOwnerFromEnvironment):
			s.audit(actor, "owner-revoke", target.Username, in.Reason, "refused: configured")
			return nil, huma.Error409Conflict("This owner is set in the server's IRCORD_OWNERS setting. Remove them there and restart the backend.")
		case err != nil:
			return nil, huma.Error503ServiceUnavailable("account storage is unavailable")
		}
		s.audit(actor, "owner-revoke", target.Username, in.Reason, "revoked")
		return result(s.options.Store.AdminUser(ctx, target.ID))
	})
	huma.Register(s.API, huma.Operation{OperationID: "administration-audit", Method: http.MethodGet, Path: administrationPrefix + "/audit", Summary: "Recent Administration actions"}, func(ctx context.Context, in *struct {
		Limit int `query:"limit" minimum:"1" maximum:"200" default:"50"`
	}) (*Output[Items[storage.OwnerEvent]], error) {
		if _, err := s.requireOwner(ctx); err != nil {
			return nil, err
		}
		events, err := s.options.Store.OwnerEvents(ctx, in.Limit)
		return result(Items[storage.OwnerEvent]{events}, err)
	})
}

// identities matches IRCord usernames, then resolves the text as a nickname on the associated
// server, so an owner can find someone by the nick they see in a channel.
func (s *Server) identities(ctx context.Context, query string) ([]IdentityMatch, error) {
	users, _, err := s.options.Store.AdminUsers(ctx, query, 0, 8)
	if err != nil {
		return nil, err
	}
	out := make([]IdentityMatch, 0, len(users)+1)
	for i := range users {
		out = append(out, IdentityMatch{Account: users[i].Username, User: &users[i], Eligible: true})
	}
	if s.options.Ergo == nil || strings.ContainsAny(query, " ,*?!@#") {
		return out, nil
	}
	who, err := s.options.Ergo.User(ctx, query)
	if errors.Is(err, ergo.ErrNotFound) || err != nil {
		// Offline nicknames and an unreachable server still leave the username matches.
		return out, nil
	}
	if who.Account == "" {
		return append([]IdentityMatch{{Nickname: who.Nickname, Note: who.Nickname + " is online but not logged in to an account."}}, out...), nil
	}
	user, err := s.options.Store.AdminUser(ctx, who.Account)
	if err != nil || !strings.EqualFold(user.Username, who.Account) {
		return append([]IdentityMatch{{Nickname: who.Nickname, Account: who.Account, Note: "The account " + who.Account + " has no IRCord account."}}, out...), nil
	}
	match := IdentityMatch{Nickname: who.Nickname, Account: user.Username, User: &user, Eligible: true}
	if !strings.EqualFold(who.Nickname, user.Username) {
		match.Note = who.Nickname + " is logged in as " + user.Username + "."
	}
	rest := []IdentityMatch{match}
	for _, m := range out {
		if m.User.ID != user.ID {
			rest = append(rest, m)
		}
	}
	return rest, nil
}
