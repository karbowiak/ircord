package irc

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/mediafetch"
	"ircord/backend/internal/model"
)

const (
	attachmentTag           = "+ircord/attachment"
	attachmentsTag          = "+ircord/attachments"
	maxAttachmentTagBytes   = 1800
	maxAttachmentsTagBytes  = 4000
	maxAttachmentsJSONBytes = 3000
	// Stay within girc's tag serializer limit, including its conservative
	// separator accounting; it otherwise silently drops overflowing tags.
	maxSendTagBytes    = 4094
	maxReceiveTagBytes = 8190
)

func validAttachment(a model.Attachment) bool {
	u, err := url.Parse(a.URL)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && clean(a.Name, 200) && len(a.URL) < 2048 && a.Width >= 0 && a.Height >= 0 && a.Width <= 16384 && a.Height <= 16384 && a.Size >= 0 && a.Size <= 100<<20
}

func (m *Manager) validGalleryAttachment(a model.Attachment) bool {
	u, err := url.Parse(a.URL)
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") ||
		len(a.URL) >= 2048 || strings.IndexFunc(a.URL, unicode.IsSpace) >= 0 ||
		!clean(a.Name, 200) || !clean(a.Alt, 200) || !clean(a.ContentType, 200) ||
		!utf8.ValidString(a.URL) || !utf8.ValidString(a.Name) || !utf8.ValidString(a.Alt) || !utf8.ValidString(a.ContentType) ||
		a.Width < 0 || a.Height < 0 || a.Width > 16384 || a.Height > 16384 || a.Size < 0 || a.Size > 100<<20 {
		return false
	}
	if u.Scheme == "https" {
		if _, err := mediafetch.URL(a.URL); err == nil {
			return true
		}
	}
	return m.localAttachmentURL(a.URL, u)
}

// localAttachmentURL allows HTTP development origins and configured private
// file origins only for an exact, live capability minted by this file service.
// This permits display metadata, not upload ownership or message access.
func (m *Manager) localAttachmentURL(raw string, u *url.URL) bool {
	if m.options.Files == nil {
		return false
	}
	start := strings.LastIndex(u.Path, "/files/")
	if start < 0 {
		return false
	}
	parts := strings.SplitN(u.Path[start+len("/files/"):], "/", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" {
		return false
	}
	record, err := m.options.Files.Record("", parts[0])
	// Network IDs belong to each account's workspace. Recipients on the same
	// IRC network have different IDs, so this read-only capability check does
	// not compare them. Sending still authenticates every ID through Attachment.
	return err == nil && record.Backend != "upstream" && record.URL == raw
}

func attachmentTextMatches(attachments []model.Attachment, text string) bool {
	if len(attachments) == 1 {
		return text == attachments[0].URL
	}
	if strings.IndexFunc(text, func(r rune) bool { return unicode.IsSpace(r) && r != ' ' && r != '\n' }) >= 0 {
		return false
	}
	urls := strings.Fields(text)
	if len(urls) != len(attachments) {
		return false
	}
	for i, attachment := range attachments {
		if urls[i] != attachment.URL {
			return false
		}
	}
	return true
}

func (m *Manager) validAttachments(attachments []model.Attachment, text string) bool {
	if len(attachments) < 2 || len(attachments) > model.MaxAttachments {
		return false
	}
	seen := make(map[string]bool, len(attachments))
	for _, attachment := range attachments {
		if !m.validGalleryAttachment(attachment) || seen[attachment.URL] {
			return false
		}
		seen[attachment.URL] = true
	}
	return attachmentTextMatches(attachments, text)
}

// tagBytes deliberately does not call Tags.Len: girc's serializer truncates
// large maps before reporting their size. Count one spare separator per map.
func tagBytes(tags girc.Tags) int {
	n := 1
	for key, value := range tags {
		n += len(key) + 1
		if value != "" {
			n += 1 + len(value)
		}
	}
	return n
}

// Incoming tag values have already been unescaped by girc. Include their wire
// escapes when enforcing the receive and client-only tag budgets.
func receivedTagBytes(tags girc.Tags, clientOnly bool) int {
	n := 1
	for key, value := range tags {
		if clientOnly && !strings.HasPrefix(key, "+") {
			continue
		}
		n += len(key) + 1
		if value != "" {
			n += 1 + len(value)
			for _, r := range value {
				if strings.ContainsRune("; \\\r\n", r) {
					n++
				}
			}
		}
	}
	return n
}

