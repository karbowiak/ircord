package linkmedia

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"ircord/backend/internal/mediafetch"
)

type YTDLPOptions struct {
	Binary   string
	Timeout  time.Duration
	MaxItems int
}
type YTDLP struct {
	options YTDLPOptions
	slots   chan struct{}
}

func NewYTDLP(o YTDLPOptions) *YTDLP {
	if o.Binary == "" {
		o.Binary = "yt-dlp"
	}
	if o.Timeout <= 0 || o.Timeout > 45*time.Second {
		o.Timeout = 45 * time.Second
	}
	if o.MaxItems <= 0 || o.MaxItems > 30 {
		o.MaxItems = 30
	}
	return &YTDLP{options: o, slots: make(chan struct{}, 2)}
}

// Resolve invokes yt-dlp in simulation mode. It extracts URLs and metadata,
// without downloading media, writing archives, loading plugins or user config.
func (y *YTDLP) Resolve(ctx context.Context, raw string) (Result, error) {
	canonical, handler, err := extractionTarget(raw)
	if err != nil {
		return Result{}, err
	}
	provider := handler.name
	if !NativeSocialSource(raw, provider) {
		return Result{}, ErrUnsupported
	}
	select {
	case y.slots <- struct{}{}:
		defer func() { <-y.slots }()
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
	ctx, cancel := context.WithTimeout(ctx, y.options.Timeout)
	defer cancel()
	proxy, closeProxy, err := mediafetch.Proxy(ctx)
	if err != nil {
		return Result{}, err
	}
	defer closeProxy()
	dir, err := os.MkdirTemp("", "ircord-extract-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(dir)
	// Ask for one extra entry to detect oversized galleries instead of silently
	// treating a truncated playlist as the complete post.
	args := []string{
		"--ignore-config", "--no-plugin-dirs", "--no-cache-dir", "--no-exec",
		"--simulate", "--skip-download", "--dump-single-json", "--ignore-no-formats-error", "--no-warnings",
		"--socket-timeout", "8", "--retries", "0", "--extractor-retries", "0",
		"--playlist-end", strconv.Itoa(y.options.MaxItems + 1),
		// H.264 first: providers such as TikTok rank HEVC highest, which Firefox and
		// many Chrome installs cannot play.
		"--format", "best[protocol=https][vcodec~='^(avc|h264)'][acodec!=none]/best[protocol=https][vcodec!=none][acodec!=none]/best[protocol=http][vcodec!=none][acodec!=none]/best",
		"--use-extractors", handler.extractor, "--proxy", proxy, "--", canonical,
	}
	cmd := exec.CommandContext(ctx, y.options.Binary, args...)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "TMPDIR=" + dir, "XDG_CONFIG_HOME=" + dir, "XDG_CACHE_HOME=" + dir, "PYTHONNOUSERSITE=1", "PYTHONSAFEPATH=1", "LANG=C"}
	var output cappedBuffer
	output.limit = 8 << 20
	cmd.Stdout = &output
	// Inspect only known restriction signals. Never return or log raw diagnostics:
	// they can contain signed URLs, cookies or authentication instructions.
	diagnostics := cappedBuffer{limit: 16 << 10}
	cmd.Stderr = &diagnostics
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		if output.exceeded || diagnostics.exceeded {
			return Result{}, ErrLimit
		}
		if restriction := extractionRestriction(diagnostics.String()); restriction != nil {
			return Result{}, errors.Join(ErrUnavailable, restriction)
		}
		return Result{}, errors.Join(ErrUnavailable, errors.New("yt-dlp extraction failed"))
	}
	if output.exceeded || diagnostics.exceeded {
		return Result{}, ErrLimit
	}
	var info ytInfo
	if json.Unmarshal(output.Bytes(), &info) != nil {
		return Result{}, errors.Join(ErrUnavailable, errors.New("invalid yt-dlp JSON"))
	}
	_, _, requestedID, _ := socialSource(canonical)
	if info.AgeLimit >= 18 {
		return Result{}, ErrAgeRestricted
	}
	if info.ID != requestedID || !strings.EqualFold(info.ExtractorKey, handler.extractor) || info.WebpageURL != "" && !sameSocialPost(info.WebpageURL, provider, requestedID) || info.HasDRM {
		return Result{}, errors.Join(ErrUnavailable, errors.New("extractor post identity or eligibility mismatch"))
	}
	if info.Availability != "" && info.Availability != "public" && info.Availability != "unlisted" {
		return Result{}, ErrRestricted
	}
	result := Result{URL: canonical, Provider: provider, ID: info.ID, Title: info.Title, Description: info.Description, Author: info.Uploader}
	if info.WebpageURL != "" {
		if _, p, err := extractionURL(info.WebpageURL); err == nil && p == provider {
			result.URL = info.WebpageURL
		}
	}
	entries := []ytInfo{info}
	if info.Entries != nil {
		entries = info.Entries
	}
	if len(entries) > y.options.MaxItems {
		return Result{}, ErrLimit
	}
	if len(entries) == 0 || info.PlaylistCount > 0 && info.PlaylistCount != len(entries) || info.NEntries > 0 && info.NEntries != len(entries) {
		return Result{}, errors.Join(ErrUnavailable, errors.New("incomplete extractor collection"))
	}
	for _, entry := range entries {
		if entry.HasDRM || entry.Entries != nil || entry.ExtractorKey != "" && !strings.EqualFold(entry.ExtractorKey, handler.extractor) || entry.WebpageURL != "" && !sameSocialPost(entry.WebpageURL, provider, requestedID) {
			return Result{}, ErrUnavailable
		}
		asset, err := entry.asset(handler.photoFallback)
		if err != nil {
			return Result{}, err
		} // don't silently return an incomplete gallery
		result.Assets = append(result.Assets, asset)
	}
	if handler.finishExtract != nil {
		handler.finishExtract(&result)
	}
	if provider == "instagram" && info.Entries != nil {
		// yt-dlp's older GraphSidecar path yields videos only and can omit
		// photos. A collection containing concrete photos came from its full
		// product-media path; an all-video collection needs the public embed
		// parser to establish that no unseen photo entries were discarded.
		hasPhoto := false
		for _, asset := range result.Assets {
			hasPhoto = hasPhoto || asset.Kind == "image"
		}
		if !hasPhoto {
			return Result{}, errors.Join(ErrUnavailable, errors.New("unverified Instagram collection completeness"))
		}
	}
	if err := validateSocialResult(result, provider, requestedID); err != nil {
		return Result{}, err
	}
	return result, nil
}

