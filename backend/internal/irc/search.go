package irc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"ircord/backend/internal/model"
	"ircord/backend/internal/msgindex"
	"ircord/backend/internal/storage"
)

type SearchResult struct {
	Message      Message        `json:"message"`
	Conversation Conversation   `json:"conversation"`
	Context      *SearchContext `json:"context,omitempty"`
	SenderNick   string         `json:"senderNick,omitempty" maxLength:"100" doc:"Display nickname for an ephemeral upstream hit; not an account identity."`
}

// SearchContext says whether the hit can be opened in retained history.
type SearchContext struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty" enum:"not-retained,missing-msgid"`
	Identity  string `json:"identity" enum:"retained,ephemeral"`
}

type SearchSource struct {
	Requested    string `json:"requested" enum:"retained,upstream"`
	Actual       string `json:"actual" enum:"retained,upstream"`
	Coverage     string `json:"coverage" enum:"retained-indexed,upstream-bounded"`
	Completeness string `json:"completeness" enum:"unknown"`
	Reason       string `json:"reason,omitempty" enum:"unsupported-scope,unsupported-filters,capability-unavailable,upstream-unavailable,continuation"`
}

// SearchValidationError contains only a public search validation message.
type SearchValidationError struct{ Detail string }

func (e *SearchValidationError) Error() string { return e.Detail }
func invalidSearch(detail string) error        { return &SearchValidationError{Detail: detail} }

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
	Author, Cursor, Source             string
	Limit                              int
}

type SearchPage struct {
	Items      []SearchResult `json:"items" nullable:"false"`
	NextCursor *string        `json:"nextCursor"`
	Source     *SearchSource  `json:"source,omitempty"`
}

func searchScope(owner string, in SearchQuery) string {
	canonical := func(values []string) []string {
		out := append([]string{}, values...)
		slices.Sort(out)
		return slices.Compact(out)
	}
	value := struct {
		Owner, Query, Author, Source       string
		HasQuery                           bool
		From, Mentions, Conversations, Has []string
		Before, After                      time.Time
	}{owner, strings.Join(msgindex.SearchTokens(in.Query), " "), in.Author, in.Source, in.Query != "", canonical(in.From), canonical(in.Mentions), canonical(in.Conversations), canonical(in.Has), in.Before.UTC(), in.After.UTC()}
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:16])
}

// SearchMessages searches only retained history visible to this account.
// Unknown conversations reject the entire list. Unknown members match nothing.
// Ignored senders remain searchable, as with stored timeline history.
func (m *Manager) SearchMessages(owner string, in SearchQuery) (SearchPage, error) {
	return m.SearchMessagesContext(context.Background(), owner, in)
}

func (m *Manager) SearchMessagesContext(ctx context.Context, owner string, in SearchQuery) (SearchPage, error) {
	page, err := m.searchRetained(ctx, owner, in)
	if err != nil || in.Source != "upstream" || in.Cursor != "" || page.Source == nil || page.Source.Reason != "capability-unavailable" {
		return page, err
	}
	w := m.space(owner)
	w.mu.Lock()
	conv := w.conversations[in.Conversations[0]]
	c := w.connections[conv.NetworkID]
	w.mu.Unlock()
	if conv.Party != nil {
		return searchFallbackReason(page, owner, in, "unsupported-scope"), nil
	}
	if c == nil {
		return page, nil
	}
	target := c.target(conv)
	if target == "" {
		return page, nil
	}
	c.mu.Lock()
	usable := c.searchUsableLocked()
	c.mu.Unlock()
	if !usable {
		return page, nil
	}
	events, attempt, upstreamErr := c.upstreamSearch(ctx, target, strings.TrimSpace(in.Query), in.LimitOrDefault())
	if ctx.Err() != nil {
		return SearchPage{}, ctx.Err()
	}
	// Re-read authorization and retained data after the wait, even on fallback.
	page, err = m.searchRetained(ctx, owner, in)
	if err != nil {
		return page, err
	}
	if upstreamErr != nil {
		return searchFallbackReason(page, owner, in, "upstream-unavailable"), nil
	}
	remote, err := m.searchUpstreamResults(ctx, owner, in, c, conv, attempt, events)
	if errors.Is(err, errSearchUnavailable) {
		return searchFallbackReason(page, owner, in, "upstream-unavailable"), nil
	}
	return remote, err
}

