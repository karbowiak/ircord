package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/irc"
)

type InboxListInput struct {
	Filter string `query:"filter" default:"all" enum:"all,unread,mentions,replies,direct,dismissed" doc:"all, mentions, replies and direct list undismissed items; unread keeps those still unread in their conversation; dismissed lists dismissed items to restore"`
	Cursor string `query:"cursor" maxLength:"512"`
	Limit  int    `query:"limit" default:"30" minimum:"1" maximum:"100"`
}
type InboxPath struct {
	MessageID string `path:"messageId" maxLength:"200" doc:"Canonical IRCord message ID (Message.id)"`
}

// The inbox is private to the signed-in account: every route reads and changes
// only that account's items, and changes reach only its own event stream.
func (s *Server) registerInbox() {
	huma.Register(s.API, huma.Operation{OperationID: "list-inbox", Method: "GET", Path: "/api/v1/inbox", Summary: "List your inbox", Description: "Mentions, replies to your messages and direct messages, newest first, one item per message. Items are resolved against your current history: deleted messages keep only their author and time, and messages of conversations you left are unavailable. Reading this list marks nothing read."}, func(ctx context.Context, in *InboxListInput) (*Output[irc.InboxPage], error) {
		return result(s.Manager.Inbox(owner(ctx), in.Filter, in.Cursor, in.Limit))
	})
	huma.Register(s.API, huma.Operation{OperationID: "inbox-summary", Method: "GET", Path: "/api/v1/inbox/summary", Summary: "Count your unread inbox items", Description: "The inbox's unread indicator: undismissed items whose message is still unread in its conversation."}, func(ctx context.Context, _ *struct{}) (*Output[irc.InboxSummary], error) {
		return result(s.Manager.InboxSummary(owner(ctx)))
	})
	huma.Register(s.API, huma.Operation{OperationID: "dismiss-inbox-item", Method: "PUT", Path: "/api/v1/inbox/{messageId}/dismissed", DefaultStatus: 204, Summary: "Dismiss an inbox item", Description: "Hides the item from the inbox on all your clients. The conversation is not marked read and no notification is sent. Idempotent."}, func(ctx context.Context, in *InboxPath) (*struct{}, error) {
		return empty(s.Manager.DismissInbox(owner(ctx), in.MessageID, true))
	})
	huma.Register(s.API, huma.Operation{OperationID: "restore-inbox-item", Method: "DELETE", Path: "/api/v1/inbox/{messageId}/dismissed", DefaultStatus: 204, Summary: "Restore a dismissed inbox item", Description: "Returns a dismissed item to the inbox on all your clients. Idempotent."}, func(ctx context.Context, in *InboxPath) (*struct{}, error) {
		return empty(s.Manager.DismissInbox(owner(ctx), in.MessageID, false))
	})
}
