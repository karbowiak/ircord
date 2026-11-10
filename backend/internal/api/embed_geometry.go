package api

import (
	"context"
	"encoding/json"

	"ircord/backend/internal/irc"
	"ircord/backend/internal/linkmedia"
	"ircord/backend/internal/model"
)

// Call only with messages already returned by an account-authorized manager
// read or private subscription. Clone the slice and replace hints, never state.
func (s *Server) messagesWithGeometry(ctx context.Context, messages []model.Message) []model.Message {
	if messages == nil {
		return nil
	}
	out := append([]model.Message{}, messages...)
	messageURLs := make([][]string, len(out))
	urls := make([]string, 0)
	seen := map[string]bool{}
	for i := range out {
		out[i].EmbedGeometry = nil
		if out[i].Deleted || out[i].HasAttachments() {
			continue
		}
		messageURLs[i] = messageGeometryURLs(out[i])
		for _, raw := range messageURLs[i] {
			if !seen[raw] && len(urls) < linkmedia.MaxGeometryURLs {
				seen[raw] = true
				urls = append(urls, raw)
			}
		}
	}
	geometry := s.options.Media.CachedImageGeometry(ctx, urls)
	for i := range out {
		if out[i].Deleted || out[i].HasAttachments() {
			continue
		}
		for _, raw := range messageURLs[i] {
			if size, ok := geometry[raw]; ok {
				out[i].EmbedGeometry = append(out[i].EmbedGeometry, model.EmbedGeometry{URL: raw, Width: size.Width, Height: size.Height})
			}
		}
	}
	return out
}

func messageGeometryURLs(message model.Message) []string {
	urls := linkmedia.URLs(textOutsideLinkCode(message.Text))
	if len(urls) > model.MaxEmbedGeometry {
		urls = urls[:model.MaxEmbedGeometry]
	}
	return urls
}

func (s *Server) messageWithGeometry(ctx context.Context, message model.Message) model.Message {
	return s.messagesWithGeometry(ctx, []model.Message{message})[0]
}

func (s *Server) messageResult(ctx context.Context, message model.Message, err error) (*Output[model.Message], error) {
	if err == nil {
		message = s.messageWithGeometry(ctx, message)
	}
	return result(message, err)
}

func (s *Server) historyResult(ctx context.Context, page model.MessagePage, err error) (*Output[model.MessagePage], error) {
	if err == nil {
		page.Items = s.messagesWithGeometry(ctx, page.Items)
	}
	return result(page, err)
}

func (s *Server) savedWithGeometry(ctx context.Context, items []irc.SavedItem) []irc.SavedItem {
	out := append([]irc.SavedItem{}, items...)
	var messages []model.Message
	var indices []int
	for i, item := range out {
		if item.Message != nil {
			message := *item.Message
			message.EmbedGeometry = nil
			out[i].Message = &message
			if item.Status == "available" {
				messages = append(messages, message)
				indices = append(indices, i)
			}
		}
	}
	for i, message := range s.messagesWithGeometry(ctx, messages) {
		out[indices[i]].Message = &message
	}
	return out
}

// Only the two typed message events are decorated. The durable envelope,
// sequence, order, and unrelated event payloads remain unchanged.
func (s *Server) eventsWithGeometry(ctx context.Context, events []irc.Event) []irc.Event {
	if events == nil {
		return nil
	}
	out := append([]irc.Event{}, events...)
	var messages []model.Message
	var indices []int
	for i, event := range out {
		if event.Type != "message.created" && event.Type != "message.updated" {
			continue
		}
		var message model.Message
		if json.Unmarshal(event.Data, &message) == nil {
			messages = append(messages, message)
			indices = append(indices, i)
		}
	}
	for i, message := range s.messagesWithGeometry(ctx, messages) {
		if data, err := json.Marshal(message); err == nil {
			out[indices[i]].Data = data
		}
	}
	return out
}
