package api

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/admin"
	"ircord/backend/internal/storage"
)

type DatabaseStatus struct {
	StartedAt time.Time `json:"startedAt" doc:"Backend process start; cumulative pool counters reset when the process restarts."`
	storage.DatabaseDiagnostics
}

func (s *Server) databaseStatus(ctx context.Context) DatabaseStatus {
	out := DatabaseStatus{StartedAt: s.startedAt, DatabaseDiagnostics: storage.DatabaseDiagnostics{
		SampledAt: time.Now().UTC(), Engine: "sqlite",
	}}
	if s.options.Store != nil {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		out.DatabaseDiagnostics = s.options.Store.DatabaseDiagnostics(ctx)
	}
	return out
}

func (s *Server) registerDatabaseAdministration() {
	huma.Register(s.API, huma.Operation{
		OperationID: "administration-database", Method: http.MethodGet,
		Path: administrationPrefix + "/database", Summary: "Read database health and connection-pool counters",
	}, func(ctx context.Context, _ *struct{}) (*Output[DatabaseStatus], error) {
		if _, err := s.requireOwner(ctx); err != nil {
			return nil, err
		}
		return result(s.databaseStatus(ctx), nil)
	})
	huma.Register(s.API, huma.Operation{
		OperationID: "admin-database", Method: http.MethodGet,
		Path: admin.Prefix + "/database", Summary: "Read database health and connection-pool counters",
		Security: []map[string][]string{{"adminBearer": {}}},
	}, func(ctx context.Context, _ *struct{}) (*Output[DatabaseStatus], error) {
		return result(s.databaseStatus(ctx), nil)
	})
}
