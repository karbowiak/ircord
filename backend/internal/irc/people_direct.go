package irc

import (
	"fmt"
	"slices"
	"strings"
)

// DirectPerson distinguishes explicit nicknames from retained member IDs.
// Legacy nick strings remain accepted only when they do not name a known ID.
func (m *Manager) DirectPerson(owner, network, memberID, nickname string) (Conversation, error) {
	if (memberID == "") == (nickname == "") {
		return Conversation{}, fmt.Errorf("choose one member ID or nickname")
	}
	w := m.space(owner)
	w.mu.Lock()
	n, exists := w.networks[network]
	if !exists || w.deleted {
		w.mu.Unlock()
		return Conversation{}, ErrNotFound
	}
	if memberID != "" {
		member, known := w.members[memberID]
		switch {
		case known && memberID == w.self.ID:
			nickname = n.Nick
		case known:
			onNetwork := strings.HasPrefix(memberID, network+":")
			if !onNetwork && !strings.Contains(memberID, ":") {
				for _, conv := range w.conversations {
					if conv.NetworkID == network && slices.Contains(conv.MemberIDs, memberID) && (conv.Party == nil || m.partyAccess(owner, conv.ID) == nil) {
						onNetwork = true
						break
					}
				}
			}
			if !onNetwork {
				w.mu.Unlock()
				return Conversation{}, ErrNotFound
			}
			nickname = member.Nick
		default:
			nickname = memberID
		}
	}
	w.mu.Unlock()
	if err := validateNickname(nickname, 0); err != nil {
		return Conversation{}, err
	}
	return m.Direct(owner, network, nickname)
}