func extractionRestriction(diagnostic string) error {
	diagnostic = strings.ToLower(diagnostic)
	for _, signal := range []string{"age-restricted", "age restricted", "confirm your age"} {
		if strings.Contains(diagnostic, signal) {
			return ErrAgeRestricted
		}
	}
	for _, signal := range []string{"empty media response", "login required", "login_required", "requires login", "log in", "logged-in", "sign in", "private post"} {
		if strings.Contains(diagnostic, signal) {
			return ErrRestricted
		}
	}
	return nil
}

type ytInfo struct {
	ID            string            `json:"id"`
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	Uploader      string            `json:"uploader"`
	WebpageURL    string            `json:"webpage_url"`
	URL           string            `json:"url"`
	Ext           string            `json:"ext"`
	Protocol      string            `json:"protocol"`
	VCodec        string            `json:"vcodec"`
	ACodec        string            `json:"acodec"`
	Width         int               `json:"width"`
	Height        int               `json:"height"`
	Duration      *float64          `json:"duration"`
	ExtractorKey  string            `json:"extractor_key"`
	HasDRM        bool              `json:"_has_drm"`
	AgeLimit      int               `json:"age_limit"`
	Availability  string            `json:"availability"`
	PlaylistCount int               `json:"playlist_count"`
	NEntries      int               `json:"n_entries"`
	Thumbnails    []ytImage         `json:"thumbnails"`
	Thumbnail     string            `json:"thumbnail"`
	Headers       map[string]string `json:"http_headers"`
	Cookies       string            `json:"cookies"`
	Entries       []ytInfo          `json:"entries"`
	Formats       []ytFormat        `json:"formats"`
}
type ytFormat struct {
	URL      string            `json:"url"`
	Ext      string            `json:"ext"`
	Protocol string            `json:"protocol"`
	VCodec   string            `json:"vcodec"`
	ACodec   string            `json:"acodec"`
	Width    int               `json:"width"`
	Height   int               `json:"height"`
	Headers  map[string]string `json:"http_headers"`
	Cookies  string            `json:"cookies"`
}

