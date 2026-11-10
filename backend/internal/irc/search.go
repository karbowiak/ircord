package irc

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"ircord/backend/internal/model"
	"ircord/backend/internal/msgindex"
	"ircord/backend/internal/storage"
)

type SearchResult struct {
	Message      Message      `json:"message"`
	Conversation Conversation `json:"conversation"`
}

func (m *Manager) Search(owner, query, conversation string) ([]SearchResult, error) {
	in := SearchQuery{Query: query}
	if conversation != "" {
		in.Conversations = []string{conversation}
	}
	page, err := m.SearchMessages(owner, in)
	return page.Items, err
}

type SearchQuery struct {
	Query                              string
	From, Mentions, Conversations, Has []string
	Before, After                      time.Time
	Author, Cursor                     string
	Limit                              int
}

type SearchPage struct {
	Items      []SearchResult `json:"items" nullable:"false"`
	NextCursor *string        `json:"nextCursor"`
}

// SearchMessages searches only retained history visible to this account.
// Unknown conversations reject the entire list. Unknown members match nothing.
// Ignored senders remain searchable, as with stored timeline history.
func (m *Manager) SearchMessages(owner string, in SearchQuery) (SearchPage, error) {
	return m.SearchMessagesContext(context.Background(), owner, in)
}

func (m *Manager) SearchMessagesContext(ctx context.Context, owner string, in SearchQuery) (SearchPage, error) {
	page := SearchPage{Items: []SearchResult{}}
	if err := ctx.Err(); err != nil {
		return page, err
	}
	in.Query = strings.TrimSpace(in.Query)
	filtered := len(in.From)+len(in.Mentions)+len(in.Conversations)+len(in.Has) > 0 || !in.Before.IsZero() || !in.After.IsZero() || in.Author != ""
	if utf8.RuneCountInString(in.Query) > 200 || !filtered && utf8.RuneCountInString(in.Query) < 2 {
		return page, fmt.Errorf("enter at least two characters or choose a filter; search text can have at most 200 characters")
	}
	for _, values := range [][]string{in.From, in.Mentions, in.Conversations, in.Has} {
		if len(values) > 50 {
			return page, fmt.Errorf("choose at most 50 values for each search filter")
		}
	}
	for _, kind := range in.Has {
		if msgindex.SearchKind(kind) == 0 {
			return page, fmt.Errorf("has must be link, image, gif, video, or file")
		}
	}
	if in.Author != "" && in.Author != "user" && in.Author != "bot" {
		return page, fmt.Errorf("author must be user or bot")
	}
	if !in.Before.IsZero() && !in.After.IsZero() && !in.Before.After(in.After) {
		return page, fmt.Errorf("before must be later than after")
	}
	if in.Limit == 0 {
		in.Limit = 50
	}
	if in.Limit < 1 || in.Limit > 100 {
		return page, fmt.Errorf("limit must be between 1 and 100")
	}
	var at int64
	var id string
	if in.Cursor != "" {
		var err error
		at, id, err = storage.ParseSearchCursor(in.Cursor)
		if err != nil {
			return page, err
		}
	}
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.deleted {
		return page, ErrNotFound
	}
	f := storage.SearchFilter{Query: in.Query, Has: in.Has, Before: in.Before, After: in.After, Author: in.Author}
	for _, cid := range in.Conversations {
		if e := m.partyAccess(owner, cid); e != nil {
			return page, e
		}
		if _, ok := w.conversations[cid]; !ok {
			return page, ErrNotFound
		}
		f.Conversations = append(f.Conversations, cid)
	}
	if len(in.Conversations) == 0 {
		for cid := range w.conversations {
			if m.partyAccess(owner, cid) != nil {
				continue
			}
			f.Conversations = append(f.Conversations, cid)
		}
		if len(f.Conversations) == 0 {
			f.MatchNone = true
		}
	}
	member := func(id string) (Member, bool) {
		if id == w.self.ID {
			return w.self, true
		}
		v, ok := w.members[id]
		return v, ok
	}
	for _, id := range in.From {
		if _, ok := member(id); ok {
			f.From = append(f.From, id)
		}
	}
	selfMention := false
	for _, id := range in.Mentions {
		if id == w.self.ID {
			selfMention = true
			continue
		}
		if v, ok := member(id); ok && v.Nick != "" {
			f.Mentions = append(f.Mentions, v.Nick)
		}
	}
	if selfMention {
		f.MentionsByConversation = map[string][]string{}
		shared := slices.Clone(f.Mentions)
		for _, conversation := range f.Conversations {
			network := w.conversations[conversation].NetworkID
			nick := w.networkSelf(network).Nick
			if network == "" {
				nick = w.self.Nick
			}
			nicks := slices.Clone(shared)
			if nick != "" {
				nicks = append(nicks, nick)
				if !slices.Contains(f.Mentions, nick) {
					f.Mentions = append(f.Mentions, nick)
				}
			}
			f.MentionsByConversation[conversation] = nicks
		}
	}
	if len(in.From) > 0 && len(f.From) == 0 || len(in.Mentions) > 0 && len(f.Mentions) == 0 {
		f.MatchNone = true
	}
	for id, v := range w.members {
		if v.Role == "bot" {
			f.Bots = append(f.Bots, id)
		}
	}
	if w.self.Role == "bot" {
		f.Bots = append(f.Bots, w.self.ID)
	}
	var messages []model.Message
	if w.store != nil {
		found, err := w.store.SearchPageContext(ctx, owner, f, in.Cursor, in.Limit)
		if ctx.Err() != nil {
			return page, ctx.Err()
		}
		if errors.Is(err, storage.ErrInvalidCursor) {
			return page, err
		}
		if err != nil {
			return page, ErrStorage
		}
		messages, page.NextCursor = found.Items, found.NextCursor
	} else {
		messages = []model.Message{}
		for cid, items := range w.messages {
			if err := ctx.Err(); err != nil {
				return page, err
			}
			if _, ok := w.conversations[cid]; !ok {
				continue
			}
			for _, msg := range items {
				if in.Cursor != "" && (msg.CreatedAt.UnixNano() > at || msg.CreatedAt.UnixNano() == at && msg.ID >= id) {
					continue
				}
				if storage.MatchesSearch(msg, f) {
					messages = append(messages, msg)
				}
			}
		}
		slices.SortFunc(messages, func(a, b model.Message) int {
			if n := b.CreatedAt.Compare(a.CreatedAt); n != 0 {
				return n
			}
			return strings.Compare(b.ID, a.ID)
		})
		if len(messages) > in.Limit {
			messages = messages[:in.Limit]
			next := storage.SearchCursor(messages[len(messages)-1])
			page.NextCursor = &next
		}
	}
	for _, msg := range messages {
		page.Items = append(page.Items, SearchResult{msg, w.conversations[msg.ConversationID]})
	}
	return clone(page), nil
}
