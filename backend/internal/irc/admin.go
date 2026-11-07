package irc

// ConnectedCount snapshots only operational state. Copy workspace pointers before
// locking individual workspaces to preserve the manager/workspace lock ordering.
func (m *Manager) ConnectedCount() int {
	m.mu.Lock()
	spaces := make([]*workspace, 0, len(m.owners))
	for _, w := range m.owners {
		spaces = append(spaces, w)
	}
	m.mu.Unlock()
	n := 0
	for _, w := range spaces {
		w.mu.Lock()
		for _, info := range w.info {
			if info.State == "connected" {
				n++
			}
		}
		w.mu.Unlock()
	}
	return n
}