type ytImage struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func (i ytInfo) asset(photoFallback func(ytInfo) (Asset, error)) (Asset, error) {
	selected := ytFormat{URL: i.URL, Ext: i.Ext, Protocol: i.Protocol, VCodec: i.VCodec, ACodec: i.ACodec, Width: i.Width, Height: i.Height, Headers: i.Headers, Cookies: i.Cookies}
	if !playable(selected) {
		selected = ytFormat{}
	}
	for _, f := range i.Formats {
		if playable(f) && (selected.URL == "" || formatPriority(f) > formatPriority(selected) || formatPriority(f) == formatPriority(selected) && int64(f.Width)*int64(f.Height) > int64(selected.Width)*int64(selected.Height)) {
			selected = f
		}
	}
	if selected.URL == "" {
		if photoFallback != nil {
			return photoFallback(i)
		}
		return Asset{}, ErrUnavailable
	}
	contentType := mime.TypeByExtension("." + selected.Ext)
	contentType = strings.Split(contentType, ";")[0]
	kind := strings.Split(contentType, "/")[0]
	if kind != "image" && kind != "video" && kind != "audio" {
		return Asset{}, ErrUnavailable
	}
	duration := float64(0)
	if i.Duration != nil {
		duration = *i.Duration
	}
	asset := Asset{URL: selected.URL, Kind: kind, ContentType: contentType, Width: selected.Width, Height: selected.Height, Duration: duration, Headers: http.Header{}}
	for key, value := range i.Headers {
		copyMediaHeader(asset.Headers, key, value)
	}
	for key, value := range selected.Headers {
		copyMediaHeader(asset.Headers, key, value)
	}
	if strings.EqualFold(i.ExtractorKey, "TikTok") {
		jar := selected.Cookies
		if jar == "" {
			jar = i.Cookies
		}
		if cookie := cookieHeader(jar, selected.URL); cookie != "" {
			u, _ := mediafetch.URL(selected.URL)
			asset.Headers.Set("Cookie", cookie)
			asset.anonymousCookieOrigin = u.Scheme + "://" + u.Host
		}
	}
	return asset, nil
}

// cookieHeader turns yt-dlp's cookie field ("name=value; Domain=.x; Path=/;
// Secure; name2=…") into a Cookie header with the cookies valid for the media
// host. Providers such as TikTok refuse their media without them.
func cookieHeader(jar, raw string) string {
	u, err := mediafetch.URL(raw)
	if jar == "" || len(jar) > 8192 || err != nil || u.Scheme != "https" || u.Hostname() != "tiktok.com" && !strings.HasSuffix(u.Hostname(), ".tiktok.com") {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	var pairs []string
	pending := ""
	flush := func(domain string) {
		if pending == "" {
			return
		}
		domain = strings.TrimPrefix(strings.ToLower(domain), ".")
		name, _, _ := strings.Cut(pending, "=")
		if domain == "tiktok.com" && (host == domain || strings.HasSuffix(host, "."+domain)) && anonymousTikTokCookie(name) && !strings.ContainsAny(pending, "\x00\r\n") && len(pairs) < 8 {
			pairs = append(pairs, pending)
		}
		pending = ""
	}
	domain := ""
	for _, part := range strings.Split(jar, ";") {
		name, value, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch strings.ToLower(name) {
		case "domain":
			domain = value
		case "path", "secure", "expires", "max-age", "httponly", "samesite", "":
		default:
			flush(domain)
			// yt-dlp quotes values containing "=" (base64 tokens); servers expect
			// the value as they set it, and TikTok rejects the quoted form.
			if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
				value = value[1 : len(value)-1]
			}
			pending, domain = name+"="+value, ""
		}
	}
	flush(domain)
	return strings.Join(pairs, "; ")
}

func anonymousTikTokCookie(name string) bool {
	return name == "ttwid" || name == "tt_chain_token" || name == "msToken" || name == "odin_tt"
}

func formatPriority(f ytFormat) int {
	switch strings.Split(mime.TypeByExtension("."+f.Ext), "/")[0] {
	case "video":
		if strings.HasPrefix(strings.ToLower(f.VCodec), "avc") || strings.HasPrefix(strings.ToLower(f.VCodec), "h264") {
			return 3
		}
		return 2
	case "audio":
		return 1
	}
	return 0
}

func playable(f ytFormat) bool {
	if _, err := mediafetch.URL(f.URL); err != nil {
		return false
	}
	if f.Protocol != "" && f.Protocol != "https" && f.Protocol != "http" {
		return false
	}
	if f.VCodec != "none" && f.ACodec == "none" && !strings.HasPrefix(mime.TypeByExtension("."+f.Ext), "image/") {
		return false
	}
	return true
}

func copyMediaHeader(headers http.Header, key, value string) {
	switch http.CanonicalHeaderKey(key) {
	case "User-Agent", "Referer", "Origin":
		if !strings.ContainsAny(value, "\r\n") {
			headers.Set(key, value)
		}
	}
}

type cappedBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		b.exceeded = true
		return 0, ErrLimit
	}
	return b.Buffer.Write(p)
}
