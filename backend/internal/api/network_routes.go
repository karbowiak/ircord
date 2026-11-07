package api

import (
	"context"
	"encoding/json"
	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/irc"
)

type SelfUpdate struct {
	Nick     *string `json:"nick,omitempty" nullable:"false"`
	Realname *string `json:"realname,omitempty" nullable:"false"`
	Away     *string `json:"away,omitempty" nullable:"true"`
}

func (s *Server) registerNetworkAPI() {
	huma.Get(s.API, "/api/v1/conversations/{id}/lists/{mode}", func(ctx context.Context, in *struct {
		ID   string `path:"id"`
		Mode string `path:"mode"`
	}) (*Output[Items[irc.ChannelListEntry]], error) {
		items, err := s.Manager.ModeList(ctx, owner(ctx), in.ID, in.Mode)
		return result(Items[irc.ChannelListEntry]{items}, err)
	})
	huma.Delete(s.API, "/api/v1/networks/{id}", func(ctx context.Context, in *struct {
		ID string `path:"id"`
	}) (*struct{}, error) {
		if err := s.Manager.DeleteNetwork(owner(ctx), in.ID); err != nil {
			_, e := result(false, err)
			return nil, e
		}
		return nil, nil
	})
	huma.Register(s.API, huma.Operation{OperationID: "send-raw", Method: "POST", Path: "/api/v1/networks/{id}/commands", DefaultStatus: 202}, func(ctx context.Context, in *struct {
		ID   string `path:"id"`
		Body struct {
			ClientID string `json:"clientId" maxLength:"100"`
			Line     string `json:"line" maxLength:"510"`
		}
	}) (*Output[struct {
		ClientID string `json:"clientId"`
	}], error) {
		return result(struct {
			ClientID string `json:"clientId"`
		}{in.Body.ClientID}, s.Manager.RawCommand(owner(ctx), in.ID, in.Body.ClientID, in.Body.Line))
	})
	huma.Get(s.API, "/api/v1/networks/{id}/channels", func(ctx context.Context, in *struct {
		ID    string `path:"id"`
		Q     string `query:"q" maxLength:"200"`
		Limit int    `query:"limit" default:"50" minimum:"1" maximum:"100"`
	}) (*Output[Items[irc.ChannelListing]], error) {
		items, err := s.Manager.Channels(ctx, owner(ctx), in.ID, in.Q, in.Limit)
		return result(Items[irc.ChannelListing]{items}, err)
	})
	huma.Patch(s.API, "/api/v1/networks/{id}/self", func(ctx context.Context, in *struct {
		ID      string `path:"id"`
		Body    SelfUpdate
		RawBody []byte
	}) (*Output[irc.Network], error) {
		var fields map[string]*string
		_ = json.Unmarshal(in.RawBody, &fields)
		for key, value := range fields {
			var err error
			switch key {
			case "nick":
				if value == nil {
					return result(irc.Network{}, huma.Error400BadRequest("nick cannot be null"))
				}
				err = s.Manager.Nick(owner(ctx), in.ID, *value)
			case "away":
				reason := ""
				if value != nil {
					reason = *value
				}
				err = s.Manager.SetAway(owner(ctx), in.ID, reason)
			case "realname":
				if value == nil {
					return result(irc.Network{}, huma.Error400BadRequest("realname cannot be null"))
				}
				err = s.Manager.Realname(owner(ctx), in.ID, *value)
			default:
				return result(irc.Network{}, huma.Error400BadRequest("unknown profile field"))
			}
			if err != nil {
				return result(irc.Network{}, err)
			}
		}
		for _, n := range s.Manager.Snapshot(owner(ctx)).Networks {
			if n.ID == in.ID {
				return result(n, nil)
			}
		}
		return result(irc.Network{}, irc.ErrNotFound)
	})
	huma.Get(s.API, "/api/v1/events/replay", func(ctx context.Context, in *struct {
		After uint64 `query:"after"`
	}) (*Output[struct {
		Items []irc.Event `json:"items"`
		Valid bool        `json:"valid"`
	}], error) {
		replay, _, valid, stop := s.Manager.Subscribe(owner(ctx), in.After)
		defer stop()
		return result(struct {
			Items []irc.Event `json:"items"`
			Valid bool        `json:"valid"`
		}{replay, valid}, s.Manager.StorageError(owner(ctx)))
	})
}
