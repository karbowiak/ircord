package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/irc"
)

type GalleryInput struct {
	ID     string `path:"id" maxLength:"200"`
	Cursor string `query:"cursor" maxLength:"512"`
	Limit  int    `query:"limit" default:"40" minimum:"1" maximum:"100"`
}

// A gallery reads only the signed-in account's own stored history of a
// conversation it has open; the images themselves are never fetched or stored.
func (s *Server) registerGallery() {
	huma.Register(s.API, huma.Operation{OperationID: "list-conversation-gallery", Method: "GET", Path: "/api/v1/conversations/{id}/gallery", Summary: "List the images posted in a conversation", Description: "Images and GIFs from uploads, direct image links, the GIF picker and server link previews, newest post first and in message order within a post. Each item links to its source message. The list is derived from your stored history and follows edits and deletions; a deleted message's images leave the gallery."}, func(ctx context.Context, in *GalleryInput) (*Output[irc.GalleryPage], error) {
		return result(s.Manager.Gallery(owner(ctx), in.ID, in.Cursor, in.Limit))
	})
}
