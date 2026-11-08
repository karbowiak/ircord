package irc

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"path"
	"strings"
	"time"

	"ircord/backend/internal/mediacdn"
	"ircord/backend/internal/model"
	"ircord/backend/internal/preview"
)

type mediaJob struct {
	workspace                          *workspace
	conversation, message, source, key string
	remove                             bool
}

func mediaSources(msg Message) []model.HostedMedia {
	out := []model.HostedMedia{}
	seen := map[string]bool{}
	add := func(raw, kind string) {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.User != nil || u.Host == "" || len(raw) > 2048 || seen[raw] || len(out) >= 30 {
			return
		}
		seen[raw] = true
		out = append(out, model.HostedMedia{Source: raw, Status: "loading", Kind: kind})
	}
	for _, raw := range linkURL.FindAllString(outsideCode(msg.Text), -1) {
		raw = strings.TrimRight(raw, ".,;!)]}\"'")
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		if video, ok := preview.ParseYouTube(u); ok {
			add("https://i.ytimg.com/vi/"+video.ID+"/hqdefault.jpg", "image")
		}
		switch strings.ToLower(path.Ext(u.Path)) {
		case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".apng", ".avif":
			add(raw, "image")
		case ".mp4", ".webm", ".m4v", ".mov":
			add(raw, "video")
		}
	}
	if g := msg.GIF; g != nil {
		add(g.URL, "image")
		add(g.PreviewURL, "image")
		add(g.StillURL, "image")
	}
	if p := msg.Preview; p != nil {
		add(p.Image, "image")
		for _, i := range p.Images {
			add(i.URL, "image")
		}
		if p.Media != nil {
			add(p.Media.URL, "image")
		}
	}
	return out
}

