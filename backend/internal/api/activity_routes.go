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
type ActivityReport = activity.Report

type notificationRoutingEvent struct {
	V    int                      `json:"v" enum:"1"`
	Type string                   `json:"type" enum:"notification.routing"`
	Data activity.RoutingSnapshot `json:"data"`
}

func (s *Server) registerActivity() {
	huma.Get(s.API, "/api/v1/activity", func(ctx context.Context, _ *struct{}) (*Output[activity.RoutingSnapshot], error) {
		if s.options.Activity == nil {
			return &Output[activity.RoutingSnapshot]{}, nil
		}
		return &Output[activity.RoutingSnapshot]{Body: s.options.Activity.Snapshot(owner(ctx), principal(ctx).Session.ID)}, nil
	})
	huma.Register(s.API, huma.Operation{OperationID: "report-activity", Method: "POST", Path: "/api/v1/activity", DefaultStatus: 204, Summary: "Report this tab's engagement or renew its focused reading lease"}, func(ctx context.Context, in *struct{ Body ActivityReport }) (*struct{}, error) {
		if s.options.Activity == nil {
			return nil, nil
		}
		if !s.options.Activity.ReportState(owner(ctx), principal(ctx).Session.ID, in.Body) {
			return nil, huma.Error409Conflict("This activity report is stale. Wait for the current connection snapshot.")
		}
		return nil, nil
	})
}
