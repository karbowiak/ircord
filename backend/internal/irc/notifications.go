package irc

import (
	"ircord/backend/internal/activity"
	"ircord/backend/internal/notify"
	"ircord/backend/internal/push"
	"strings"
	"time"
)

func (m *Manager) notifyMessage(w *workspace, conv Conversation, msg Message, sender Member, nick string, replyToSelf ...bool) {
	if w.ownerID == partyControlOwner {
		return
	}
	shared := ""
	if conv.ChannelSettings != nil {
		shared = conv.ChannelSettings.Notifications
	}
	if m.options.Push == nil && m.options.Router == nil || msg.Historical || msg.Deleted || w.deleted || w.storageErr != nil {
		return
	}
	// This runs with the workspace lock after storing the live message. Check
	// the durable read boundary here; the router cannot call back into this
	// workspace while Notify holds that same lock.
	if w.store != nil {
		unread, err := w.store.NotificationMessageUnread(w.ownerID, conv.ID, msg.ID)
		if err != nil || !unread {
			return
		}
	} else {
		read, err := w.readTime(conv.ID)
		if err != nil || !msg.CreatedAt.After(read) {
			return
		}
	}
	kind := w.policy.Notification(conv.ID, conversationPolicyKind(conv), shared, msg.Kind, msg.Text, w.recipient(conv, nick, msg), sender.Nick, sender.Username, sender.Host, sender.ID == w.self.ID, time.Now(), replyToSelf...)
	if kind == "" {
		return
	}
	badge := 0
	for _, c := range w.conversations {
		if conversationPolicyKind(c) == "direct" {
			badge += c.UnreadCount
		} else {
			badge += c.MentionCount
		}
	}
	title := sender.DisplayName
	if title == "" {
		title = sender.Nick
	}
	if conv.Kind == "channel" && conv.Party == nil {
		title = sender.Nick + " in #" + conv.Name
	}
	text := notify.Plain(msg.Text)
	if msg.GIF != nil && strings.TrimSpace(text) == "" {
		text = "Sent a GIF"
	}
	body := []rune(text)
	if len(body) > 512 {
		body = body[:512]
	}
	if m.options.Router != nil {
		target, all := w.policy.Delays()
		m.options.Router.Notify(w.ownerID, activity.Item{Conversation: conv.ID, Message: msg.ID, At: msg.CreatedAt, Title: title, Body: string(body), Kind: kind, Badge: badge, SenderNick: sender.Nick, SenderUser: sender.Username, SenderHost: sender.Host}, activity.Delays{Target: target, All: all})
		return
	}
	_ = m.options.Push.Notify(w.ownerID, push.Payload{Title: title, Body: string(body), ConversationID: conv.ID, Tag: conv.ID, Badge: badge})
}

// recipient is this account as conv sees it, for group mentions such as
// "@here" or "@operator" in msg. Callers hold w.mu.
func (w *workspace) recipient(conv Conversation, nick string, msg Message) notify.Recipient {
	support := w.networks[conv.NetworkID].ISupport
	at := func(member string, mode byte) bool { return channelAt(conv, support, member, mode) }
	r := notify.Recipient{
		Nick:           nick,
		CaseMapping:    support["CASEMAPPING"],
		Channel:        conv.Kind == "channel" && conv.Party == nil,
		Away:           w.awayOn[conv.NetworkID],
		Offline:        msg.Historical,
		Owner:          at(w.self.ID, 'q'),
		Operator:       at(w.self.ID, 'o'),
		Voiced:         at(w.self.ID, 'v'),
		SenderOperator: at(msg.SenderID, 'o'),
	}
	if conv.ChannelSettings != nil {
		r.GroupMentions = conv.ChannelSettings.GroupMentions
	}
	return r
}
