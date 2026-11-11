package irc

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"ircord/backend/internal/msgindex"
	"ircord/backend/internal/storage"
)

// LinkSourcesPerCard bounds how many posts one link card lists.
const LinkSourcesPerCard = 5

// LinkSource is one post of a link: where and when it was posted, by whom, the
// URL exactly as posted, and a plain-text excerpt of the message.
type LinkSource struct {
	MessageID      string    `json:"messageId" doc:"Canonical IRCord message ID, for jumping to the post"`
	ConversationID string    `json:"conversationId"`
	NetworkID      string    `json:"networkId"`
	NSFW           bool      `json:"nsfw" doc:"Current sensitive-content classification for this source channel"`
	NSFWKnown      bool      `json:"nsfwKnown" doc:"False when the canonical source identity or shared classification is unavailable"`
	SenderID       string    `json:"senderId"`
	PostedAt       time.Time `json:"postedAt"`
	URL            string    `json:"url" doc:"The link as it was posted, including its query string"`
	Excerpt        string    `json:"excerpt" doc:"The message text without IRC formatting, bounded like inbox excerpts"`
}

// LinkCard is one resource of the link catalog. Every count and source comes
// from the signed-in account's own stored history under the same filter.
type LinkCard struct {
	Identity     string       `json:"identity" doc:"Groups posts of the same resource; opaque"`
	URL          string       `json:"url" doc:"The most recent post's URL"`
	Host         string       `json:"host"`
	Kind         string       `json:"kind" enum:"link,image,gif,video,audio,youtube"`
	Count        int          `json:"count" doc:"How many matching posts name this resource"`
	LastPostedAt time.Time    `json:"lastPostedAt"`
	Sources      []LinkSource `json:"sources" nullable:"false" maxItems:"5" doc:"The newest matching posts"`
}

type LinkPage struct {
	Items      []LinkCard `json:"items" nullable:"false"`
	NextCursor *string    `json:"nextCursor"`
	Indexing   bool       `json:"indexing" doc:"History stored before the link catalog existed is still being indexed, so older links may be missing"`
}

// LinkQuery is a link listing request.
type LinkQuery struct {
	Conversations []string
	Network       string
	Query         string
	Host          string
	Kinds         []string
	Since, Until  time.Time
	Cursor        string
	Limit         int
}

// Links pages the account's link catalog: links posted in its channels,
// grouped by resource, most recently posted first.
func (m *Manager) Links(owner string, in LinkQuery) (LinkPage, error) {
	page := LinkPage{Items: []LinkCard{}}
	if in.Limit < 1 || in.Limit > 50 {
		return page, fmt.Errorf("limit must be between 1 and 50")
	}
	in.Query = strings.TrimSpace(in.Query)
	if len(in.Query) > 200 {
		return page, fmt.Errorf("query must have at most 200 bytes")
	}
	if strings.TrimSpace(in.Host) != "" {
		host, ok := msgindex.FilterHost(in.Host)
		if !ok {
			return page, fmt.Errorf("host must be a host name such as example.com or an IP address")
		}
		in.Host = host
	} else {
		in.Host = ""
	}
	if len(in.Conversations) > 50 {
		return page, fmt.Errorf("at most 50 conversations can be selected")
	}
	if !in.Until.IsZero() && !in.Since.IsZero() && !in.Until.After(in.Since) {
		return page, fmt.Errorf("until must be after since")
	}
	w := m.space(owner)
	w.mu.Lock()
	if w.deleted {
		w.mu.Unlock()
		return page, ErrNotFound
	}
	for _, id := range in.Conversations {
		if m.partyAccess(owner, id) != nil {
			w.mu.Unlock()
			return page, ErrNotFound
		}
		if conv, ok := w.conversations[id]; !ok || conv.Kind != "channel" {
			w.mu.Unlock()
			return page, ErrNotFound
		}
	}
	store := w.store
	w.mu.Unlock()
	// The catalog lives in durable storage; a workspace without it has none.
	if store == nil {
		return page, nil
	}
	groups, next, err := store.Links(owner, storage.LinkFilter{Conversations: in.Conversations, Network: in.Network, Query: in.Query, Host: in.Host, Kinds: in.Kinds, Since: in.Since, Until: in.Until}, in.Cursor, in.Limit, LinkSourcesPerCard)
	if errors.Is(err, storage.ErrInvalidCursor) {
		return page, err
	}
	if err != nil {
		return page, ErrStorage
	}
	for _, g := range groups {
		if len(g.Sources) == 0 {
			continue
		}
		newest := g.Sources[0]
		card := LinkCard{Identity: g.Identity, URL: newest.URL, Host: newest.Host, Kind: newest.Kind, Count: g.Count, LastPostedAt: g.Last, Sources: make([]LinkSource, 0, len(g.Sources))}
		for _, s := range g.Sources {
			if m.partyAccess(owner, s.ConversationID) != nil {
				continue
			}
			text, _ := excerpt(s.Message.Text)
			nsfw, known := m.linkSourceNSFW(w, s.ConversationID, s.NetworkID)
			card.Sources = append(card.Sources, LinkSource{MessageID: s.MessageID, ConversationID: s.ConversationID, NetworkID: s.NetworkID, NSFW: nsfw, NSFWKnown: known, SenderID: s.Message.SenderID, PostedAt: s.CreatedAt, URL: s.URL, Excerpt: text})
		}
		if len(card.Sources) > 0 {
			page.Items = append(page.Items, card)
		}
	}
	page.NextCursor = next
	complete, err := store.LinkIndexComplete()
	if err != nil {
		return page, ErrStorage
	}
	page.Indexing = !complete
	return page, nil
}

// linkSourceNSFW uses the last verified canonical route saved with this
// account's conversation, even while disconnected or after a restart. Legacy
// conversations without one remain explicitly unknown until a verified join;
// a same-named channel or changed account-local network route is no substitute.
func (m *Manager) linkSourceNSFW(w *workspace, conversation, network string) (bool, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	conv, ok := w.conversations[conversation]
	if !ok || w.deleted || conv.NetworkID != network || conv.Kind != "channel" || conv.Party != nil {
		return false, false
	}
	if identity := conv.ChannelIdentity; identity != nil {
		value, known := m.sharedChannel(identity.Network, identity.Channel)
		if known {
			return value.NSFW, true
		}
	}
	return conv.ChannelSettings != nil && conv.ChannelSettings.NSFW, false
}
