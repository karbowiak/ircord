package irc

import (
	"strconv"
	"time"

	"github.com/lrstanley/girc"
)

// metadataRetryUnit is the unit of RPL_METADATASYNCLATER's retry-after, a variable for tests.
var metadataRetryUnit = time.Second

// metadataKeys are the draft/metadata-3 keys IRCord shows for other users.
var metadataKeys = []string{"display-name", "avatar", "color", "bio"}

const (
	// metadataJoinDelay coalesces the SYNCs for users joining a shared channel.
	metadataJoinDelay = 250 * time.Millisecond
	// metadataJoinBurst is the number of users joining one channel within the delay above
	// that are synced with one channel SYNC instead of one SYNC each (a netjoin, say).
	metadataJoinBurst = 5
	// metadataRetries bounds how often a target postponed with RPL_METADATASYNCLATER is retried.
	metadataRetries = 3
)

// A server shares a user's metadata only with sessions that are subscribed and already share
// a channel with that user when it changes, plus a joining session for the channel it joins.
// Everything else has to be asked for with METADATA <target> SYNC:
//   - members restored from storage keep whatever was seen before IRCord went away, so each
//     channel is synced once its member list is complete (our own NAMES reply, or on
//     subscribing for channels a bouncer such as Ergo's always-on mode had already rejoined);
//   - a user who set their profile before joining a channel we are in was never shared with
//     us, so a user joining a shared channel is synced too.
//
// The batch answering a SYNC lists every subscribed key that is set for its target, so a key
// missing from it is not set any more and is cleared.

// metadataSync collects one server "metadata" batch answering a SYNC IRCord sent.
type metadataSync struct {
	target string
	// nicks are the target's users when the batch opened. Users who join while the batch is
	// being delivered were not part of the reply and are left alone.
	nicks []string
	seen  map[string]map[string]bool
}

// subscribeMetadata subscribes to the profile keys, checks and shares our own profile and syncs
// the channels already joined. It takes only c.mu, so callers may hold the workspace mutex.
func (c *connection) subscribeMetadata(client *girc.Client) {
	_ = c.enqueue(&girc.Event{Command: "METADATA", Params: append([]string{"*", "SUB"}, subscribedKeys()...)})
	// Our own profile goes first: a long SYNC burst must not delay or crowd it out.
	c.checkPublishedProfile()
	c.mu.Lock()
	c.metadataSubscribed = true
	c.mu.Unlock()
	c.requestMetadataSync(client.ChannelList()...)
}

// requestMetadataSync asks for the metadata of channels or nicks, in one queue slot. Only the
// batch answering such a request is treated as complete.
func (c *connection) requestMetadataSync(targets ...string) {
	c.requestMetadataSyncForAttempt(nil, targets...)
}

// A timer supplies its captured client so it cannot enqueue into a new attempt.
func (c *connection) requestMetadataSyncForAttempt(client *girc.Client, targets ...string) {
	if len(targets) == 0 {
		return
	}
	events := make([]*girc.Event, 0, len(targets))
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.metadataSubscribed || (client != nil && (c.client != client || !c.ready || c.transport == nil)) {
		// Before SUB the server would answer with no keys at all, which reads as "all cleared".
		return
	}
	if c.metadataRequested == nil {
		c.metadataRequested = map[string]bool{}
	}
	for _, target := range targets {
		c.metadataRequested[girc.ToRFC1459(target)] = true
		events = append(events, &girc.Event{Command: "METADATA", Params: []string{target, "SYNC"}})
	}
	if err := c.enqueueLocked(events...); err != nil {
		for _, target := range targets {
			delete(c.metadataRequested, girc.ToRFC1459(target))
		}
	}
}

// metadataMembership reacts to membership events: the NAMES reply completing our own JOIN
// syncs that channel, and another user's live JOIN syncs that user.
func (c *connection) metadataMembership(client *girc.Client, e girc.Event) {
	if !c.has("draft/metadata-3") {
		return
	}
	switch {
	case e.Command == "JOIN" && e.Source != nil && len(e.Params) > 0 && e.Tags["batch"] == "":
		if c.fold(e.Source.Name) == c.fold(client.GetNick()) {
			c.mu.Lock()
			if c.metadataJoined == nil {
				c.metadataJoined = map[string]bool{}
			}
			c.metadataJoined[girc.ToRFC1459(e.Params[0])] = true
			c.mu.Unlock()
		} else {
			c.queueJoinSync(e.Params[0], e.Source.Name)
		}
	case e.Command == "366" && len(e.Params) >= 2 && girc.IsValidChannel(e.Params[1]):
		// Only the NAMES reply that follows our own JOIN; a NAMES the user asked for is not a
		// reason to sync again.
		key := girc.ToRFC1459(e.Params[1])
		c.mu.Lock()
		joined := c.metadataJoined[key]
		delete(c.metadataJoined, key)
		c.mu.Unlock()
		if joined {
			c.requestMetadataSync(e.Params[1])
		}
	}
}

