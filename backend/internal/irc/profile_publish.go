package irc

import (
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lrstanley/girc"
)

// Publishing our own profile as draft/metadata-3 values.
//
// A user's metadata lives on the server's client, and every session of one account shares that
// client (Ergo's multiclient and always-on modes). Two IRCord backends signed in to the same
// upstream account, such as production and a local instance, therefore publish over each
// other's values: each backend keeps its own profile in its own account storage.
//
// So a backend does not publish its whole profile blindly when it connects. It first asks the
// server what the account already shares (METADATA * GET) and publishes a field only when that
// cannot overwrite something another session set:
//   - the server has no value for it (a fresh client, or the field was never shared), or
//   - the server still has the value this backend last sent or found already shared for it,
//     so the difference is our own edit made while IRC was unreachable.
//
// What was last sent or found is kept per account and network in account storage, so it holds
// across reconnects, network edits and backend restarts. Any other value was set elsewhere and
// is left alone until the user edits that field here. An edit publishes only the fields it
// changed.

// profileCheckTimeout bounds how long a connect-time check waits for the server's answers; a
// variable for tests.
var profileCheckTimeout = 10 * time.Second

// profileValues are the values our profile shares, by metadata key.
func profileValues(p Member) map[string]string {
	// Only an explicit profile name is shared; a nickname or real-name fallback is already
	// visible over IRC, so clear any earlier display-name instead.
	name := ""
	if p.NameSource == "profile" {
		name = p.DisplayName
	}
	// Receivers only accept HTTPS avatars. A development instance's http:// upload URL would
	// replace a usable avatar with one nobody shows, so it is shared as no avatar.
	avatar := p.AvatarURL
	if !strings.HasPrefix(avatar, "https://") {
		avatar = ""
	}
	// IRC metadata is a single line. Keep the original bio in account storage.
	bio := strings.ReplaceAll(p.Bio, "\n", " ")
	for len(bio) > 300 {
		_, size := utf8.DecodeLastRuneInString(bio)
		bio = bio[:len(bio)-size]
	}
	values := richProfileValues(p)
	values["display-name"], values["avatar"], values["color"], values["bio"] = name, avatar, p.NameColor, bio
	for key, value := range values {
		if value == "" {
			delete(values, key)
		}
	}
	return values
}

// IRC permits at most 15 parameters per line. Keep key lists well below the
// line bound as well, including on servers with a modest message limit.
func metadataKeyEvents(command string, keys []string) []*girc.Event {
	var events []*girc.Event
	for len(keys) > 0 {
		n := min(len(keys), 8)
		events = append(events, &girc.Event{Command: "METADATA", Params: append([]string{"*", command}, keys[:n]...)})
		keys = keys[n:]
	}
	return events
}

func profileWireLength(key, value string) int {
	return len("METADATA * SET ") + len(key) + len(" :") + len(value)
}

// Called with w.mu held before persistence or upload. Offline values remain
// valid, but a connected network with a smaller limit rejects a changed value.
func (w *workspace) validateProfileWireBudget(network string, before, after Member) error {
	c := w.connections[network]
	if c == nil {
		return nil
	}
	c.mu.Lock()
	capable, limit := c.caps["draft/metadata-3"], c.lineBytes
	c.mu.Unlock()
	if !capable {
		return nil
	}
	if limit <= 0 {
		limit = safeMessageLineBytes(girc.DefaultMaxLineLength - girc.DefaultMaxPrefixLength)
	}
	values := profileValues(after)
	for _, key := range changedProfileKeys(before, after) {
		if profileWireLength(key, values[key]) > limit {
			return fmt.Errorf("profile value exceeds this network's metadata line limit")
		}
	}
	return nil
}

// changedProfileKeys lists the metadata keys whose shared value differs between two profiles.
func changedProfileKeys(before, after Member) []string {
	a, b := profileValues(before), profileValues(after)
	var keys []string
	for _, key := range metadataKeys {
		if a[key] != b[key] {
			keys = append(keys, key)
		}
	}
	return keys
}

