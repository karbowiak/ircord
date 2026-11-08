package notify

import (
	"regexp"
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
	// Away is the account's own away state on the message's network.
	Away bool
	// Offline marks a message from history: one sent before this account was
	// there to see it (catch-up after a disconnect, scrollback). @here,
	// @channel and @everyone reach only who was present, so they skip it.
	Offline bool
	// Owner, Operator and Voiced are whether the account holds that channel
	// status or a higher one, by position in the network's PREFIX. A role
	// mention includes the ranks above it: "@voiced" reaches everyone who can
	// speak in a moderated channel, "@operator" also reaches admins and owners.
	Owner, Operator, Voiced bool
	// GroupMentions is the channel's setting for @here, @channel and
	// @everyone: "all", "ops" (also when empty) or "off".
	GroupMentions string
	// SenderOperator is whether the sender holds operator status or higher in
	// the channel, as this account sees it.
	SenderOperator bool
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
	// Channel-wide mentions follow the channel's setting and reach only who is
	// there; role mentions are open to everyone.
	wide := !r.Offline && (r.GroupMentions == "all" || r.GroupMentions != "off" && r.SenderOperator)
	for _, group := range Groups(text) {
		switch {
		case group == "everyone" && wide,
			(group == "here" || group == "channel") && wide && !r.Away,
			group == "owner" && r.Owner,
			group == "operator" && r.Operator,
			group == "voiced" && r.Voiced:
			return true
		}
	}
	return false
}

// Mentioned is Highlight plus group mentions: the rule for a personal mention.
func (p *Policy) Mentioned(text string, r Recipient) bool {
	return p.Highlight(text, r.Nick) || GroupMentioned(text, r)
}
