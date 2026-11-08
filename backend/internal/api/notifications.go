package api

import (
	"context"
	"encoding/json"
	"ircord/backend/internal/push"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/notify"
)

type NotificationSettings = notify.Settings
type BrowserSubscription struct {
	push.Subscription
	ExpirationTime *float64 `json:"expirationTime,omitempty" nullable:"true"`
}

func (s *Server) notificationSettings(account string) (NotificationSettings, error) {
	raw, err := s.Manager.AppData(account, "notification-settings.updated")
	return notify.Compile(raw).Settings, err
}
func (s *Server) registerNotifications() {
	huma.Get(s.API, "/api/v1/notifications/settings", func(ctx context.Context, _ *struct{}) (*Output[NotificationSettings], error) {
		return result(s.notificationSettings(owner(ctx)))
	})
	huma.Put(s.API, "/api/v1/notifications/settings", func(ctx context.Context, in *struct{ Body NotificationSettings }) (*Output[NotificationSettings], error) {
		in.Body = notify.New(in.Body).Settings
		// Per-conversation levels are personal for direct messages only; a channel's level is
		// a channel setting, so entries for anything but a direct message are dropped.
		direct := map[string]bool{}
		for _, c := range s.Manager.Snapshot(owner(ctx)).Conversations {
			direct[c.ID] = c.Kind == "direct"
		}
		levels := map[string]string{}
		for id, v := range in.Body.Levels {
			if direct[id] {
				levels[id] = v
			}
		}
		in.Body.Levels = levels
		if in.Body.HighlightWords == nil {
			in.Body.HighlightWords = []string{}
		}
		if in.Body.Ignore == nil {
			in.Body.Ignore = []string{}
		}
		raw, _ := json.Marshal(in.Body)
		_, err := s.Manager.SaveAppData(owner(ctx), "notification-settings.updated", raw)
		return result(in.Body, err)
	})

	huma.Get(s.API, "/api/v1/push/key", func(ctx context.Context, _ *struct{}) (*Output[PushKey], error) {
		key := ""
		if s.options.Push != nil {
			key = s.options.Push.PublicKey()
		}
		return result(PushKey{key, key != ""}, nil)
	})
	huma.Register(s.API, huma.Operation{OperationID: "save-push-subscription", Method: "POST", Path: "/api/v1/push/subscriptions", DefaultStatus: 204}, func(ctx context.Context, in *struct {
		Body struct {
			Subscription BrowserSubscription `json:"subscription"`
			Device       string              `json:"device,omitempty" maxLength:"120"`
		}
	}) (*struct{}, error) {
		if s.options.Push == nil {
			return nil, huma.Error501NotImplemented("Web Push is not configured on this server.")
		}
		if err := s.options.Push.Add(ctx, owner(ctx), principal(ctx).Session.ID, in.Body.Device, in.Body.Subscription.Subscription); err != nil {
			_, e := result(false, err)
			return nil, e
		}
		s.devicesChanged(owner(ctx))
		return nil, nil
	})
	huma.Delete(s.API, "/api/v1/push/subscriptions", func(ctx context.Context, in *struct {
		Body struct {
			Endpoint string `json:"endpoint" maxLength:"4096"`
		}
	}) (*struct{}, error) {
		if s.options.Push != nil {
			if err := s.options.Push.Remove(owner(ctx), in.Body.Endpoint); err != nil {
				_, e := result(false, err)
				return nil, e
			}
		}
		s.devicesChanged(owner(ctx))
		return nil, nil
	})
	huma.Register(s.API, huma.Operation{OperationID: "test-push", Method: "POST", Path: "/api/v1/push/test", DefaultStatus: 204}, func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		if s.options.Push == nil {
			return nil, huma.Error501NotImplemented("Web Push is not configured on this server.")
		}
		limited, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := s.options.Push.Send(limited, owner(ctx), push.Payload{Title: "IRCord", Body: "Notifications are working on this device.", Tag: "test"}); err != nil {
			return nil, huma.Error503ServiceUnavailable("Test notification could not be delivered.")
		}
		return nil, nil
	})
}

// devicesChanged refreshes open Devices & Sessions lists after a push registration change.
func (s *Server) devicesChanged(account string) {
	if s.options.Auth != nil {
		s.options.Auth.DevicesChanged(account)
	}
}
