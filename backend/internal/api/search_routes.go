package api

import (
	"context"
	"errors"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/storage"
)

type SearchInput struct {
	Q              string    `query:"q" maxLength:"200" doc:"Text phrase with a prefix on the last word. Optional when a filter is set."`
	From           []string  `query:"from,explode" maxItems:"50"`
	Mentions       []string  `query:"mentions,explode" maxItems:"50" doc:"Current member nicks, matched as whole words. Unknown member IDs match nothing."`
	ConversationID []string  `query:"conversationId,explode" maxItems:"50" doc:"Any inaccessible conversation rejects the request with 404."`
	Has            []string  `query:"has,explode" maxItems:"50" enum:"link,image,gif,video,file"`
	Before         time.Time `query:"before" doc:"Exclusive upper date bound."`
	After          time.Time `query:"after" doc:"Inclusive lower date bound."`
	Author         string    `query:"author" enum:"user,bot"`
	Cursor         string    `query:"cursor" maxLength:"512"`
	Limit          int       `query:"limit" default:"50" minimum:"1" maximum:"100"`
}

func (s *Server) registerSearch() {
	huma.Get(s.API, "/api/v1/search", func(ctx context.Context, in *SearchInput) (*Output[irc.SearchPage], error) {
		page, err := s.Manager.SearchMessagesContext(ctx, owner(ctx), irc.SearchQuery{Query: in.Q, From: in.From, Mentions: in.Mentions, Conversations: in.ConversationID, Has: in.Has, Before: in.Before, After: in.After, Author: in.Author, Cursor: in.Cursor, Limit: in.Limit})
		if err != nil && !errors.Is(err, irc.ErrNotFound) && !errors.Is(err, irc.ErrStorage) && !errors.Is(err, storage.ErrInvalidCursor) {
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		return result(page, err)
	})
}
