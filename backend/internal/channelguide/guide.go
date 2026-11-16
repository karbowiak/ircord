// Package channelguide holds the server's channel guide (#258): the owner-curated list of home
// network channels that new members pick from when they join, and that everyone can browse
// later. The guide only describes channels; changing it never joins or parts anyone.
package channelguide

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"ircord/backend/internal/model"
)

// Entry states.
const (
	// Everyone channels are joined when an account is created and are locked in the picker.
	// Members can still leave them later like any other channel.
	Everyone = "everyone"
	// Suggested channels start ticked in the picker.
	Suggested = "suggested"
	// Listed channels start unticked.
	Listed = "listed"
)

// Limits on what owners can save.
const (
	MaxGroups      = 12
	MaxEntries     = 50
	MaxEveryone    = 10
	MaxGroupName   = 40
	MaxDescription = 120
	MaxChannelName = 50
	MaxRequests    = 100
)

// ChannelGuideGroup is a heading in the guide. Groups are shown in the order they are saved.
type ChannelGuideGroup struct {
	Name string `json:"name" minLength:"1" maxLength:"40"`
}

// ChannelGuideEntry is one channel in the guide. Entries are shown in the order they are
// saved, under their group; an empty group lists the channel under the guide's closing
// "More channels".
type ChannelGuideEntry struct {
	Channel     string `json:"channel" minLength:"2" maxLength:"64" doc:"Channel name, including its # prefix"`
	Group       string `json:"group,omitempty" maxLength:"40"`
	Description string `json:"description,omitempty" maxLength:"120"`
	State       string `json:"state" enum:"everyone,suggested,listed"`
}

// ChannelGuide is the whole document owners edit.
type ChannelGuide struct {
	Groups  []ChannelGuideGroup `json:"groups" nullable:"false" maxItems:"12"`
	Entries []ChannelGuideEntry `json:"entries" nullable:"false" maxItems:"50"`
}

// Short names for use in Go; the API schemas carry the longer ones.
type (
	Guide = ChannelGuide
	Group = ChannelGuideGroup
	Entry = ChannelGuideEntry
)

// Fold is the key a channel name is compared and stored by. It uses RFC 1459 casemapping, the
// IRC connections' default, so names the server treats as one channel ("#a[" and "#a{") are
// one guide entry. On an ascii-mapped server that only merges names differing in []\^ chars.
func Fold(channel string) string {
	return model.FoldIdentity(strings.TrimSpace(channel), "rfc1459")
}

// NormalChannel trims a channel name and adds a missing # prefix. It reports false for a name
// that can't be a public channel on the home network.
func NormalChannel(channel string) (string, bool) {
	channel = strings.TrimSpace(channel)
	if channel != "" && channel[0] != '#' {
		channel = "#" + channel
	}
	if len(channel) < 2 || utf8.RuneCountInString(channel) > MaxChannelName || strings.ContainsAny(channel, " ,:\x00\x07\r\n") || !utf8.ValidString(channel) {
		return "", false
	}
	return channel, true
}

// CleanText trims a free-text field and refuses control characters.
func CleanText(value string, limit int) (string, error) {
	value = strings.Join(strings.Fields(value), " ")
	if utf8.RuneCountInString(value) > limit {
		return "", fmt.Errorf("must be %d characters or fewer", limit)
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("can't contain control characters")
		}
	}
	return value, nil
}

// Normalize checks a guide an owner wants to save and returns its canonical form. Errors are
// sentences written for the owner.
func Normalize(in Guide) (Guide, error) {
	out := Guide{Groups: []Group{}, Entries: []Entry{}}
	if len(in.Groups) > MaxGroups {
		return out, fmt.Errorf("The guide can have up to %d groups.", MaxGroups)
	}
	if len(in.Entries) > MaxEntries {
		return out, fmt.Errorf("The guide can list up to %d channels.", MaxEntries)
	}
	groups := map[string]string{}
	for _, g := range in.Groups {
		name, err := CleanText(g.Name, MaxGroupName)
		if err != nil {
			return out, fmt.Errorf("Group names %s.", err)
		}
		if name == "" {
			return out, fmt.Errorf("Every group needs a name.")
		}
		key := strings.ToLower(name)
		if _, dup := groups[key]; dup {
			return out, fmt.Errorf("There are two groups called %q.", name)
		}
		groups[key] = name
		out.Groups = append(out.Groups, Group{Name: name})
	}
	seen := map[string]bool{}
	everyone := 0
	for _, e := range in.Entries {
		channel, ok := NormalChannel(e.Channel)
		if !ok {
			return out, fmt.Errorf("%q isn't a channel name.", strings.TrimSpace(e.Channel))
		}
		if seen[Fold(channel)] {
			return out, fmt.Errorf("%s is in the guide twice.", channel)
		}
		seen[Fold(channel)] = true
		group := ""
		if name, err := CleanText(e.Group, MaxGroupName); err != nil || name != "" {
			canonical, ok := groups[strings.ToLower(name)]
			if err != nil || !ok {
				return out, fmt.Errorf("%s is in a group that doesn't exist.", channel)
			}
			group = canonical
		}
		description, err := CleanText(e.Description, MaxDescription)
		if err != nil {
			return out, fmt.Errorf("The description of %s %s.", channel, err)
		}
		switch e.State {
		case Everyone:
			everyone++
		case Suggested, Listed:
		default:
			return out, fmt.Errorf("%s needs to be for everyone, suggested or listed.", channel)
		}
		out.Entries = append(out.Entries, Entry{Channel: channel, Group: group, Description: description, State: e.State})
	}
	if everyone > MaxEveryone {
		return out, fmt.Errorf("Up to %d channels can be for everyone.", MaxEveryone)
	}
	return out, nil
}

// Fallback is the guide in effect before an owner saves one: the configured default channels,
// each for everyone, so an existing install behaves as before.
func Fallback(defaults []string) Guide {
	g := Guide{Groups: []Group{}, Entries: []Entry{}}
	seen := map[string]bool{}
	for _, raw := range defaults {
		channel, ok := NormalChannel(raw)
		if !ok || seen[Fold(channel)] || len(g.Entries) >= MaxEntries {
			continue
		}
		seen[Fold(channel)] = true
		g.Entries = append(g.Entries, Entry{Channel: channel, State: Everyone})
	}
	return g
}

// EveryoneChannels lists the channels a new account joins.
func (g Guide) EveryoneChannels() []string {
	out := []string{}
	for _, e := range g.Entries {
		if e.State == Everyone {
			out = append(out, e.Channel)
		}
	}
	return out
}

// Find returns the entry for a channel.
func (g Guide) Find(channel string) (Entry, bool) {
	key := Fold(channel)
	for _, e := range g.Entries {
		if Fold(e.Channel) == key {
			return e, true
		}
	}
	return Entry{}, false
}