func searchFallbackReason(page SearchPage, owner string, in SearchQuery, reason string) SearchPage {
	page.Source.Reason = reason
	if page.NextCursor != nil && len(page.Items) > 0 {
		in.Query = strings.TrimSpace(in.Query)
		next := storage.ScopedSearchCursor(page.Items[len(page.Items)-1].Message, searchScope(owner, in), reason)
		page.NextCursor = &next
	}
	return page
}

func (in SearchQuery) LimitOrDefault() int {
	if in.Limit == 0 {
		return 50
	}
	return in.Limit
}

func (m *Manager) searchRetained(ctx context.Context, owner string, in SearchQuery) (SearchPage, error) {
	page := SearchPage{Items: []SearchResult{}}
	if err := ctx.Err(); err != nil {
		return page, err
	}
	in.Query = strings.TrimSpace(in.Query)
	filtered := len(in.From)+len(in.Mentions)+len(in.Conversations)+len(in.Has) > 0 || !in.Before.IsZero() || !in.After.IsZero() || in.Author != ""
	if utf8.RuneCountInString(in.Query) > 200 || !filtered && utf8.RuneCountInString(in.Query) < 2 {
		return page, invalidSearch("enter at least two characters or choose a filter; search text can have at most 200 characters")
	}
	for _, values := range [][]string{in.From, in.Mentions, in.Conversations, in.Has} {
		if len(values) > 50 {
			return page, invalidSearch("choose at most 50 values for each search filter")
		}
	}
	for _, kind := range in.Has {
		if msgindex.SearchKind(kind) == 0 {
			return page, invalidSearch("has must be link, image, gif, video, or file")
		}
	}
	if in.Author != "" && in.Author != "user" && in.Author != "bot" {
		return page, invalidSearch("author must be user or bot")
	}
	if !in.Before.IsZero() && !in.After.IsZero() && !in.Before.After(in.After) {
		return page, invalidSearch("before must be later than after")
	}
	if in.Limit == 0 {
		in.Limit = 50
	}
	if in.Limit < 1 || in.Limit > 100 {
		return page, invalidSearch("limit must be between 1 and 100")
	}
	if in.Source != "" && in.Source != "retained" && in.Source != "upstream" {
		return page, invalidSearch("source must be retained or upstream")
	}
	scope := searchScope(owner, in)
	reason := ""
	var at int64
	var id string
	if in.Cursor != "" {
		var err error
		at, id, reason, err = storage.CheckSearchCursor(in.Cursor, scope)
		if err != nil {
			return page, err
		}
		if reason != "" && !slices.Contains([]string{"unsupported-scope", "unsupported-filters", "capability-unavailable", "upstream-unavailable", "continuation"}, reason) {
			return page, storage.ErrInvalidCursor
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
	if in.Source != "" {
		if in.Source == "upstream" && reason == "" {
			switch {
			case in.Cursor != "":
				reason = "continuation"
			case len(in.Conversations) != 1:
				reason = "unsupported-scope"
			case len(in.From)+len(in.Mentions)+len(in.Has) > 0 || in.Author != "" || !in.Before.IsZero() || !in.After.IsZero() || in.Query == "":
				reason = "unsupported-filters"
			default:
				reason = "capability-unavailable"
			}
		}
		page.Source = &SearchSource{Requested: in.Source, Actual: "retained", Coverage: "retained-indexed", Completeness: "unknown", Reason: reason}
	}
	if page.NextCursor != nil && len(messages) > 0 {
		if len(messages[len(messages)-1].ID) > 200 {
			return SearchPage{}, ErrStorage
		}
		next := storage.ScopedSearchCursor(messages[len(messages)-1], scope, reason)
		page.NextCursor = &next
	}
	for _, msg := range messages {
		result := SearchResult{Message: msg, Conversation: w.conversations[msg.ConversationID]}
		if in.Source != "" {
			result.Context = &SearchContext{Available: true, Identity: "retained"}
		}
		page.Items = append(page.Items, result)
	}
	return clone(page), nil
}
