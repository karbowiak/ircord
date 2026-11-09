package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/model"
)

type NetworkInput struct{ Body irc.NetworkConfig }
type NetworkPatch struct {
	ID   string `path:"id"`
	Body NetworkUpdate
}
type NetworkUpdate struct {
	Status   *string          `json:"status,omitempty" enum:"connected,disconnected" nullable:"false"`
	Name     *string          `json:"name,omitempty" nullable:"false" maxLength:"100"`
	Host     *string          `json:"host,omitempty" nullable:"false" maxLength:"253"`
	Port     *int             `json:"port,omitempty" nullable:"false" minimum:"1" maximum:"65535"`
	TLS      *bool            `json:"tls,omitempty" nullable:"false"`
	Nick     *string          `json:"nick,omitempty" nullable:"false" maxLength:"32"`
	AltNick  *string          `json:"altNick,omitempty" nullable:"false" maxLength:"32"`
	Username *string          `json:"username,omitempty" nullable:"false" maxLength:"20"`
	Realname *string          `json:"realname,omitempty" nullable:"false" maxLength:"150"`
	Auth     *model.AuthInput `json:"auth,omitempty"`
	AutoJoin *[]string        `json:"autoJoin,omitempty" nullable:"false"`
	Perform  *[]string        `json:"perform,omitempty" nullable:"false"`
}

type SelfUpdate struct {
	Nick     *string `json:"nick,omitempty" nullable:"false"`
	Realname *string `json:"realname,omitempty" nullable:"false"`
	Away     *string `json:"away,omitempty" nullable:"true"`
}

func (s *Server) registerNetworkAPI() {
	huma.Post(s.API, "/api/v1/networks/check", func(ctx context.Context, in *NetworkCheckInput) (*Output[NetworkCheck], error) {
		err := s.Manager.CheckNetwork(ctx, in.Body.Host, in.Body.Port, in.Body.TLS)
		if err != nil {
			return result(NetworkCheck{Reachable: false, Detail: err.Error()}, nil)
		}
		return result(NetworkCheck{Reachable: true, Detail: "Server address verified. Sign-in is checked when you connect."}, nil)
	})
	huma.Post(s.API, "/api/v1/networks", func(ctx context.Context, in *NetworkInput) (*Output[irc.Network], error) {
		if in.Body.Nick == "" {
			in.Body.Nick = "irc" + owner(ctx)[:min(8, len(owner(ctx)))]
		}
		return result(s.Manager.Connect(owner(ctx), in.Body))
	})
	huma.Patch(s.API, "/api/v1/networks/{id}", func(ctx context.Context, in *NetworkPatch) (*Output[irc.Network], error) {
		data, _ := json.Marshal(in.Body)
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(data, &fields)
		if raw, ok := fields["status"]; ok {
			if len(fields) != 1 {
				return result(irc.Network{}, fmt.Errorf("status cannot be combined with configuration"))
			}
			var status string
			if json.Unmarshal(raw, &status) != nil || (status != "connected" && status != "disconnected") {
				return result(irc.Network{}, fmt.Errorf("invalid connection status"))
			}
			var err error
			if status == "connected" {
				err = s.Manager.Reconnect(owner(ctx), in.ID)
			} else {
				err = s.Manager.Disconnect(owner(ctx), in.ID)
			}
			if err != nil {
				return result(irc.Network{}, err)
			}
			return result(s.Manager.Network(owner(ctx), in.ID))
		}
		return result(s.Manager.PatchNetwork(owner(ctx), in.ID, fields))
	})
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
		return empty(s.Manager.DeleteNetwork(owner(ctx), in.ID))
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
		return result(s.Manager.Network(owner(ctx), in.ID))
	})
}
