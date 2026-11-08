package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/irc"
)

type SavedListInput struct {
	Cursor string `query:"cursor" maxLength:"512"`
	Q      string `query:"q" maxLength:"200" doc:"Keep only saved messages whose text contains this, ignoring ASCII letter case"`
	Limit  int    `query:"limit" default:"30" minimum:"1" maximum:"100"`
}
type SavedPath struct {
	MessageID string `path:"messageId" maxLength:"200" doc:"Canonical IRCord message ID (Message.id), not an upstream msgid"`
}
type SaveInput struct {
	MessageID string `path:"messageId" maxLength:"200" doc:"Canonical IRCord message ID (Message.id), not an upstream msgid"`
	Body      struct {
		ConversationID string `json:"conversationId" minLength:"1" maxLength:"200"`
	}
}

// Saved messages are private bookmarks: every route is scoped to the signed-in
// account, and changes reach only that account's own event stream.
func (s *Server) registerSaved() {
	huma.Register(s.API, huma.Operation{OperationID: "list-saved-messages", Method: "GET", Path: "/api/v1/saved-messages", Summary: "List your saved messages", Description: "Newest first. Each item is resolved against your current history: deleted messages keep only their author and time, and messages of conversations you left are unavailable."}, func(ctx context.Context, in *SavedListInput) (*Output[irc.SavedPage], error) {
		return result(s.Manager.SavedMessages(owner(ctx), in.Cursor, in.Q, in.Limit))
	})
	huma.Register(s.API, huma.Operation{OperationID: "list-saved-message-ids", Method: "GET", Path: "/api/v1/saved-message-ids", Summary: "List which messages you saved", Description: "References only, bounded by the per-account limit, so a client can show the saved state on any message."}, func(ctx context.Context, _ *struct{}) (*Output[Items[irc.SavedMessage]], error) {
		refs, err := s.Manager.SavedRefs(owner(ctx))
		return result(Items[irc.SavedMessage]{refs}, err)
	})
	huma.Register(s.API, huma.Operation{OperationID: "save-message", Method: "PUT", Path: "/api/v1/saved-messages/{messageId}", Summary: "Save a message for later", Description: "Idempotent: saving a message you already saved returns the existing bookmark. The message must be a stored message of the conversation, named by its canonical ID. Nobody else is notified and the message is unchanged."}, func(ctx context.Context, in *SaveInput) (*Output[irc.SavedItem], error) {
		return result(s.Manager.SaveMessage(owner(ctx), in.Body.ConversationID, in.MessageID))
	})
	huma.Register(s.API, huma.Operation{OperationID: "unsave-message", Method: "DELETE", Path: "/api/v1/saved-messages/{messageId}", DefaultStatus: 204, Summary: "Remove a saved message", Description: "Removes only your bookmark, by canonical message ID. Removing one that is not saved succeeds."}, func(ctx context.Context, in *SavedPath) (*struct{}, error) {
		if err := s.Manager.UnsaveMessage(owner(ctx), in.MessageID); err != nil {
			_, mapped := result(struct{}{}, err)
			return nil, mapped
		}
		return nil, nil
	})
}
