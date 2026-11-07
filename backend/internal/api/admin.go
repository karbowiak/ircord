package api

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/admin"
)

func (s *Server) registerAdmin() {
	huma.Register(s.API, huma.Operation{
		OperationID: "admin-status", Method: http.MethodGet, Path: admin.Prefix + "/status",
		Summary: "Inspect backend status", Description: "Operator-only status. A degraded backend returns status=degraded and database=unavailable. No user data, connection configuration, or credentials are returned.",
		Security: []map[string][]string{{"adminBearer": {}}},
	}, func(ctx context.Context, _ *struct{}) (*Output[admin.Status], error) {
		status := admin.Status{Status: "ok", Database: "available", StartedAt: s.startedAt,
			UptimeSeconds: max(0, int64(time.Since(s.startedAt).Seconds())), Info: s.options.Build}
		check, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if s.options.Store == nil || s.options.Store.DB.PingContext(check) != nil {
			status.Status, status.Database = "degraded", "unavailable"
		}
		return result(status, nil)
	})
}
