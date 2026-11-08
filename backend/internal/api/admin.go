package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/admin"
	"ircord/backend/internal/auth"
	"ircord/backend/internal/ergo"
)

func (s *Server) registerAdmin() {
	huma.Register(s.API, huma.Operation{OperationID: "admin-capabilities", Method: http.MethodGet, Path: admin.Prefix + "/capabilities", Summary: "Discover configured administration features"}, func(ctx context.Context, _ *struct{}) (*Output[admin.AdminCapabilities], error) {
		features := []string{}
		if s.options.Ergo != nil {
			features = append(features, "ergo")
		}
		return result(admin.AdminCapabilities{Features: features}, nil)
	})
	huma.Register(s.API, huma.Operation{OperationID: "admin-doctor", Method: http.MethodGet, Path: admin.Prefix + "/doctor", Summary: "Check backend and optional associated-server health"}, func(ctx context.Context, _ *struct{}) (*Output[admin.Doctor], error) {
		status := s.adminStatus(ctx)
		checks := []admin.DoctorCheck{{Name: "backend", Status: "ok", Detail: "Administration API authenticated and reachable"}, {Name: "database", Status: "ok", Detail: "Database and aggregate counters are available"}}
		if status.Database != "available" {
			checks[1].Status = "unavailable"
			checks[1].Detail = "Database health check failed"
		}
		if status.Ergo != nil {
			check := admin.DoctorCheck{Name: "ergo", Status: status.Ergo.State, Detail: "Associated server API reachable; transport authenticated"}
			if status.Ergo.State != "ok" {
				check.Detail = status.Ergo.Error
			}
			checks = append(checks, check)
		}
		return result(admin.Doctor{Status: status.Status, Checks: checks}, nil)
	})
	huma.Register(s.API, huma.Operation{
		OperationID: "admin-status", Method: http.MethodGet, Path: admin.Prefix + "/status",
		Summary: "Inspect backend status", Description: "Operator-only aggregate backend health, with optional associated-Ergo statistics. An unassociated backend omits Ergo entirely; failed checks retain the available sections and mark status degraded. No credentials or private API location is returned.",
		Security: []map[string][]string{{"adminBearer": {}}},
	}, func(ctx context.Context, _ *struct{}) (*Output[admin.Status], error) {
		return result(s.adminStatus(ctx), nil)
	})
	s.registerErgo()
	s.registerAccountAdmin()
	s.registerOwnerAdmin()
}

type adminPasswordResetInput struct {
	Username string `path:"username" minLength:"1" maxLength:"64" pattern:"^[^\\s\\x00-\\x1f\\x7f]+$"`
	Body     auth.PasswordReset
}

// resetTimeout bounds the upstream part of a reset. Whatever doesn't finish in time stays
// saved and is finished by credential recovery, so the reset reports pending instead.
const resetTimeout = 8 * time.Second

func (s *Server) registerAccountAdmin() {
	huma.Register(s.API, huma.Operation{
		OperationID: "admin-reset-password", Method: http.MethodPost, Path: admin.Prefix + "/accounts/{username}/password", MaxBodyBytes: 4096,
		Summary:     "Reset an account's password",
		Description: "Operator-only. Sets a new password for an existing IRCord account without its current password, using the same rules as Account settings. IRCord and the associated IRC server account converge on the new value, the home network's saved IRC login follows when it still signs in as this account, and every session of the account is signed out. A reset that can't finish yet is saved and retried by the backend, and reported as pending. Passwords are never returned.",
	}, func(ctx context.Context, in *adminPasswordResetInput) (*Output[auth.PasswordResetResult], error) {
		if s.options.Auth == nil {
			return nil, huma.Error501NotImplemented("this backend has no IRCord accounts")
		}
		ctx, cancel := context.WithTimeout(ctx, resetTimeout)
		defer cancel()
		v, err := s.options.Auth.ResetPassword(ctx, in.Username, in.Body)
		if errors.Is(err, auth.ErrNoAccount) {
			return nil, huma.Error404NotFound(capitalize(err.Error()))
		}
		if err != nil {
			return nil, authError(err)
		}
		return result(v, nil)
	})
}

