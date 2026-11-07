package irc

import (
	"context"
	"fmt"
	"ircord/backend/internal/model"
	"ircord/backend/internal/notify"
	"ircord/backend/internal/push"
	"regexp"
	"strings"
	"time"
)

type previewJob struct {
	workspace                  *workspace
	conversation, message, url string
}

var linkURL = regexp.MustCompile(`https://[^\s<>\x00-\x20]+`)

// Called with the workspace lock held. Fetching runs outside every owner lock.
func (m *Manager) schedulePreview(w *workspace, conv Conversation, msg Message) {
	if m.options.Previews == nil || conv.PreviewMode != "server" || msg.Preview != nil || msg.Deleted || msg.GIF != nil || msg.Attachment != nil || msg.Kind != "message" {
		return
	}
	raw := strings.TrimRight(linkURL.FindString(msg.Text), ".,;!)]}\"'")
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
	for i, item := range w.messages[msg.ConversationID] {
		if item.ID == msg.ID {
			w.messages[msg.ConversationID][i] = msg
		}
	}
	w.emit("message.updated", msg)
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
	if m.options.Push == nil || msg.Historical || w.policy.Notification(conv.ID, conv.Kind, msg.Kind, msg.Text, nick, sender.Nick, sender.Username, sender.Host, sender.ID == w.self.ID, time.Now()) == "" {
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
