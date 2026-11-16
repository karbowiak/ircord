package channelguide

import (
	"context"
	"encoding/json"

	"ircord/backend/internal/storage"
)

// Load returns the guide in effect: the saved one, or the fallback built from the configured
// default channels when no owner has saved one. saved is false for the fallback.
func Load(ctx context.Context, store *storage.Store, defaults []string) (guide Guide, stored storage.StoredChannelGuide, saved bool, err error) {
	if store == nil {
		return Fallback(defaults), stored, false, nil
	}
	stored, err = store.ChannelGuide(ctx)
	if err != nil {
		return Guide{}, stored, false, err
	}
	if stored.Revision == 0 {
		return Fallback(defaults), stored, false, nil
	}
	if err = json.Unmarshal(stored.Payload, &guide); err != nil {
		return Guide{}, stored, false, err
	}
	// A stored guide passed validation when it was saved; normalizing again only fills nil
	// lists. A document that no longer validates is served as it is rather than lost.
	if normal, e := Normalize(guide); e == nil {
		guide = normal
	}
	if guide.Groups == nil {
		guide.Groups = []Group{}
	}
	if guide.Entries == nil {
		guide.Entries = []Entry{}
	}
	return guide, stored, true, nil
}