func (s *Server) adminStatus(ctx context.Context) admin.Status {
	status := admin.Status{Status: "ok", Database: "available", StartedAt: s.startedAt, UptimeSeconds: max(0, int64(time.Since(s.startedAt).Seconds())), Info: s.options.Build}
	check, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if s.options.Store == nil {
		status.Status, status.Database = "degraded", "unavailable"
	} else {
		overview, err := s.options.Store.AdminOverview(check)
		if err != nil {
			status.Status, status.Database = "degraded", "unavailable"
		} else {
			overview.IRCConnections = s.Manager.ConnectedCount()
			status.Overview = &overview
		}
	}
	if s.options.Ergo != nil {
		v := s.options.Ergo.Status(ctx)
		status.Ergo = &v
		if v.State != "ok" {
			status.Status = "degraded"
		}
	}
	return status
}

type ergoIdentityInput struct {
	Name string `path:"name" minLength:"1" maxLength:"100" pattern:"^[^\\s\\x00-\\x1f\\x7f]+$"`
}

func (s *Server) requireErgo() error {
	if s.options.Ergo == nil {
		return huma.Error404NotFound("this backend has no associated server integration")
	}
	return nil
}
func ergoResult[T any](v T, err error) (*Output[T], error) {
	if errors.Is(err, ergo.ErrNotFound) {
		return nil, huma.Error404NotFound(err.Error())
	}
	if err != nil {
		return nil, huma.Error503ServiceUnavailable(err.Error())
	}
	return result(v, nil)
}
func (s *Server) registerErgo() {
	huma.Register(s.API, huma.Operation{OperationID: "admin-ergo-status", Method: http.MethodGet, Path: admin.Prefix + "/ergo/status", Summary: "Inspect the associated Ergo server"}, func(ctx context.Context, _ *struct{}) (*Output[ergo.ErgoStatus], error) {
		if err := s.requireErgo(); err != nil {
			return nil, err
		}
		return result(s.options.Ergo.Status(ctx), nil)
	})
	huma.Register(s.API, huma.Operation{OperationID: "admin-ergo-channels", Method: http.MethodGet, Path: admin.Prefix + "/ergo/channels", Summary: "List associated-server channels"}, func(ctx context.Context, _ *struct{}) (*Output[Items[ergo.ErgoChannel]], error) {
		if err := s.requireErgo(); err != nil {
			return nil, err
		}
		v, err := s.options.Ergo.Channels(ctx)
		return ergoResult(Items[ergo.ErgoChannel]{Items: v}, err)
	})
	huma.Register(s.API, huma.Operation{OperationID: "admin-ergo-user", Method: http.MethodGet, Path: admin.Prefix + "/ergo/users/{name}", Summary: "Inspect an associated-server nickname"}, func(ctx context.Context, in *ergoIdentityInput) (*Output[ergo.ErgoUser], error) {
		if err := s.requireErgo(); err != nil {
			return nil, err
		}
		return ergoResult(s.options.Ergo.User(ctx, in.Name))
	})
	huma.Register(s.API, huma.Operation{OperationID: "admin-ergo-accounts", Method: http.MethodGet, Path: admin.Prefix + "/ergo/accounts", Summary: "List associated-server NickServ accounts"}, func(ctx context.Context, _ *struct{}) (*Output[Items[ergo.ErgoAccount]], error) {
		if err := s.requireErgo(); err != nil {
			return nil, err
		}
		v, err := s.options.Ergo.Accounts(ctx)
		return ergoResult(Items[ergo.ErgoAccount]{Items: v}, err)
	})
	huma.Register(s.API, huma.Operation{OperationID: "admin-ergo-account", Method: http.MethodGet, Path: admin.Prefix + "/ergo/accounts/{name}", Summary: "Inspect an associated-server NickServ account"}, func(ctx context.Context, in *ergoIdentityInput) (*Output[ergo.ErgoAccount], error) {
		if err := s.requireErgo(); err != nil {
			return nil, err
		}
		return ergoResult(s.options.Ergo.Account(ctx, in.Name))
	})
}