// pushProfileKeys publishes the given fields of our profile; an empty value removes the key.
// The value counts as published once the writer has sent it (recordSentProfile).
func (c *connection) pushProfileKeys(p Member, keys []string) {
	if len(keys) == 0 || !c.has("draft/metadata-3") {
		return
	}
	values := profileValues(p)
	c.mu.Lock()
	limit := c.lineBytes
	c.mu.Unlock()
	if limit <= 0 {
		limit = safeMessageLineBytes(girc.DefaultMaxLineLength - girc.DefaultMaxPrefixLength)
	}
	for _, key := range keys {
		// Do not let girc split a metadata value into different wire lines.
		if profileWireLength(key, values[key]) > limit {
			slog.Warn("Profile value exceeds the network metadata line limit", "network", c.config.ID, "key", key)
			continue
		}
		params := []string{"*", "SET", key}
		if values[key] != "" {
			params = append(params, values[key])
		}
		_ = c.enqueue(&girc.Event{Command: "METADATA", Params: params})
	}
}

// recordSentProfile is called by the writer for every line it sent. A queued SET that was
// dropped on a disconnect is never recorded, so the next connect can still publish it.
func (c *connection) recordSentProfile(e *girc.Event) {
	if e.Command != "METADATA" || len(e.Params) < 3 || e.Params[0] != "*" || e.Params[1] != "SET" || !isMetadataKey(e.Params[2]) {
		return
	}
	value := ""
	if len(e.Params) >= 4 {
		value = e.Params[3]
	}
	c.recordPublished(map[string]string{e.Params[2]: value})
}

// publishedProfile returns what this account last published on this network, loading it from
// account storage on first use.
func (c *connection) publishedProfile() map[string]string {
	c.profileSave.Lock()
	defer c.profileSave.Unlock()
	return maps.Clone(c.loadPublishedLocked())
}

// loadPublishedLocked is called with c.profileSave held.
func (c *connection) loadPublishedLocked() map[string]string {
	c.mu.Lock()
	loaded := c.profilePublished
	c.mu.Unlock()
	if loaded != nil {
		return loaded
	}
	values := map[string]string{}
	if c.owner != nil && c.owner.store != nil {
		stored, err := c.owner.store.ProfilePublished(c.owner.ownerID, c.config.ID)
		if err != nil {
			slog.Warn("Loading the published profile failed", "error", err)
		} else {
			values = stored
		}
	}
	c.mu.Lock()
	c.profilePublished = values
	c.mu.Unlock()
	return values
}

// recordPublished stores values the server now holds because this backend sent or found them.
func (c *connection) recordPublished(values map[string]string) {
	if len(values) == 0 {
		return
	}
	c.profileSave.Lock()
	defer c.profileSave.Unlock()
	current := c.loadPublishedLocked()
	next := maps.Clone(current)
	maps.Copy(next, values)
	if maps.Equal(current, next) {
		return
	}
	c.mu.Lock()
	c.profilePublished = next
	c.mu.Unlock()
	if c.owner != nil && c.owner.store != nil {
		if err := c.owner.store.SaveProfilePublished(c.owner.ownerID, c.config.ID, next); err != nil {
			slog.Warn("Saving the published profile failed", "error", err)
		}
	}
}

// checkPublishedProfile asks the server for the values our account already shares. The profile
// is published once all of them are answered; without every answer within profileCheckTimeout
// nothing is published on this connection until the next edit. It takes only c.mu.
func (c *connection) checkPublishedProfile() {
	c.mu.Lock()
	c.profileCheck = map[string]string{}
	c.profileCheckDone = false
	c.profileCheckGen++
	gen := c.profileCheckGen
	c.mu.Unlock()
	if err := c.enqueue(metadataKeyEvents("GET", metadataKeys)...); err != nil {
		c.mu.Lock()
		c.profileCheck = nil
		c.mu.Unlock()
		return
	}
	time.AfterFunc(profileCheckTimeout, func() {
		c.mu.Lock()
		expired := c.profileCheck != nil && c.profileCheckGen == gen
		if expired {
			c.profileCheck = nil
			c.profileCheckDone = true
		}
		c.mu.Unlock()
		if expired {
			slog.Warn("The server did not answer the profile check; the profile is published on its next edit", "network", c.config.ID)
		}
	})
}

