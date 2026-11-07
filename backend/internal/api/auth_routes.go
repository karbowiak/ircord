package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/auth"
	"ircord/backend/internal/storage"
)

type requestKey struct{}
type principalKey struct{}
type httpState struct {
	Writer  http.ResponseWriter
	Request *http.Request
}
type RegisteredAccount struct {
	Key     string          `json:"key"`
	Account storage.Account `json:"account"`
}

func principal(ctx context.Context) auth.Principal {
	p, _ := ctx.Value(principalKey{}).(auth.Principal)
	return p
}
func httpRequest(ctx context.Context) httpState {
	v, _ := ctx.Value(requestKey{}).(httpState)
	return v
}
func authError(err error) error {
	if errors.Is(err, auth.ErrUnauthorized) {
		return huma.Error401Unauthorized(err.Error())
	}
	if errors.Is(err, auth.ErrLimited) {
		return huma.Error429TooManyRequests(err.Error())
	}
	if errors.Is(err, auth.ErrRegistration) {
		return huma.Error403Forbidden(err.Error())
	}
	return huma.Error503ServiceUnavailable("Account operation could not be completed.")
}
func (s *Server) registerAuth() {
	if s.options.Auth == nil {
		return
	}
	huma.Register(s.API, huma.Operation{OperationID: "register-account", Method: "POST", Path: "/api/v1/auth/register", DefaultStatus: 201}, func(ctx context.Context, _ *struct{}) (*Output[RegisteredAccount], error) {
		r := httpRequest(ctx)
		v, err := s.options.Auth.Login(r.Writer, r.Request, auth.Credentials{}, true)
		if err != nil {
			return nil, authError(err)
		}
		return &Output[RegisteredAccount]{Body: RegisteredAccount{Key: v.Key, Account: v.Account}}, nil
	})
	huma.Post(s.API, "/api/v1/auth/login", func(ctx context.Context, in *struct{ Body auth.Credentials }) (*Output[auth.LoginResult], error) {
		r := httpRequest(ctx)
		v, err := s.options.Auth.Login(r.Writer, r.Request, in.Body, false)
		if err != nil {
			return nil, authError(err)
		}
		return &Output[auth.LoginResult]{Body: v}, nil
	})
	huma.Get(s.API, "/api/v1/auth/session", func(ctx context.Context, _ *struct{}) (*Output[AccountSession], error) {
		return &Output[AccountSession]{Body: AccountSession{func() *storage.Account {
			p := principal(ctx)
			if p.Account.ID == "" {
				return nil
			}
			return &p.Account
		}()}}, nil
	})
	huma.Register(s.API, huma.Operation{OperationID: "logout", Method: "POST", Path: "/api/v1/auth/logout", DefaultStatus: 204}, func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		if err := s.options.Auth.Logout(httpRequest(ctx).Writer, principal(ctx)); err != nil {
			return nil, authError(err)
		}
		return nil, nil
	})
	huma.Post(s.API, "/api/v1/auth/rotate", func(ctx context.Context, _ *struct{}) (*Output[AccountKey], error) {
		key, err := s.options.Auth.Rotate(principal(ctx))
		if err != nil {
			return nil, authError(err)
		}
		return &Output[AccountKey]{Body: AccountKey{key}}, nil
	})
	huma.Register(s.API, huma.Operation{OperationID: "delete-account", Method: "DELETE", Path: "/api/v1/auth/account", DefaultStatus: 204}, func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		if err := s.options.Auth.Delete(owner(ctx)); err != nil {
			return nil, authError(err)
		}
		http.SetCookie(httpRequest(ctx).Writer, &http.Cookie{Name: "ircord_session", Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
		return nil, nil
	})
}
