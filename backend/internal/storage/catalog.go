package storage

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"ircord/backend/internal/model"
)

// catalogRows selects the link catalog's rows of the message link index: the
// links of message text, which carry an identity. It matches the partial
// indexes of migration 19 (addLinkCatalog).
const catalogRows = "l.identity!=''"

// LinkFilter narrows a link listing. Zero values do not filter.
type LinkFilter struct {
	// Conversations keeps links posted in these conversations.
	Conversations []string
	// Network keeps links posted on this network.
	Network string
	// Query keeps links whose URL or message text contains it,
	// ignoring ASCII case.
	Query string
	// Host keeps links to this host or its subdomains.
	Host  string
	Kinds []string
	// Since and Until bound when a link was posted (Until exclusive).
	Since, Until time.Time
}

// LinkSourceRow is one post of a link, with its stored message.
type LinkSourceRow struct {
	MessageID      string
	ConversationID string
	NetworkID      string
	URL            string
	Kind           string
	Host           string
	Previewed      bool
	CreatedAt      time.Time
	Message        model.Message
}

// LinkGroup is one resource and the posts of it a filter matches, newest first.
type LinkGroup struct {
	Identity string
	Count    int
	Last     time.Time
	Sources  []LinkSourceRow
}

// A link page cursor is the last card's newest post time and identity, as
// "<unix ns>\x00<identity>" in URL-safe base64. Identities are at most
// msgindex.MaxLinkLength bytes, so a cursor stays under 2.8 KiB.
type linkCursor struct {
	At int64
	ID string
}

// maxLinkCursor bounds an accepted cursor: base64 of the time, a separator and
// the longest identity, with room to spare.
const maxLinkCursor = 4096

func linkPageCursor(g LinkGroup) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(g.Last.UnixNano(), 10) + "\x00" + g.Identity))
}

func parseLinkCursor(raw string) (linkCursor, error) {
	var c linkCursor
	if len(raw) > maxLinkCursor {
		return c, ErrInvalidCursor
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return c, ErrInvalidCursor
	}
	at, id, ok := strings.Cut(string(b), "\x00")
	if !ok || id == "" {
		return c, ErrInvalidCursor
	}
	if c.At, err = strconv.ParseInt(at, 10, 64); err != nil || c.At <= 0 {
		return c, ErrInvalidCursor
	}
	c.ID = id
	return c, nil
}

// linkScope is the FROM/WHERE shared by the group listing and its sources, so
// every count and every source shown come from the same authorized rows: links
// of the owner's own stored, undeleted messages in channels still in their
// workspace, from senders they do not ignore.
func linkScope(owner string, f LinkFilter) (string, []any) {
	q := " FROM message_links l JOIN messages m ON m.owner=l.owner AND m.id=l.message AND m.conversation=l.conversation JOIN entities e ON e.owner=l.owner AND e.kind='conversation' AND e.id=l.conversation WHERE l.owner=? AND " + catalogRows + " AND json_extract(e.payload,'$.kind')='channel' AND l.sender NOT IN (SELECT member FROM ignored_senders WHERE owner=l.owner)"
	args := []any{owner}
	if len(f.Conversations) > 0 {
		q += " AND l.conversation IN (?" + strings.Repeat(",?", len(f.Conversations)-1) + ")"
		for _, c := range f.Conversations {
			args = append(args, c)
		}
	}
	if f.Network != "" {
		q += " AND json_extract(e.payload,'$.networkId')=?"
		args = append(args, f.Network)
	}
	if f.Host != "" {
		host := strings.ToLower(strings.TrimSuffix(f.Host, "."))
		q += " AND (l.host=? OR substr(l.host,-length(?)-1)=('.'||?))"
		args = append(args, host, host, host)
	}
	if len(f.Kinds) > 0 {
		q += " AND l.kind IN (?" + strings.Repeat(",?", len(f.Kinds)-1) + ")"
		for _, k := range f.Kinds {
			args = append(args, k)
		}
	}
	if !f.Since.IsZero() {
		q += " AND l.created_ns>=?"
		args = append(args, f.Since.UnixNano())
	}
	if !f.Until.IsZero() {
		q += " AND l.created_ns<?"
		args = append(args, f.Until.UnixNano())
	}
	if f.Query != "" {
		q += " AND (instr(lower(l.url),lower(?))>0 OR instr(lower(m.text),lower(?))>0)"
		args = append(args, f.Query, f.Query)
	}
	return q, args
}

