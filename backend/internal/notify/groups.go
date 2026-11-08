package notify

import (
	"regexp"
	"slices"
	"strings"
)

// Group mentions are an IRCord convention, not an IRC feature: the text goes
// out exactly as typed, and every IRCord account decides from its own view of
// the channel whether "@here" or "@operator" includes it. Other clients just
// see the word.

// Recipient is this account as a message's channel sees it.
type Recipient struct {
	Nick string
	// Channel is false in direct messages, where group mentions never apply.
	Channel bool
	// Away is the account's own away state on IRC.
	Away bool
	// Rank is the account's highest channel status: "owner", "operator",
	// "halfop", "voiced" or "".
	Rank string
	// GroupMentions is the channel's setting for @here, @channel and
	// @everyone: "all", "ops" (also when empty) or "off".
	GroupMentions string
	// SenderOperator is whether the sender holds operator status or higher in
	// the channel, as this account sees it.
	SenderOperator bool
}

// Ranks from highest to lowest. A role mention includes the ranks above it:
// "@voiced" reaches everyone who can speak in a moderated channel, and
// "@operator" also reaches owners and admins.
var ranks = []string{"owner", "admin", "operator", "halfop", "voiced"}

var rankOfMode = map[byte]string{'q': "owner", 'a': "admin", 'o': "operator", 'h': "halfop", 'v': "voiced"}

// RankFor maps channel prefixes ("@", "~+") to the highest named rank, using
// the network's PREFIX (default "(qaohv)~&@%+").
func RankFor(prefixes, isupportPrefix string) string {
	modes, symbols := "qaohv", "~&@%+"
	if strings.HasPrefix(isupportPrefix, "(") {
		if a, b, ok := strings.Cut(isupportPrefix[1:], ")"); ok && len(a) == len(b) && len(a) > 0 {
			modes, symbols = a, b
		}
	}
	best := ""
	bestRank := len(ranks)
	for _, own := range prefixes {
		i := strings.IndexRune(symbols, own)
		if i < 0 {
			continue
		}
		name := rankOfMode[modes[i]]
		if r := slices.Index(ranks, name); r >= 0 && r < bestRank {
			best, bestRank = name, r
		}
	}
	return best
}

var groupMention = regexp.MustCompile(`(?i)@(here|channel|everyone|owner|operator|oper|voiced)`)

func wordChar(b byte) bool {
	return b == '_' || b == '-' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// Groups lists the group mentions in text, lower-cased, "@oper" as "operator".
// A token is a whole word: "@heresy" and "mail@here" are not mentions.
func Groups(text string) []string {
	text = Plain(text)
	result := []string{}
	for _, m := range groupMention.FindAllStringSubmatchIndex(text, -1) {
		if m[0] > 0 && wordChar(text[m[0]-1]) || m[1] < len(text) && wordChar(text[m[1]]) {
			continue
		}
		name := strings.ToLower(text[m[2]:m[3]])
		if name == "oper" {
			name = "operator"
		}
		result = append(result, name)
	}
	return result
}

// GroupMentioned reports whether a group mention in text includes r.
func GroupMentioned(text string, r Recipient) bool {
	if !r.Channel {
		return false
	}
	at := func(want string) bool {
		for _, v := range ranks {
			if v == r.Rank {
				return true
			}
			if v == want {
				return false
			}
		}
		return false
	}
	// Channel-wide mentions follow the channel's setting; role mentions are open to all.
	wide := r.GroupMentions == "all" || r.GroupMentions != "off" && r.SenderOperator
	for _, group := range Groups(text) {
		switch group {
		case "everyone":
			if wide {
				return true
			}
		case "here", "channel":
			if wide && !r.Away {
				return true
			}
		case "owner", "operator", "voiced":
			if at(group) {
				return true
			}
		}
	}
	return false
}

// Mentioned is Highlight plus group mentions: the rule for a personal mention.
func (p *Policy) Mentioned(text string, r Recipient) bool {
	return p.Highlight(text, r.Nick) || GroupMentioned(text, r)
}
