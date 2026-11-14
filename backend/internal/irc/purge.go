package irc

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"
)

// Bounds for deleting one person's recent messages in a channel. Ergo keeps one
// week of history by default, so older messages can't be deleted for everyone.
const (
	PurgeMaxAge      = 7 * 24 * time.Hour
	PurgeMaxMessages = 500
	// purgeMaxPages bounds how far back the stored copy is loaded for one request.
	purgeMaxPages = 100
	// purgeKeep is how long a finished job can still be read.
	purgeKeep = time.Hour
)

// PurgeJob reports a background deletion of one person's recent messages.
type PurgeJob struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversationId"`
	Channel        string    `json:"channel"`
	Nick           string    `json:"nick"`
	Authority      string    `json:"authority" enum:"operator,owner"`
	State          string    `json:"state" enum:"running,done"`
	Total          int       `json:"total" doc:"Messages selected for deletion."`
	Deleted        int       `json:"deleted" doc:"Messages deleted so far, including ones someone else deleted meanwhile."`
	TooOld         int       `json:"tooOld" doc:"Messages the network no longer has, usually because they are older than its history."`
	Failed         int       `json:"failed" doc:"Messages the network refused to delete for another reason or did not confirm."`
	StartedAt      time.Time `json:"startedAt"`
	owner          string
	finished       time.Time
}

// ErrPurgeRunning refuses a second deletion job for the same channel.
var ErrPurgeRunning = errors.New("messages in this channel are already being deleted; wait until that finishes")

type purgeJobs struct {
	mu   sync.Mutex
	jobs map[string]*PurgeJob
}

// PurgeSelection is the preview of a deletion of one person's recent messages.
type PurgeSelection struct {
	Channel   string
	Nick      string
	Authority string
	Messages  []Message
}

// moderationAuthority reports how owner may delete other people's messages in
// conv: as a channel operator, as a server owner, or not at all. Callers hold w.mu.
func (m *Manager) moderationAuthority(owner string, c *connection, conv Conversation, network Network, self string) string {
	if conv.Kind != "channel" || conv.Party != nil {
		return ""
	}
	if conv.Joined && channelAt(conv, network.ISupport, self, 'o') {
		return AuthorityOperator
	}
	if c.home() && m.moderationNetwork() == conv.NetworkID && m.isServerOwner(owner) {
		return AuthorityOwner
	}
	return ""
}

// RecentMessages selects the messages memberID sent in a channel during the last
// age, newest first, after loading the stored copy back to the start of that
// range. Messages are matched by the member's account when it is known, so a
// nickname change does not hide them. At most PurgeMaxMessages are returned.
func (m *Manager) RecentMessages(ctx context.Context, owner, conversation, memberID string, age time.Duration) (PurgeSelection, error) {
	if age <= 0 || age > PurgeMaxAge {
		return PurgeSelection{}, errors.New("the time range must be at most 7 days")
	}
	if e := m.partyAccess(owner, conversation); e != nil {
		return PurgeSelection{}, e
	}
	w := m.space(owner)
	w.mu.Lock()
	conv, ok := w.conversations[conversation]
	c := w.connections[conv.NetworkID]
	if !ok || c == nil {
		w.mu.Unlock()
		return PurgeSelection{}, ErrNotFound
	}
	self := w.self.ID
	authority := m.moderationAuthority(owner, c, conv, w.networks[conv.NetworkID], self)
	member, known := w.members[memberID]
	w.mu.Unlock()
	if authority == "" {
		return PurgeSelection{}, ErrDeleteForbidden
	}
	if !known || memberID == self {
		return PurgeSelection{}, ErrNotFound
	}
	if !c.has("draft/message-redaction") {
		return PurgeSelection{}, ErrRedactUnsupported
	}
	account := member.Account
	if account == "*" {
		account = ""
	}
	matches := func(msg Message) bool {
		if msg.Deleted || !addressable(msg) || msg.Kind == "system" || msg.Kind == "event" {
			return false
		}
		if msg.SenderID == memberID {
			return true
		}
		return account != "" && strings.EqualFold(msg.SenderAccount, account)
	}
	cutoff := time.Now().Add(-age)
	out := PurgeSelection{Channel: c.target(conv), Nick: member.Nick, Authority: authority}
	seen := map[string]bool{}
	before := ""
	for range purgeMaxPages {
		page, err := m.HistoryContext(ctx, owner, conversation, before, 100)
		if err != nil {
			return PurgeSelection{}, err
		}
		older := false
		for _, msg := range slices.Backward(page.Items) {
			if msg.CreatedAt.Before(cutoff) {
				older = true
				continue
			}
			if !seen[msg.ID] && matches(msg) {
				seen[msg.ID] = true
				out.Messages = append(out.Messages, msg)
			}
		}
		if older || page.NextCursor == nil || len(page.Items) == 0 || len(out.Messages) >= PurgeMaxMessages {
			break
		}
		before = *page.NextCursor
	}
	slices.SortStableFunc(out.Messages, func(a, b Message) int { return b.CreatedAt.Compare(a.CreatedAt) })
	if len(out.Messages) > PurgeMaxMessages {
		out.Messages = out.Messages[:PurgeMaxMessages]
	}
	return out, nil
}

