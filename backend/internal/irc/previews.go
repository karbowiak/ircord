package irc

import (
	"context"
	"fmt"
	"ircord/backend/internal/activity"
	"ircord/backend/internal/model"
	"ircord/backend/internal/msgindex"
	"ircord/backend/internal/notify"
	"ircord/backend/internal/push"
	"strings"
	"time"
)

type previewJob struct {
	workspace                  *workspace
	conversation, message, url string
}

// linkURL and outsideCode are the link rules shared with the message index
// (msgindex), so a previewed link and a catalogued one are found the same way.
var linkURL = msgindex.LinkURL

func outsideCode(text string) string { return msgindex.OutsideCode(text) }

// Called with the workspace lock held. Fetching runs outside every owner lock.
func (m *Manager) schedulePreview(w *workspace, conv Conversation, msg Message) {
	if m.options.Previews == nil || conv.PreviewMode != "server" || msg.Preview != nil || msg.Deleted || msg.GIF != nil || msg.Attachment != nil || msg.Kind != "message" {
		return
	}
	raw := msgindex.PreviewTarget(msg.Text)
	if raw == "" {
		return
	}
	select {
	case m.previewJobs <- previewJob{w, conv.ID, msg.ID, raw}:
	default:
	}
}
func (m *Manager) previewWorker() {
	defer m.wg.Done()
	for {
		select {
		case <-m.ctx.Done():
			return
		case job := <-m.previewJobs:
			if m.ctx.Err() != nil {
				return
			}
			w := job.workspace
			w.mu.Lock()
			valid := !w.deleted && w.conversations[job.conversation].PreviewMode == "server"
			w.mu.Unlock()
			if !valid {
				continue
			}
			value, err := m.options.Previews.Fetch(m.ctx, job.url)
			w.mu.Lock()
			if w.deleted || w.conversations[job.conversation].PreviewMode != "server" {
				w.mu.Unlock()
				continue
			}
			var msg Message
			found := false
			for _, item := range w.messages[job.conversation] {
				if item.ID == job.message {
					msg = item
					found = true
					break
				}
			}
			if !found && w.store != nil {
				msg, errRead := w.store.Message(w.ownerID, job.conversation, job.message)
				if errRead == nil {
					m.applyPreview(w, msg, value, err)
				}
				w.mu.Unlock()
				continue
			}
			if found {
				m.applyPreview(w, msg, value, err)
			}
			w.mu.Unlock()
		}
	}
}
func (m *Manager) applyPreview(w *workspace, msg Message, value model.LinkPreview, fetchError error) {
	if msg.Deleted || msg.Preview != nil {
		return
	}
	msg.PreviewSource = "client"
	if fetchError == nil {
		msg.Preview = &value
		msg.PreviewSource = "server"
	}
	msg = m.prepareMedia(w, msg)
	for i, item := range w.messages[msg.ConversationID] {
		if item.ID == msg.ID {
			w.messages[msg.ConversationID][i] = msg
		}
	}
	w.emit("message.updated", msg)
}

// CheckPreviewMode reports whether a personal preview mode may be set on a
// conversation, so a request can be refused before any of it is applied.
func (m *Manager) CheckPreviewMode(owner, conversation string) error {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	conv, ok := w.conversations[conversation]
	if !ok || w.deleted {
		return ErrNotFound
	}
	if conv.Kind == "channel" {
		return ErrChannelSetting
	}
	return nil
}
func (m *Manager) SetPreviewMode(owner, conversation, mode string) (Conversation, error) {
	if mode != "client" && mode != "server" {
		return Conversation{}, fmt.Errorf("preview mode must be client or server")
	}
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	conv, ok := w.conversations[conversation]
	if !ok || w.deleted {
		return Conversation{}, ErrNotFound
	}
	if conv.Kind == "channel" {
		return Conversation{}, ErrChannelSetting
	}
	conv.PreviewMode = mode
	w.conversations[conv.ID] = conv
	w.emit("conversation.updated", conv)
	if w.storageErr != nil {
		return Conversation{}, ErrStorage
	}
	return conv, nil
}
func (m *Manager) LinkPreview(ctx context.Context, owner, conversation, url string) (model.PreviewResult, error) {
	w := m.space(owner)
	w.mu.Lock()
	conv, ok := w.conversations[conversation]
	w.mu.Unlock()
	if !ok {
		return model.PreviewResult{}, ErrNotFound
	}
	if conv.PreviewMode != "server" || m.options.Previews == nil {
		return model.PreviewResult{Mode: "client"}, nil
	}
	value, err := m.options.Previews.Fetch(ctx, url)
	if err != nil {
		return model.PreviewResult{Mode: "client", Reason: err.Error()}, nil
	}
	return model.PreviewResult{Mode: "server", Preview: &value}, nil
}

func (m *Manager) notifyMessage(w *workspace, conv Conversation, msg Message, sender Member, nick string) {
	shared := ""
	if conv.ChannelSettings != nil {
		shared = conv.ChannelSettings.Notifications
	}
	if m.options.Push == nil && m.options.Router == nil || msg.Historical {
		return
	}
	kind := w.policy.Notification(conv.ID, conv.Kind, shared, msg.Kind, msg.Text, nick, sender.Nick, sender.Username, sender.Host, sender.ID == w.self.ID, time.Now())
	if kind == "" {
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
