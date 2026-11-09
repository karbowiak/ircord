package irc

import "sort"

// Snapshot returns the current complete account view and its durable cursor.
func (m *Manager) Snapshot(owner string) Snapshot {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.snapshot()
}

// snapshot is called with the account lock held, including during exports.
func (w *workspace) snapshot() Snapshot {
	s := Snapshot{Version: 1, Demo: false, User: w.self, Members: []Member{}, Networks: []Network{}, Conversations: []Conversation{}, Preferences: w.preferences, Cursor: w.seq}
	for _, v := range w.invitations {
		if !v.Rejected {
			s.Invitations = append(s.Invitations, v)
		}
	}
	sort.Slice(s.Invitations, func(i, j int) bool { return s.Invitations[i].CreatedAt.Before(s.Invitations[j].CreatedAt) })
	for _, v := range w.members {
		s.Members = append(s.Members, v)
	}
	for _, v := range w.networks {
		s.Networks = append(s.Networks, v)
	}
	for _, v := range w.conversations {
		s.Conversations = append(s.Conversations, v)
	}
	sort.Slice(s.Networks, func(i, j int) bool { return s.Networks[i].ID < s.Networks[j].ID })
	sort.Slice(s.Conversations, func(i, j int) bool { return s.Conversations[i].ID < s.Conversations[j].ID })
	return clone(s)
}

// Cursor reads the account's committed position without constructing a snapshot.
func (m *Manager) Cursor(owner string) uint64 {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.seq
}
