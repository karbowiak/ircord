package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

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

func principal(ctx context.Context) auth.Principal {
	p, _ := ctx.Value(principalKey{}).(auth.Principal)
	return p
}
func httpRequest(ctx context.Context) httpState {
	v, _ := ctx.Value(requestKey{}).(httpState)
	return v
}
func authError(err error) error {
	var invalid auth.InvalidError
	switch {
	case errors.Is(err, auth.ErrUnauthorized), errors.Is(err, auth.ErrSession):
		return huma.Error401Unauthorized(capitalize(err.Error()))
	case errors.Is(err, auth.ErrLimited):
		return huma.Error429TooManyRequests(capitalize(err.Error()))
	case errors.Is(err, auth.ErrRegistration), errors.Is(err, auth.ErrRegistrationKey), errors.Is(err, auth.ErrWrongPassword):
		return huma.Error403Forbidden(capitalize(err.Error()))
	case errors.Is(err, auth.ErrUsernameTaken):
		return huma.Error409Conflict(capitalize(err.Error()))
	case errors.As(err, &invalid):
		return huma.Error422UnprocessableEntity(invalid.Reason)
	case errors.Is(err, auth.ErrNetworkUnavailable):
		return huma.Error503ServiceUnavailable(capitalize(err.Error()))
	}
	return huma.Error503ServiceUnavailable("Account operation could not be completed.")
}

// capitalize turns an error's text into a sentence for the user.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:] + "."
}
func (s *Server) registerAuth() {
	if s.options.Auth == nil {
		return
	}
	huma.Get(s.API, "/api/v1/auth/registration", func(ctx context.Context, _ *struct{}) (*Output[auth.RegistrationPolicy], error) {
		return &Output[auth.RegistrationPolicy]{Body: s.options.Auth.RegistrationPolicy()}, nil
	})
	huma.Register(s.API, huma.Operation{OperationID: "signup", Method: "POST", Path: "/api/v1/auth/signup", DefaultStatus: 201, MaxBodyBytes: 4096}, func(ctx context.Context, in *struct{ Body auth.SignupCredentials }) (*Output[auth.LoginResult], error) {
		r := httpRequest(ctx)
		v, err := s.options.Auth.Signup(r.Writer, r.Request, in.Body)
		if err != nil {
			return nil, authError(err)
		}
		return &Output[auth.LoginResult]{Body: v}, nil
	})
	huma.Register(s.API, huma.Operation{OperationID: "login", Method: "POST", Path: "/api/v1/auth/login", MaxBodyBytes: 4096}, func(ctx context.Context, in *struct{ Body auth.Credentials }) (*Output[auth.LoginResult], error) {
		r := httpRequest(ctx)
		v, err := s.options.Auth.Login(r.Writer, r.Request, in.Body)
		if err != nil {
			return nil, authError(err)
		}
		return &Output[auth.LoginResult]{Body: v}, nil
	})
	huma.Get(s.API, "/api/v1/auth/username", func(ctx context.Context, in *struct {
		Name string `query:"name" maxLength:"64"`
	}) (*Output[auth.UsernameCheck], error) {
		v, err := s.options.Auth.CheckUsername(ctx, in.Name)
		if err != nil {
			return nil, authError(err)
		}
		return &Output[auth.UsernameCheck]{Body: v}, nil
	})
	huma.Register(s.API, huma.Operation{OperationID: "change-password", Method: "POST", Path: "/api/v1/auth/password", DefaultStatus: 204, MaxBodyBytes: 4096}, func(ctx context.Context, in *struct{ Body auth.PasswordChange }) (*struct{}, error) {
		if err := s.options.Auth.ChangePassword(ctx, principal(ctx), in.Body); err != nil {
			return nil, authError(err)
		}
		return nil, nil
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
	huma.Register(s.API, huma.Operation{OperationID: "delete-account", Method: "DELETE", Path: "/api/v1/auth/account", DefaultStatus: 204}, func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		if err := s.options.Auth.Delete(owner(ctx)); err != nil {
			return nil, authError(err)
		}
		http.SetCookie(httpRequest(ctx).Writer, &http.Cookie{Name: "ircord_session", Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
		return nil, nil
	})
}
