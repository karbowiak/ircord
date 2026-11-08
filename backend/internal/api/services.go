package api

import (
	"context"
	"errors"
	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/irc"
)

type AccessPath struct {
	ID      string `path:"id"`
	Account string `path:"account" minLength:"1" maxLength:"64"`
}
type AccessChange struct {
	ID      string `path:"id"`
	Account string `path:"account" minLength:"1" maxLength:"64"`
	Body    struct {
		Mode string `json:"mode" enum:"q,a,o,h,v"`
	}
}

func serviceResult[T any](value T, err error) (*Output[T], error) {
	var failure *irc.ServiceError
	if errors.As(err, &failure) {
		return nil, huma.NewError(failure.Status, failure.Detail)
	}
	return result(value, err)
}
func (s *Server) registerServices() {
	huma.Register(s.API, huma.Operation{OperationID: "get-channel-registration", Method: "GET", Path: "/api/v1/conversations/{id}/registration"}, func(ctx context.Context, in *ConversationPath) (*Output[irc.ChannelRegistration], error) {
		return serviceResult(s.Manager.ChannelRegistration(ctx, owner(ctx), in.ID))
	})
	huma.Register(s.API, huma.Operation{OperationID: "register-channel", Method: "POST", Path: "/api/v1/conversations/{id}/registration"}, func(ctx context.Context, in *ConversationPath) (*Output[irc.ChannelRegistration], error) {
		return serviceResult(s.Manager.RegisterChannel(ctx, owner(ctx), in.ID))
	})
	huma.Register(s.API, huma.Operation{OperationID: "unregister-channel", Method: "DELETE", Path: "/api/v1/conversations/{id}/registration", DefaultStatus: 204}, func(ctx context.Context, in *ConversationPath) (*struct{}, error) {
		_, err := serviceResult(false, s.Manager.UnregisterChannel(ctx, owner(ctx), in.ID))
		return nil, err
	})
	huma.Register(s.API, huma.Operation{OperationID: "get-channel-access", Method: "GET", Path: "/api/v1/conversations/{id}/access"}, func(ctx context.Context, in *ConversationPath) (*Output[Items[irc.AccessEntry]], error) {
		items, err := s.Manager.ChannelAccess(ctx, owner(ctx), in.ID)
		return serviceResult(Items[irc.AccessEntry]{items}, err)
	})
	huma.Register(s.API, huma.Operation{OperationID: "set-channel-access", Method: "PUT", Path: "/api/v1/conversations/{id}/access/{account}"}, func(ctx context.Context, in *AccessChange) (*Output[Items[irc.AccessEntry]], error) {
		items, err := s.Manager.SetChannelAccess(ctx, owner(ctx), in.ID, in.Account, in.Body.Mode, false)
		return serviceResult(Items[irc.AccessEntry]{items}, err)
	})
	huma.Register(s.API, huma.Operation{OperationID: "remove-channel-access", Method: "DELETE", Path: "/api/v1/conversations/{id}/access/{account}"}, func(ctx context.Context, in *AccessPath) (*Output[Items[irc.AccessEntry]], error) {
		items, err := s.Manager.SetChannelAccess(ctx, owner(ctx), in.ID, in.Account, "", true)
		return serviceResult(Items[irc.AccessEntry]{items}, err)
	})
	huma.Register(s.API, huma.Operation{OperationID: "transfer-channel", Method: "POST", Path: "/api/v1/conversations/{id}/transfer"}, func(ctx context.Context, in *struct {
		ID   string `path:"id"`
		Body struct {
			Account string `json:"account" minLength:"1" maxLength:"64"`
		}
	}) (*Output[irc.TransferResult], error) {
		return serviceResult(s.Manager.TransferChannel(ctx, owner(ctx), in.ID, in.Body.Account))
	})
}
