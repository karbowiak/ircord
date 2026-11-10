package model

import "strings"

// Person separates a nickname, a services account and an optional local account.
// IRC usernames (idents) and hosts are deliberately absent.
type Person struct {
	Key              string   `json:"key"`
	NetworkID        string   `json:"networkId"`
	Nickname         string   `json:"nickname,omitempty"`
	Nicknames        []string `json:"nicknames,omitempty"`
	ServicesAccount  string   `json:"servicesAccount,omitempty"`
	DisplayName      string   `json:"displayName"`
	AvatarURL        string   `json:"avatarUrl,omitempty"`
	MemberID         string   `json:"memberId,omitempty"`
	BackendAccountID string   `json:"backendAccountId,omitempty"`
	Presence         string   `json:"presence" enum:"online,away,offline,unknown"`
	Eligible         bool     `json:"eligible"`
	Reason           string   `json:"reason,omitempty" enum:"account_unknown,identity_unverified,local_account_required,nickname_unknown"`
}

type PeoplePage struct {
	Items  []Person `json:"items" nullable:"false"`
	Source string   `json:"source" enum:"known,registered" doc:"Known means people retained in this workspace, not a server-wide online roster."`
	Status string   `json:"status" enum:"ready,partial,unavailable"`
	More   bool     `json:"more"`
}

type PeopleQuery struct {
	NetworkID      string `query:"networkId" required:"true" minLength:"1" maxLength:"100"`
	Purpose        string `query:"purpose" required:"true" enum:"mention,command,mask,history,dm,services,party"`
	ConversationID string `query:"conversationId" maxLength:"200"`
	Query          string `query:"query" maxLength:"100"`
	Limit          int    `query:"limit" minimum:"1" maximum:"50" default:"20"`
}

// FoldIdentity applies only the requested server's nickname/account casemapping.
func FoldIdentity(value, mapping string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		if mapping != "ascii" {
			switch r {
			case '[':
				return '{'
			case ']':
				return '}'
			case '\\':
				return '|'
			case '^':
				if mapping != "strict-rfc1459" {
					return '~'
				}
			}
		}
		return r
	}, value)
}

// PersonMatchRank is shared by public lookup and private administrative search.
// Exact identities precede exact display names, then prefixes, then substrings.
func PersonMatchRank(query, mapping, displayName string, identities ...string) int {
	query = strings.TrimSpace(query)
	if query == "" {
		return 4
	}
	identityQuery, textQuery := FoldIdentity(query, mapping), strings.ToLower(query)
	best := 99
	for _, identity := range identities {
		if identity == "" {
			continue
		}
		folded := FoldIdentity(identity, mapping)
		switch {
		case folded == identityQuery:
			best = min(best, 0)
		case strings.HasPrefix(folded, identityQuery):
			best = min(best, 2)
		case strings.Contains(folded, identityQuery):
			best = min(best, 4)
		}
	}
	display := strings.ToLower(displayName)
	switch {
	case display == textQuery:
		best = min(best, 1)
	case strings.HasPrefix(display, textQuery):
		best = min(best, 3)
	case strings.Contains(display, textQuery):
		best = min(best, 4)
	}
	return best
}
