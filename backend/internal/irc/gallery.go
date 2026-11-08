package irc

import (
	"errors"
	"fmt"
	"time"

	"ircord/backend/internal/storage"
)

// GalleryItem is one image or GIF posted in a conversation, with its source
// message. Items come from the account's own stored history only, so a gallery
// never shows more than the conversation's timeline could.
type GalleryItem struct {
	MessageID      string    `json:"messageId" doc:"Canonical IRCord message ID of the source post"`
	Index          int       `json:"index" doc:"Position of the image within its message"`
	ConversationID string    `json:"conversationId"`
	NetworkID      string    `json:"networkId"`
	SenderID       string    `json:"senderId"`
	CreatedAt      time.Time `json:"createdAt"`
	URL            string    `json:"url" doc:"The original link or upload URL; the gallery never stores a copy of the image"`
	ThumbURL       string    `json:"thumbUrl,omitempty" doc:"A smaller rendition the source names (a picked GIF's preview), for thumbnails; open url for the full image"`
	Kind           string    `json:"kind" enum:"image,gif"`
	Source         string    `json:"source" enum:"link,upload,gif,preview" doc:"Where the image came from: a direct link, an upload, the GIF picker, or a server link preview"`
	FileID         string    `json:"fileId,omitempty" doc:"The stored upload behind an uploaded image, set only for uploads on this server"`
	Width          int       `json:"width,omitempty"`
	Height         int       `json:"height,omitempty"`
	Excerpt        string    `json:"excerpt" doc:"The source message as plain text, at most 280 characters"`
	Truncated      bool      `json:"truncated,omitempty"`
}

type GalleryPage struct {
	Items      []GalleryItem `json:"items" nullable:"false"`
	NextCursor *string       `json:"nextCursor"`
	Total      int           `json:"total" doc:"Images in the conversation's stored history; counted for the first page only and 0 on later pages"`
	Indexing   bool          `json:"indexing" doc:"History stored before the gallery existed is still being indexed, so older images may be missing"`
}

// Gallery pages the images and GIFs posted in one of the account's
// conversations, newest post first. It reads the message index, which covers
// the history stored for this account; nothing is fetched to fill it.
func (m *Manager) Gallery(owner, conversation, cursor string, limit int) (GalleryPage, error) {
	if limit < 1 || limit > 100 {
		return GalleryPage{}, fmt.Errorf("limit must be between 1 and 100")
	}
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	conv, ok := w.conversations[conversation]
	if w.deleted || !ok {
		return GalleryPage{}, ErrNotFound
	}
	page := GalleryPage{Items: []GalleryItem{}}
	if w.store == nil {
		return page, nil
	}
	rows, more, err := w.store.Gallery(owner, conversation, cursor, limit)
	if errors.Is(err, storage.ErrInvalidCursor) {
		return page, err
	}
	if err != nil {
		return page, ErrStorage
	}
	for _, row := range rows {
		item := GalleryItem{MessageID: row.MessageID, Index: row.Index, ConversationID: conversation, NetworkID: conv.NetworkID, SenderID: row.SenderID, CreatedAt: row.CreatedAt, URL: row.URL, ThumbURL: row.ThumbURL, Kind: row.Kind, Source: row.Source, FileID: row.FileID, Width: row.Width, Height: row.Height}
		item.Excerpt, item.Truncated = excerpt(row.Text)
		page.Items = append(page.Items, item)
	}
	if more && len(rows) > 0 {
		next := storage.GalleryCursor(rows[len(rows)-1])
		page.NextCursor = &next
	}
	if cursor == "" {
		if page.Total, err = w.store.GalleryCount(owner, conversation); err != nil {
			return page, ErrStorage
		}
	}
	complete, err := w.store.LinkIndexComplete()
	if err != nil {
		return page, ErrStorage
	}
	page.Indexing = !complete
	return page, nil
}