// Links pages owner's link catalog grouped by resource, most recently posted
// first. Each group carries its matching post count and at most sources of
// those posts. It reads one group past limit to tell whether another page exists.
func (s *Store) Links(owner string, f LinkFilter, after string, limit, sources int) ([]LinkGroup, *string, error) {
	scope, args := linkScope(owner, f)
	query := "SELECT l.identity,max(l.created_ns) AS last,count(DISTINCT l.message)" + scope + " GROUP BY l.identity"
	if after != "" {
		c, err := parseLinkCursor(after)
		if err != nil {
			return nil, nil, err
		}
		query += " HAVING last<? OR (last=? AND l.identity<?)"
		args = append(args, c.At, c.At, c.ID)
	}
	query += " ORDER BY last DESC,l.identity DESC LIMIT ?"
	args = append(args, limit+1)
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, nil, err
	}
	groups := []LinkGroup{}
	for rows.Next() {
		var g LinkGroup
		var at int64
		if err = rows.Scan(&g.Identity, &at, &g.Count); err != nil {
			rows.Close()
			return nil, nil, err
		}
		g.Last = time.Unix(0, at).UTC()
		groups = append(groups, g)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}
	var next *string
	if len(groups) > limit {
		groups = groups[:limit]
		c := linkPageCursor(groups[len(groups)-1])
		next = &c
	}
	if err = s.linkSources(owner, f, groups, sources); err != nil {
		return nil, nil, err
	}
	return groups, next, nil
}

// linkSources fills the newest posts of each group under the same scope, in
// one query for the whole page. A message naming one resource several times
// (two spellings) is one post, represented by its first link that matches the
// filter, so every counted post has a source.
func (s *Store) linkSources(owner string, f LinkFilter, groups []LinkGroup, limit int) error {
	if len(groups) == 0 {
		return nil
	}
	scope, args := linkScope(owner, f)
	scope += " AND l.identity IN (?" + strings.Repeat(",?", len(groups)-1) + ")"
	at := map[string]int{}
	for i, g := range groups {
		args = append(args, g.Identity)
		at[g.Identity] = i
	}
	// SQLite reads the bare columns of a min() aggregate from the row that holds the minimum.
	posts := "SELECT l.identity AS identity,l.message AS message,min(l.idx) AS idx,l.conversation AS conversation,coalesce(json_extract(e.payload,'$.networkId'),'') AS network,l.url AS url,l.kind AS kind,l.host AS host,l.previewed AS previewed,l.created_ns AS created_ns,m.payload AS payload" + scope + " GROUP BY l.identity,l.message"
	query := "SELECT identity,message,conversation,network,url,kind,host,previewed,created_ns,payload FROM (SELECT *,row_number() OVER (PARTITION BY identity ORDER BY created_ns DESC,message DESC) AS n FROM (" + posts + ")) WHERE n<=? ORDER BY identity,n"
	args = append(args, limit)
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for i := range groups {
		groups[i].Sources = []LinkSourceRow{}
	}
	for rows.Next() {
		var r LinkSourceRow
		var identity string
		var created int64
		var raw []byte
		if err = rows.Scan(&identity, &r.MessageID, &r.ConversationID, &r.NetworkID, &r.URL, &r.Kind, &r.Host, &r.Previewed, &created, &raw); err != nil {
			return err
		}
		if err = json.Unmarshal(raw, &r.Message); err != nil {
			return err
		}
		r.CreatedAt = time.Unix(0, created).UTC()
		if i, ok := at[identity]; ok {
			groups[i].Sources = append(groups[i].Sources, r)
		}
	}
	return rows.Err()
}