// readAttachments accepts display metadata only. Upload authority always comes
// from Files.Attachment with the requesting account and conversation network.
func (c *connection) readAttachments(msg *Message, tags girc.Tags) {
	if receivedTagBytes(tags, false) > maxReceiveTagBytes {
		return
	}
	single, hasSingle := tags[attachmentTag]
	plural, hasPlural := tags[attachmentsTag]
	if hasSingle && hasPlural {
		return
	}
	if hasPlural {
		if receivedTagBytes(tags, true) > maxSendTagBytes {
			return
		}
		if len(plural) == 0 || len(plural) > maxAttachmentsTagBytes {
			return
		}
		raw, err := base64.RawURLEncoding.Strict().DecodeString(plural)
		if err != nil || len(raw) > maxAttachmentsJSONBytes || !utf8.Valid(raw) || base64.RawURLEncoding.EncodeToString(raw) != plural {
			return
		}
		var attachments []model.Attachment
		if json.Unmarshal(raw, &attachments) == nil && c.manager.validAttachments(attachments, msg.Text) {
			msg.SetAttachments(attachments)
		}
		return
	}
	if hasSingle && len(single) > 0 && len(single) <= maxAttachmentTagBytes {
		if raw, err := base64.RawURLEncoding.DecodeString(single); err == nil && len(raw) <= maxAttachmentTagBytes*3/4 {
			var attachment model.Attachment
			if json.Unmarshal(raw, &attachment) == nil && attachment.URL == msg.Text && validAttachment(attachment) {
				msg.SetAttachments([]model.Attachment{attachment})
			}
		}
	}
}

// uploadAttachments resolves the entire batch before any command is persisted
// or queued. A missing or foreign upload must not produce a partial send.
func (m *Manager) uploadAttachments(owner, network string, input SendInput) ([]model.Attachment, error) {
	if len(input.Attachments) == 0 {
		return nil, nil
	}
	if len(input.Attachments) > model.MaxAttachments {
		return nil, fmt.Errorf("a message can contain at most %d uploads", model.MaxAttachments)
	}
	if m.options.Files == nil {
		return nil, ErrUnsupported
	}
	if input.GIF != nil {
		return nil, fmt.Errorf("uploads and a GIF cannot share a message")
	}
	attachments := make([]model.Attachment, 0, len(input.Attachments))
	ids, urls := map[string]bool{}, map[string]bool{}
	for _, id := range input.Attachments {
		if id == "" || ids[id] {
			return nil, fmt.Errorf("each upload must appear once in a message")
		}
		ids[id] = true
		attachment, err := m.options.Files.Attachment(owner, network, id)
		if err != nil || attachment == nil {
			return nil, fmt.Errorf("upload is unavailable in this conversation")
		}
		if urls[attachment.URL] {
			return nil, fmt.Errorf("each upload URL must appear once in a message")
		}
		urls[attachment.URL] = true
		attachments = append(attachments, *attachment)
	}
	if !attachmentTextMatches(attachments, input.Text) {
		return nil, fmt.Errorf("message text must contain only the upload URLs in order, separated by spaces or newlines")
	}
	if len(attachments) > 1 && !m.validAttachments(attachments, input.Text) {
		return nil, fmt.Errorf("upload metadata cannot be sent as a gallery")
	}
	return attachments, nil
}

func addAttachmentTags(tags girc.Tags, attachments []model.Attachment, policy string) error {
	if len(attachments) == 0 {
		return nil
	}
	if len(attachments) == 1 {
		// Keep the legacy single-file transport unchanged on networks that
		// cannot carry this optional display tag.
		if tags != nil && tagAllowed(policy, attachmentTag[1:]) {
			raw, _ := json.Marshal(attachments[0])
			encoded := base64.RawURLEncoding.EncodeToString(raw)
			if len(encoded) <= maxAttachmentTagBytes {
				tags[attachmentTag] = encoded
			}
		}
		return nil
	}
	if tags == nil || !tagAllowed(policy, attachmentsTag[1:]) {
		return fmt.Errorf("%w: this network does not support upload galleries", ErrUnsupported)
	}
	raw, err := json.Marshal(attachments)
	if err != nil || len(raw) > maxAttachmentsJSONBytes {
		return fmt.Errorf("upload gallery metadata exceeds the network tag limit; send a smaller batch")
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	if len(encoded) > maxAttachmentsTagBytes {
		return fmt.Errorf("upload gallery metadata exceeds the network tag limit; send a smaller batch")
	}
	tags[attachmentsTag] = encoded
	if tagBytes(tags) > maxSendTagBytes {
		return fmt.Errorf("upload gallery metadata and reply exceed the network tag limit; send a smaller batch")
	}
	return nil
}