// StartPurge deletes the messages RecentMessages selects, one REDACT at a time,
// each confirmed by the network before the next. It returns at once; done runs
// once the job has finished. Only one job per conversation runs at a time.
func (m *Manager) StartPurge(ctx context.Context, owner, conversation, memberID string, age time.Duration, reason string, done func(PurgeJob)) (PurgeJob, error) {
	selection, err := m.RecentMessages(ctx, owner, conversation, memberID, age)
	if err != nil {
		return PurgeJob{}, err
	}
	job := &PurgeJob{ID: id(), ConversationID: conversation, Channel: selection.Channel, Nick: selection.Nick, Authority: selection.Authority, State: "running", Total: len(selection.Messages), StartedAt: time.Now().UTC(), owner: owner}
	m.purges.mu.Lock()
	if m.purges.jobs == nil {
		m.purges.jobs = map[string]*PurgeJob{}
	}
	for key, v := range m.purges.jobs {
		if v.State == "done" && time.Since(v.finished) > purgeKeep {
			delete(m.purges.jobs, key)
		}
		if v.State == "running" && v.owner == owner && v.ConversationID == conversation {
			m.purges.mu.Unlock()
			return PurgeJob{}, ErrPurgeRunning
		}
	}
	m.purges.jobs[job.ID] = job
	snapshot := *job
	m.purges.mu.Unlock()
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for _, msg := range selection.Messages {
			if m.ctx.Err() != nil {
				break
			}
			_, err := m.RedactAs(m.ctx, owner, conversation, msg.ID, reason)
			m.purges.mu.Lock()
			switch {
			case err == nil, errors.Is(err, ErrAlreadyDeleted):
				job.Deleted++
			case errors.Is(err, ErrRedactRefused) && tooOld(err):
				job.TooOld++
			default:
				job.Failed++
			}
			m.purges.mu.Unlock()
		}
		m.purges.mu.Lock()
		job.State, job.finished = "done", time.Now()
		result := *job
		m.purges.mu.Unlock()
		if done != nil {
			done(result)
		}
	}()
	return snapshot, nil
}

// Purge returns one of owner's deletion jobs.
func (m *Manager) Purge(owner, jobID string) (PurgeJob, error) {
	m.purges.mu.Lock()
	defer m.purges.mu.Unlock()
	job := m.purges.jobs[jobID]
	if job == nil || job.owner != owner {
		return PurgeJob{}, ErrNotFound
	}
	return *job, nil
}

// tooOld recognizes Ergo's refusal for a message it no longer keeps: "This
// message does not exist or is too old".
func tooOld(err error) bool {
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "too old") || strings.Contains(text, "does not exist") || strings.Contains(text, "unknown_msgid")
}