// profileCheckReply records an answer to checkPublishedProfile: RPL_KEYVALUE (761 <client>
// <target> <key> <visibility> :<value>) or RPL_KEYNOTSET (766 <client> <target> <key>).
// Unsolicited notifications of our own values carry "*" as <client> and are not answers.
func (c *connection) profileCheckReply(e girc.Event) {
	if len(e.Params) < 3 || e.Params[0] == "*" {
		return
	}
	value := ""
	switch {
	case e.Command == "761" && len(e.Params) >= 5:
		value = e.Params[4]
	case e.Command == "766":
	default:
		return
	}
	target, key := e.Params[1], e.Params[2]
	if c.fold(target) != c.fold(c.currentNick()) || !isMetadataKey(key) {
		return
	}
	c.mu.Lock()
	if c.profileCheck == nil {
		c.mu.Unlock()
		return
	}
	c.profileCheck[key] = value
	if len(c.profileCheck) < len(metadataKeys) {
		c.mu.Unlock()
		return
	}
	upstream := c.profileCheck
	c.profileCheck = nil
	c.mu.Unlock()

	published := c.publishedProfile()
	matched := map[string]string{}
	w := c.owner
	// Holding the workspace mutex orders this publish against a concurrent edit.
	w.mu.Lock()
	if _, ok := w.networks[c.config.ID]; !ok || w.deleted {
		w.mu.Unlock()
		return
	}
	self := w.networkSelf(c.config.ID)
	before := self
	ours := profileValues(self)
	imports := maps.Clone(w.profileImports[c.config.ID])
	importsChanged := false
	releaseImport := func(key string) {
		delete(imports, key)
		importsChanged = true
	}
	var keys []string
	for _, key := range metadataKeys {
		switch {
		case ours[key] == upstream[key]:
			// Already shared: remember it, so a later offline edit of this field is ours.
			matched[key] = ours[key]
			candidate := self
			if imports[key] && ours[key] != "" && applyProfileMetadata(&candidate, key, ours[key]) {
				releaseImport(key)
			}
		case imports[key]:
			// A legacy copy alone is not publication intent. A valid server value or
			// matching network-specific history establishes that this field was shared here.
			if upstream[key] != "" && applyProfileMetadata(&self, key, upstream[key]) {
				matched[key] = upstream[key]
				releaseImport(key)
			} else if upstream[key] == "" && ours[key] != "" && published[key] == ours[key] {
				candidate := self
				if applyProfileMetadata(&candidate, key, ours[key]) {
					releaseImport(key)
					if shouldPublish(ours[key], upstream[key], published, key) {
						keys = append(keys, key)
					}
				}
			}
		case shouldPublish(ours[key], upstream[key], published, key):
			keys = append(keys, key)
		default:
			// Another client changed this field. Learn it on this server only.
			if applyProfileMetadata(&self, key, upstream[key]) {
				matched[key] = upstream[key]
			}
		}
	}
	if !equal(before, self) {
		w.saveNetworkSelf(c.config.ID, self)
		if before.StatusExpiresAt != nil || self.StatusExpiresAt != nil {
			w.wakeProfileExpiry()
		}
	}
	// Persist an adopted value before releasing its import flag, so an interrupted
	// save cannot cause the old legacy value to be published on reconnect.
	if w.storageErr == nil && importsChanged {
		if w.store != nil {
			if err := w.store.SaveProfileImports(w.ownerID, c.config.ID, imports); err != nil {
				w.storageErr = err
			}
		}
		if w.storageErr == nil {
			w.profileImports[c.config.ID] = imports
		}
	}
	if w.storageErr == nil {
		c.pushProfileKeys(self, keys)
		c.recordPublished(matched)
	}
	w.mu.Unlock()
	c.mu.Lock()
	c.profileCheckDone = true
	c.mu.Unlock()
}

// shouldPublish decides whether connecting publishes one field: only when that cannot replace a
// value another session of the account shared.
func shouldPublish(ours, upstream string, published map[string]string, key string) bool {
	if ours == upstream {
		return false
	}
	if upstream == "" {
		return true
	}
	last, known := published[key]
	return known && last == upstream
}

func isMetadataKey(key string) bool {
	for _, k := range metadataKeys {
		if k == key {
			return true
		}
	}
	return false
}