// queueJoinSync batches the SYNCs for users joining shared channels.
func (c *connection) queueJoinSync(channel, nick string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.metadataJoins == nil {
		c.metadataJoins = map[string]map[string]string{}
	}
	key := girc.ToRFC1459(channel)
	if c.metadataJoins[key] == nil {
		// The empty key keeps the channel's own spelling.
		c.metadataJoins[key] = map[string]string{"": channel}
	}
	c.metadataJoins[key][girc.ToRFC1459(nick)] = nick
	if c.metadataJoinTimer != nil {
		return
	}
	client := c.client
	c.metadataJoinTimer = time.AfterFunc(metadataJoinDelay, func() { c.flushJoinSyncs(client) })
}

func (c *connection) flushJoinSyncs(client *girc.Client) {
	c.mu.Lock()
	if !c.ready || c.client != client || c.ctx.Err() != nil {
		c.mu.Unlock()
		return
	}
	joins := c.metadataJoins
	c.metadataJoins = nil
	c.metadataJoinTimer = nil
	c.mu.Unlock()
	var targets []string
	nicks := map[string]string{}
	for _, users := range joins {
		if len(users)-1 > metadataJoinBurst {
			targets = append(targets, users[""])
			continue
		}
		for key, nick := range users {
			if key != "" {
				nicks[key] = nick
			}
		}
	}
	for _, nick := range nicks {
		targets = append(targets, nick)
	}
	c.requestMetadataSyncForAttempt(client, targets...)
}

// openMetadataSync is called with c.mu held when a metadata batch starts.
func (c *connection) openMetadataSync(ref string, e girc.Event) {
	if len(e.Params) < 3 || e.Params[1] != "metadata" {
		return
	}
	target := e.Params[2]
	key := girc.ToRFC1459(target)
	if !c.metadataRequested[key] {
		return
	}
	delete(c.metadataRequested, key)
	delete(c.metadataRetries, key)
	nicks := []string{target}
	if girc.IsValidChannel(target) {
		nicks = nil
		if c.client != nil {
			if channel := c.client.LookupChannel(target); channel != nil {
				nicks = channel.UserList
			}
		}
	}
	if c.metadataSyncs == nil {
		c.metadataSyncs = map[string]*metadataSync{}
	}
	if len(c.metadataSyncs) < 32 {
		c.metadataSyncs[ref] = &metadataSync{target: target, nicks: nicks, seen: map[string]map[string]bool{}}
	}
}

// sawMetadata records a value delivered inside a metadata batch. Called with the workspace
// mutex held.
func (c *connection) sawMetadata(e girc.Event, memberID, key string) {
	ref := e.Tags["batch"]
	if ref == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if s := c.metadataSyncs[ref]; s != nil {
		if s.seen[memberID] == nil {
			s.seen[memberID] = map[string]bool{}
		}
		s.seen[memberID][key] = true
	}
}

// finishMetadataSync clears the keys the synced users no longer have: a value removed while
// IRCord was not watching produces no notification, only its absence here.
func (c *connection) finishMetadataSync(s *metadataSync) {
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()
	if client == nil {
		return
	}
	present := map[string]bool{}
	if girc.IsValidChannel(s.target) {
		channel := client.LookupChannel(s.target)
		if channel == nil {
			return
		}
		for _, nick := range channel.UserList {
			present[girc.ToRFC1459(nick)] = true
		}
	} else if client.LookupUser(s.target) != nil {
		present[girc.ToRFC1459(s.target)] = true
	}
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, nick := range s.nicks {
		if !present[girc.ToRFC1459(nick)] {
			continue
		}
		memberID := c.nicks[c.fold(nick)]
		v, ok := w.members[memberID]
		if memberID == "" || !ok || memberID == w.self.ID {
			continue
		}
		changed := false
		for _, key := range subscribedKeys() {
			if !s.seen[memberID][key] && (key != "display-name" || v.NameSource == "profile") {
				changed = clearMetadata(&v, key) || changed
			}
		}
		if changed {
			w.members[v.ID] = v
			w.emit("member.updated", v)
		}
	}
}

// clearMetadata removes one metadata value from a member and reports whether it changed.
func clearMetadata(v *Member, key string) bool {
	before := *v
	switch key {
	case "display-name":
		v.NameSource = ""
		resolveMemberName(v)
	case "avatar":
		v.AvatarURL = ""
	case "color":
		v.NameColor = ""
	case "bio":
		v.Bio = ""
	case deviceKey:
		v.Device = ""
	}
	return !equal(before, *v)
}

// retryMetadataSync honours RPL_METADATASYNCLATER (774 <client> <target> [<retry-after>]) for
// a sync IRCord requested, a few times per target and connection.
func (c *connection) retryMetadataSync(e girc.Event) {
	if len(e.Params) < 2 {
		return
	}
	target := e.Params[1]
	key := girc.ToRFC1459(target)
	delay := metadataRetryUnit
	if len(e.Params) >= 3 {
		if seconds, err := strconv.Atoi(e.Params[2]); err == nil && seconds > 0 {
			delay = time.Duration(min(seconds, 60)) * metadataRetryUnit
		}
	}
	c.mu.Lock()
	if !c.metadataRequested[key] || c.metadataRetries[key] >= metadataRetries {
		delete(c.metadataRequested, key)
		c.mu.Unlock()
		return
	}
	if c.metadataRetries == nil {
		c.metadataRetries = map[string]int{}
	}
	c.metadataRetries[key]++
	client := c.client
	c.mu.Unlock()
	time.AfterFunc(delay, func() {
		c.requestMetadataSyncForAttempt(client, target)
	})
}
