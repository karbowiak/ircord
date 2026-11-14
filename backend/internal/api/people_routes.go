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
	huma.Register(s.API, huma.Operation{OperationID: "whois", Method: http.MethodGet, Path: "/api/v1/networks/{id}/whois", Summary: "Look someone up with WHOIS", Description: "Runs one WHOIS on the network for a profile: user and host, real name, server, channels, idle and sign-on time, account, TLS, operator status and away message. The person is not notified. Waits up to 10 seconds."}, func(ctx context.Context, in *struct {
		ID   string `path:"id" maxLength:"100"`
		Nick string `query:"nick" required:"true" minLength:"1" maxLength:"64"`
	}) (*Output[model.WhoisDetails], error) {
		return result(s.Manager.Whois(ctx, owner(ctx), in.ID, in.Nick))
	})
	huma.Register(s.API, huma.Operation{OperationID: "ctcp", Method: http.MethodPost, Path: "/api/v1/networks/{id}/ctcp", Summary: "Ask someone's client with CTCP", Description: "Sends one CTCP VERSION, TIME or PING to a nickname and waits up to 10 seconds for the reply. The other person's client may show the request. No reply in time returns answered false."}, func(ctx context.Context, in *struct {
		ID   string `path:"id" maxLength:"100"`
		Body struct {
			Nick    string `json:"nick" minLength:"1" maxLength:"64"`
			Command string `json:"command" enum:"VERSION,TIME,PING"`
		}
	}) (*Output[model.CTCPReply], error) {
		return result(s.Manager.CTCP(ctx, owner(ctx), in.ID, in.Body.Nick, in.Body.Command))
	})
}
