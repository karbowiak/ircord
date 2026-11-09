package storage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"time"

	"ircord/backend/internal/model"
	"ircord/backend/internal/msgindex"
)

// SearchFilter contains account-visible IDs and current nicks resolved by the manager.
type SearchFilter struct {
	Query                               string
	From, Conversations, Mentions, Bots []string
	Has                                 []string
	Before, After                       time.Time
	Author                              string
	MatchNone                           bool
}

type searchCursor struct {
	V  int    `json:"v"`
	At int64  `json:"at"`
	ID string `json:"id"`
}

func SearchCursor(msg model.Message) string {
	raw, _ := json.Marshal(searchCursor{1, msg.CreatedAt.UnixNano(), msg.ID})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func ParseSearchCursor(raw string) (int64, string, error) {
	var c searchCursor
	if len(raw) > 512 {
		return 0, "", ErrInvalidCursor
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || json.Unmarshal(b, &c) != nil || c.V != 1 || c.ID == "" || len(c.ID) > 200 {
		return 0, "", ErrInvalidCursor
	}
	return c.At, c.ID, nil
}

func searchFTS(raw string, prefix bool) string {
	terms := msgindex.SearchTokens(raw)
	if len(terms) == 0 {
		return ""
	}
	phrase := `"` + strings.Join(terms, " ") + `"`
	if prefix {
		phrase += "*"
	}
	return phrase
}

// searchSQL uses rowid lookups from FTS for text, and ordered account indexes
// for filter-only requests. Every branch keeps the owner predicate.
func searchSQL(owner string, f SearchFilter, cursor string) (string, []any, error) {
	query := "SELECT m.payload FROM messages m WHERE m.owner=? AND m.search_visible=1"
	args := []any{owner}
	if f.MatchNone {
		query += " AND 0"
	}
	list := func(column string, values []string, exclude bool) {
		if len(values) == 0 {
			return
		}
		op := " IN ("
		if exclude {
			op = " NOT IN ("
		}
		query += " AND " + column + op + strings.TrimSuffix(strings.Repeat("?,", len(values)), ",") + ")"
		for _, v := range values {
			args = append(args, v)
		}
	}
	list("m.sender", f.From, false)
	list("m.conversation", f.Conversations, false)
	if f.Author != "" {
		if f.Author == "bot" && len(f.Bots) == 0 {
			query += " AND 0"
		}
		list("m.sender", f.Bots, f.Author == "user")
	}
	if !f.Before.IsZero() {
		query += " AND m.created_ns<?"
		args = append(args, f.Before.UnixNano())
	}
	if !f.After.IsZero() {
		query += " AND m.created_ns>=?"
		args = append(args, f.After.UnixNano())
	}
	if len(f.Has) > 0 {
		mask := 0
		for _, kind := range f.Has {
			mask |= msgindex.SearchKind(kind)
		}
		query += " AND (m.search_kinds & ?)!=0"
		args = append(args, mask)
	}
	fts := ""
	if f.Query != "" {
		fts = searchFTS(f.Query, true)
		if fts == "" {
			query += " AND 0"
		}
	}
	if len(f.Mentions) > 0 {
		parts := []string{}
		allTokenized := true
		for _, nick := range f.Mentions {
			if p := searchFTS(nick, false); p != "" {
				parts = append(parts, p)
			} else {
				allTokenized = false
			}
		}
		// Punctuation-only nicks use the boundary post-filter without an FTS pre-filter.
		if allTokenized {
			mentions := "(" + strings.Join(parts, " OR ") + ")"
			if fts != "" {
				fts = "(" + fts + ") AND " + mentions
			} else {
				fts = mentions
			}
		}
	}
	if fts != "" {
		// CROSS JOIN fixes the loop order: visit FTS hits, then their message
		// rowids, rather than walking every message of a large account.
		query = strings.Replace(query, "FROM messages m", "FROM message_search CROSS JOIN messages m ON m.rowid=message_search.rowid", 1)
		query += " AND message_search MATCH ?"
		args = append(args, fts)
	}
	if cursor != "" {
		at, id, err := ParseSearchCursor(cursor)
		if err != nil {
			return "", nil, err
		}
		query += " AND (m.created_ns,m.id)<(?,?)"
		args = append(args, at, id)
	}
	return query + " ORDER BY m.created_ns DESC,m.id DESC", args, nil
}

func mentionPatterns(nicks []string) []*regexp.Regexp {
	patterns := make([]*regexp.Regexp, 0, len(nicks))
	for _, nick := range nicks {
		patterns = append(patterns, msgindex.MentionPattern(nick))
	}
	return patterns
}

func matchesMentions(text string, patterns []*regexp.Regexp) bool {
	return len(patterns) == 0 || slices.ContainsFunc(patterns, func(p *regexp.Regexp) bool { return p.MatchString(text) })
}

// SearchPage fills pages after the mention boundary post-filter, so rejected
// FTS candidates cannot leave gaps. The query never decodes JSON payloads.
func (s *Store) SearchPage(owner string, f SearchFilter, cursor string, limit int) (model.MessagePage, error) {
	return s.SearchPageContext(context.Background(), owner, f, cursor, limit)
}

func (s *Store) SearchPageContext(ctx context.Context, owner string, f SearchFilter, cursor string, limit int) (model.MessagePage, error) {
	page := model.MessagePage{Items: []model.Message{}}
	query, args, err := searchSQL(owner, f, cursor)
	if err != nil {
		return page, err
	}
	patterns := mentionPatterns(f.Mentions)
	if len(patterns) == 0 {
		query += " LIMIT ?"
		args = append(args, limit+1)
	}
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		var msg model.Message
		if err = rows.Scan(&raw); err != nil {
			return page, err
		}
		if err = json.Unmarshal(raw, &msg); err != nil {
			return page, err
		}
		if msg.Deleted || !matchesMentions(msg.Text, patterns) {
			continue
		}
		if len(page.Items) == limit {
			next := SearchCursor(page.Items[len(page.Items)-1])
			page.NextCursor = &next
			break
		}
		page.Items = append(page.Items, msg)
	}
	return page, rows.Err()
}

// MatchesSearch keeps the in-memory fallback aligned with SQL filters.
func MatchesSearch(msg model.Message, f SearchFilter) bool {
	if f.MatchNone || msg.Deleted || f.Query != "" && !msgindex.SearchPhrase(msg.Text, f.Query) {
		return false
	}
	if len(f.From) > 0 && !slices.Contains(f.From, msg.SenderID) {
		return false
	}
	if len(f.Conversations) > 0 && !slices.Contains(f.Conversations, msg.ConversationID) {
		return false
	}
	if !f.Before.IsZero() && !msg.CreatedAt.Before(f.Before) || !f.After.IsZero() && msg.CreatedAt.Before(f.After) {
		return false
	}
	if f.Author != "" && slices.Contains(f.Bots, msg.SenderID) != (f.Author == "bot") {
		return false
	}
	if len(f.Has) > 0 {
		mask := 0
		for _, kind := range f.Has {
			mask |= msgindex.SearchKind(kind)
		}
		if msgindex.SearchKinds(msg)&mask == 0 {
			return false
		}
	}
	return matchesMentions(msg.Text, mentionPatterns(f.Mentions))
}

// Search retains the internal text-search convenience used by history callers.
func (s *Store) Search(owner, query, conversation string, limit int) ([]model.Message, error) {
	f := SearchFilter{Query: query}
	if conversation != "" {
		f.Conversations = []string{conversation}
	}
	page, err := s.SearchPage(owner, f, "", limit)
	return page.Items, err
}
