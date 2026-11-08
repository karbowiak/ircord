package api

import (
	"context"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/irc"
)

type LinkListInput struct {
	Conversation []string  `query:"conversation,explode" maxItems:"50" doc:"Keep links posted in these channels (conversation IDs; repeat the parameter for several)"`
	Network      string    `query:"network" maxLength:"200" doc:"Keep links posted on this network"`
	Q            string    `query:"q" maxLength:"200" doc:"Keep links whose URL, message text or preview title contains this, ignoring ASCII letter case"`
	Host         string    `query:"host" maxLength:"253" doc:"Keep links to this host or its subdomains, such as example.com"`
	Kind         []string  `query:"kind,explode" maxItems:"6" enum:"link,image,gif,video,audio,youtube" doc:"Keep links of these kinds (repeat the parameter for several)"`
	Since        time.Time `query:"since" doc:"Keep links posted at or after this time"`
	Until        time.Time `query:"until" doc:"Keep links posted before this time"`
	Cursor       string    `query:"cursor" maxLength:"4096"`
	Limit        int       `query:"limit" default:"30" minimum:"1" maximum:"50"`
}

// The link catalog is derived from each account's own stored history, so every
// card, count and source is one the account can already read in its channels.
func (s *Server) registerLinks() {
	huma.Register(s.API, huma.Operation{OperationID: "list-links", Method: "GET", Path: "/api/v1/links", Summary: "List links posted in your channels", Description: "Links from your channels' stored history, grouped by resource and ordered by their most recent post. Links in code, deleted messages, ignored senders and closed conversations are left out. Nothing is fetched to build the list; a card carries a server link preview only when its channel already resolved one."}, func(ctx context.Context, in *LinkListInput) (*Output[irc.LinkPage], error) {
		return result(s.Manager.Links(owner(ctx), irc.LinkQuery{Conversations: in.Conversation, Network: in.Network, Query: in.Q, Host: in.Host, Kinds: in.Kind, Since: in.Since, Until: in.Until, Cursor: in.Cursor, Limit: in.Limit}))
	})
}
