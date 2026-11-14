package api

import (
	"context"
	"errors"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/irc"
)

type SearchInput struct {
	Q              string    `query:"q" maxLength:"200" doc:"Text to search. Retained search uses a phrase with a prefix on the last word; upstream matching depends on the server. Optional when a filter is set."`
	From           []string  `query:"from,explode" maxItems:"50"`
	Mentions       []string  `query:"mentions,explode" maxItems:"50" doc:"Current member nicks, matched as whole words. Unknown member IDs match nothing."`
	ConversationID []string  `query:"conversationId,explode" maxItems:"50" doc:"Any inaccessible conversation rejects the request with 404."`
	Has            []string  `query:"has,explode" maxItems:"50" enum:"link,image,gif,video,file"`
	Before         time.Time `query:"before" doc:"Exclusive upper date bound."`
	After          time.Time `query:"after" doc:"Inclusive lower date bound."`
	Author         string    `query:"author" enum:"user,bot"`
	Cursor         string    `query:"cursor" maxLength:"512"`
	Limit          int       `query:"limit" default:"50" minimum:"1" maximum:"100"`
	Source         string    `query:"source" enum:"retained,upstream" doc:"Search retained messages or request a bounded upstream search."`
}

func (s *Server) registerSearch() {
	huma.Get(s.API, "/api/v1/search", func(ctx context.Context, in *SearchInput) (*Output[irc.SearchPage], error) {
		page, err := s.Manager.SearchMessagesContext(ctx, owner(ctx), irc.SearchQuery{Query: in.Q, From: in.From, Mentions: in.Mentions, Conversations: in.ConversationID, Has: in.Has, Before: in.Before, After: in.After, Author: in.Author, Cursor: in.Cursor, Limit: in.Limit, Source: in.Source})
		var invalid *irc.SearchValidationError
		if errors.As(err, &invalid) {
			return nil, huma.Error422UnprocessableEntity(invalid.Detail)
		}
		if err == nil {
			page.Items = append([]irc.SearchResult{}, page.Items...)
			messages := make([]irc.Message, len(page.Items))
			for i := range page.Items {
				messages[i] = page.Items[i].Message
			}
			for i, message := range s.messagesWithGeometry(ctx, messages) {
				page.Items[i].Message = message
			}
		}
		return result(page, err)
	})
}
