package irc

import (
	"ircord/backend/internal/notify"
	"ircord/backend/internal/push"
	"time"
)

func (m *Manager) notifyMessage(w *workspace, conv Conversation, msg Message, sender Member, nick string) {
	shared := ""
	if conv.ChannelSettings != nil {
		shared = conv.ChannelSettings.Notifications
	}
	if m.options.Push == nil || msg.Historical || w.policy.Notification(conv.ID, conv.Kind, shared, msg.Kind, msg.Text, nick, sender.Nick, sender.Username, sender.Host, sender.ID == w.self.ID, time.Now()) == "" {
		return
	}
	badge := 0
	for _, c := range w.conversations {
		if c.Kind == "direct" {
			badge += c.UnreadCount
		} else {
			badge += c.MentionCount
		}
	}
	title := sender.DisplayName
	if title == "" {
		title = sender.Nick
	}
	if conv.Kind == "channel" {
		title = sender.Nick + " in #" + conv.Name
	}
	body := []rune(notify.Plain(msg.Text))
	if len(body) > 512 {
		body = body[:512]
	}
	_ = m.options.Push.Notify(w.ownerID, push.Payload{Title: title, Body: string(body), ConversationID: conv.ID, Tag: conv.ID, Badge: badge})
}
