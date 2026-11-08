package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/activity"
)

// notificationAlertEvent asks this connection's device to alert: play the sound and, where
// the device shows page notifications, show one. The server sends it to one connection per
// device it routes the alert to, never while any of the account's clients is active.
type notificationAlertEvent struct {
	V    int            `json:"v" enum:"1"`
	Type string         `json:"type" enum:"notification.alert"`
	Data activity.Alert `json:"data"`
}

// ActivityReport is one tab's activity state. Clients send it when the tab becomes active or
// inactive and renew it while active; it carries no input details.
type ActivityReport struct {
	Tab        string `json:"tab" minLength:"1" maxLength:"64" doc:"Random per-tab ID, the same one sent in session.resume."`
	Active     bool   `json:"active" doc:"The tab is visible and focused and its user is present. Renew about every 25 seconds; a report lapses after 60."`
	FormFactor string `json:"formFactor,omitempty" enum:"desktop,phone,tablet" doc:"The kind of device, shared with other IRCord users as a phone or tablet indicator."`
}

func (s *Server) registerActivity() {
	huma.Register(s.API, huma.Operation{OperationID: "report-activity", Method: "POST", Path: "/api/v1/activity", DefaultStatus: 204, Summary: "Report whether this tab is in use", Description: "While any of an account's tabs is active, no device is interrupted by alerts. When none is, the most recently active device is alerted after the account's lastDeviceDelay and every device after allDevicesDelay."}, func(ctx context.Context, in *struct{ Body ActivityReport }) (*struct{}, error) {
		if s.options.Activity == nil {
			return nil, nil
		}
		s.options.Activity.Report(owner(ctx), principal(ctx).Session.ID, in.Body.Tab, in.Body.Active, in.Body.FormFactor)
		return nil, nil
	})
}
