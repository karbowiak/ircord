package irc

func (m *Manager) FilehostConfig(owner, network string) (host, advertised, user, password string, err error) {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	cfg, ok := w.configs[network]
	if !ok {
		err = ErrNotFound
		return
	}
	host = cfg.Host
	advertised = w.info[network].FilehostURL
	user = cfg.SASLUser
	password = cfg.SASLPassword
	return
}
