package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/model"
)

func (s *Server) registerPeople() {
	huma.Register(s.API, huma.Operation{OperationID: "people-lookup", Method: http.MethodGet, Path: "/api/v1/people", Summary: "Find people for a network and purpose", Description: "Returns bounded, purpose-eligible people from the signed-in workspace. Services and party lookup can include names from the associated server's registered account directory. Known people are not a server-wide online roster. Nicknames, services accounts, and local account IDs are distinct."}, func(ctx context.Context, in *model.PeopleQuery) (*Output[model.PeoplePage], error) {
		return result(s.Manager.LookupPeople(ctx, owner(ctx), *in))
	})
}