// prepareMedia runs under the workspace lock and never fetches remote bytes.
// The initial canonical message is persisted with loading states before workers
// can publish private message.updated events.
func (m *Manager) prepareMedia(w *workspace, msg Message) Message {
	if m.options.Media == nil || msg.Deleted || (msg.Kind != "message" && msg.Kind != "action") {
		return msg
	}
	old := map[string]model.HostedMedia{}
	for _, v := range msg.Media {
		old[v.Source] = v
	}
	msg.Media = mediaSources(msg)
	for i, item := range msg.Media {
		if prev, ok := old[item.Source]; ok && (prev.Status == "ready" || prev.Status == "removed") {
			if _, err := m.options.Media.Lookup(m.ctx, item.Source); errors.Is(err, mediacdn.ErrRemoved) {
				prev.Status = "removed"
				prev.URL = ""
				prev.Reason = "This media was removed."
			}
			msg.Media[i] = prev
			continue
		}
		key := w.ownerID + "\x00" + msg.ConversationID + "\x00" + msg.ID + "\x00" + item.Source
		m.mediaMu.Lock()
		queued := m.mediaQueued[key]
		if !queued {
			m.mediaQueued[key] = true
		}
		m.mediaMu.Unlock()
		if queued {
			continue
		}
		if err := m.options.Media.Track(m.ctx, w.ownerID, msg.ConversationID, msg.ID, item.Source); err != nil {
			msg.Media[i].Status = "failed"
			msg.Media[i].Reason = "Media storage is unavailable."
			m.unqueueMedia(key)
			continue
		}
		select {
		case m.mediaJobs <- mediaJob{workspace: w, conversation: msg.ConversationID, message: msg.ID, source: item.Source, key: key}:
		default:
			msg.Media[i].Status = "failed"
			msg.Media[i].Reason = "Media processing is busy. Reload this conversation to retry."
			m.unqueueMedia(key)
		}
	}
	for i, old := range w.messages[msg.ConversationID] {
		if old.ID == msg.ID {
			old.Media = msg.Media
			w.messages[msg.ConversationID][i] = old
			break
		}
	}
	return msg
}
func (m *Manager) unqueueMedia(key string) {
	m.mediaMu.Lock()
	delete(m.mediaQueued, key)
	m.mediaMu.Unlock()
}
func (m *Manager) mediaWorker() {
	defer m.wg.Done()
	for {
		select {
		case <-m.ctx.Done():
			return
		case job := <-m.mediaJobs:
			if job.remove {
				ctx, cancel := context.WithTimeout(m.ctx, 20*time.Second)
				if err := m.options.Media.RemoveMessage(ctx, job.workspace.ownerID, job.conversation, job.message); err != nil && m.ctx.Err() == nil {
					slog.Warn("Media cleanup will retry through maintenance", "error", err)
				}
				cancel()
				continue
			}
			record, err := m.options.Media.Fetch(m.ctx, job.source)
			w := job.workspace
			w.mu.Lock()
			if !w.deleted {
				var msg Message
				found := false
				for _, v := range w.messages[job.conversation] {
					if v.ID == job.message {
						msg = clone(v)
						found = true
						break
					}
				}
				if !found && w.store != nil {
					v, e := w.store.Message(w.ownerID, job.conversation, job.message)
					if e == nil {
						msg = v
						found = true
					}
				}
				if found && !msg.Deleted {
					changed := false
					for i, item := range msg.Media {
						if item.Source != job.source {
							continue
						}
						changed = true
						if err != nil {
							item.Status = "failed"
							item.Reason = "Media could not be stored."
							if errors.Is(err, mediacdn.ErrRemoved) {
								item.Status = "removed"
								item.Reason = "This media was removed."
							} else if errors.Is(err, mediacdn.ErrLimit) {
								item.Reason = "The media storage limit has been reached."
							}
							item.URL = ""
						} else {
							item.Status = "ready"
							item.URL = record.URL
							item.Hash = record.Hash
							item.ContentType = record.ContentType
							item.Size = record.Size
							item.Width = record.Width
							item.Height = record.Height
							item.Animated = record.Animated
							if strings.HasPrefix(record.ContentType, "video/") {
								item.Kind = "video"
							} else {
								item.Kind = "image"
							}
						}
						msg.Media[i] = item
					}
					if changed {
						for i, v := range w.messages[job.conversation] {
							if v.ID == msg.ID {
								w.messages[job.conversation][i] = msg
							}
						}
						w.emit("message.updated", msg)
					}
				}
			}
			w.mu.Unlock()
			m.unqueueMedia(job.key)
		}
	}
}
func (m *Manager) mediaMaintenance() {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
		if err := m.options.Media.Sweep(ctx); err != nil && m.ctx.Err() == nil {
			slog.Warn("Media maintenance failed", "error", err)
		}
		cancel()
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (m *Manager) removeMessageMedia(w *workspace, msg Message) {
	if m.options.Media == nil {
		return
	}
	select {
	case m.mediaJobs <- mediaJob{workspace: w, conversation: msg.ConversationID, message: msg.ID, remove: true}:
	default: /* retention maintenance eventually reclaims expired objects */
	}
}

// MediaRemoved refreshes each affected account's own private timeline. Historical
// pages consult the same durable removal state when they are loaded later.
func (m *Manager) MediaRemoved(hash string) {
	m.mu.Lock()
	owners := make([]*workspace, 0, len(m.owners))
	for _, w := range m.owners {
		owners = append(owners, w)
	}
	m.mu.Unlock()
	for _, w := range owners {
		w.mu.Lock()
		for cid, items := range w.messages {
			for index, msg := range items {
				changed := false
				msg = clone(msg)
				for i, item := range msg.Media {
					if item.Hash == hash {
						item.Status = "removed"
						item.URL = ""
						item.Reason = "This media was removed."
						msg.Media[i] = item
						changed = true
					}
				}
				if changed {
					w.messages[cid][index] = msg
					w.emit("message.updated", msg)
				}
			}
		}
		w.mu.Unlock()
	}
}
