package preview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"ircord/backend/internal/model"
)

// YouTube links are resolved through YouTube's oEmbed endpoint instead of the watch page: it
// answers with a small JSON document (title, channel, thumbnail), reports videos that refuse
// embedding (401/403) and unavailable ones (400/404). Only the video ID is passed to clients,
// which build the youtube-nocookie.com player URL themselves.

var youtubeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// youtubeHosts are the hosts whose /watch, /shorts, /embed, /live and /v links name a video.
var youtubeHosts = []string{"youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com", "youtube-nocookie.com", "www.youtube-nocookie.com"}

// YouTubeVideo is a video a link names, after normalizing its URL variants.
type YouTubeVideo struct {
	ID    string
	Start int
	Short bool
}

// ParseYouTube recognizes the common YouTube video URL variants: watch links, youtu.be short
// links, Shorts, embeds and live links, with an optional t= or start= offset.
func ParseYouTube(u *url.URL) (YouTubeVideo, bool) {
	if u == nil || u.Scheme != "https" || u.User != nil {
		return YouTubeVideo{}, false
	}
	host := strings.ToLower(u.Hostname())
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	var video YouTubeVideo
	switch {
	case host == "youtu.be":
		video.ID = segments[0]
	case contains(youtubeHosts, host):
		switch {
		case len(segments) == 1 && segments[0] == "watch":
			video.ID = u.Query().Get("v")
		case len(segments) == 2 && (segments[0] == "embed" || segments[0] == "live" || segments[0] == "v"):
			video.ID = segments[1]
		case len(segments) == 2 && segments[0] == "shorts":
			video.ID, video.Short = segments[1], true
		}
	}
	if !youtubeIDPattern.MatchString(video.ID) {
		return YouTubeVideo{}, false
	}
	query := u.Query()
	offset := query.Get("t")
	if offset == "" {
		offset = query.Get("start")
	}
	video.Start = startOffset(offset)
	return video, true
}
func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

var offsetPattern = regexp.MustCompile(`^(?:(\d{1,3})h)?(?:(\d{1,4})m)?(?:(\d{1,6})s?)?$`)

// startOffset reads "90", "90s" or "1h2m3s"; anything else, or more than a day, is no offset.
func startOffset(value string) int {
	match := offsetPattern.FindStringSubmatch(value)
	if value == "" || match == nil {
		return 0
	}
	total := 0
	for i, scale := range []int{3600, 60, 1} {
		if match[i+1] != "" {
			n, _ := strconv.Atoi(match[i+1])
			total += n * scale
		}
	}
	if total > 86400 {
		return 0
	}
	return total
}

// maxOEmbed bounds the oEmbed document; real responses are under 2 KiB.
const maxOEmbed = 64 << 10

// youtube resolves a video's preview. The canonical watch URL keys the cache, so every variant
// of the same link is fetched once.
func (s *Service) youtube(ctx context.Context, raw string, video YouTubeVideo) (model.LinkPreview, error) {
	key := "youtube:" + video.ID
	s.mu.Lock()
	cached, ok := s.cache[key]
	s.mu.Unlock()
	if ok && cached.fresh() {
		value := cached.value
		value.URL = raw
		value.Embed = embedFor(video, value.Embed.Embeddable)
		return value, nil
	}
	watch := "https://www.youtube.com/watch?v=" + video.ID
	endpoint := "https://www.youtube.com/oembed?format=json&url=" + url.QueryEscape(watch)
	if _, err := s.validate(ctx, endpoint); err != nil {
		return model.LinkPreview{}, err
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "IRCord-LinkPreview/1")
	response, err := s.client.Do(req)
	if err != nil {
		return model.LinkPreview{}, errors.New("preview request failed")
	}
	defer response.Body.Close()
	value := model.LinkPreview{URL: raw, SiteName: "YouTube", Kind: "video", Title: "YouTube video", Image: thumbnail(video.ID)}
	switch response.StatusCode {
	case http.StatusOK:
		var document struct {
			Title  string `json:"title"`
			Author string `json:"author_name"`
		}
		if err := json.NewDecoder(io.LimitReader(response.Body, maxOEmbed)).Decode(&document); err != nil {
			return model.LinkPreview{}, errors.New("YouTube returned unreadable metadata")
		}
		if title := bounded(document.Title, 200); title != "" {
			value.Title = title
		}
		value.Author = bounded(document.Author, 100)
		value.Embed = embedFor(video, true)
	case http.StatusUnauthorized, http.StatusForbidden:
		// The video exists but its owner turned off embedding: it is still shown as a card that
		// links to YouTube, without a player.
		value.Embed = embedFor(video, false)
	case http.StatusBadRequest, http.StatusNotFound:
		return model.LinkPreview{}, errors.New("this YouTube video is unavailable")
	default:
		return model.LinkPreview{}, fmt.Errorf("preview returned HTTP %d", response.StatusCode)
	}
	s.store(key, value)
	return value, nil
}
func embedFor(video YouTubeVideo, embeddable bool) *model.PreviewEmbed {
	return &model.PreviewEmbed{Provider: "youtube", ID: video.ID, Start: video.Start, Short: video.Short, Embeddable: embeddable}
}

// thumbnail is YouTube's own image host; IDs are validated, so the URL cannot point elsewhere.
func thumbnail(id string) string { return "https://i.ytimg.com/vi/" + id + "/hqdefault.jpg" }
