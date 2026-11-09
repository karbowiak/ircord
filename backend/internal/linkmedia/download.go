package linkmedia

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"ircord/backend/internal/cdn"
	"ircord/backend/internal/mediafetch"
)

func (r *Resolver) probe(ctx context.Context, raw string) (string, int64, error) {
	asset, err := r.probeAsset(ctx, raw)
	return asset.ContentType, asset.Size, err
}

func (r *Resolver) probeAsset(ctx context.Context, raw string) (Asset, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return Asset{}, err
	}
	// 64 KiB holds image dimensions: GIF colour tables and JPEG SOF markers after EXIF.
	req.Header.Set("Range", "bytes=0-65535")
	req.Header.Set("Accept-Encoding", "identity")
	res, err := r.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Asset{}, ctx.Err()
		}
		return Asset{}, ErrUnavailable
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 206 {
		return Asset{}, &HTTPError{Provider: "media source", Status: res.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if err != nil || len(data) == 0 {
		return Asset{}, ErrUnavailable
	}
	contentType := mediaType(res.Header.Get("Content-Type"), data)
	if contentType == "" {
		return Asset{}, ErrUnavailable
	}
	size := res.ContentLength
	if res.StatusCode == 206 {
		size = 0
		if _, tail, ok := strings.Cut(res.Header.Get("Content-Range"), "/"); ok {
			size, _ = strconv.ParseInt(tail, 10, 64)
		}
	}
	kind, _, _ := strings.Cut(contentType, "/")
	asset := Asset{URL: raw, Kind: kind, ContentType: contentType, Size: size, SourceExpiresAt: sourceExpiry(raw)}
	if kind == "image" {
		if config, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
			asset.Width, asset.Height = config.Width, config.Height
		}
	}
	return asset, nil
}

func mediaType(header string, data []byte) string {
	sniffed := http.DetectContentType(data)
	// A login/challenge page is never a playable image or video, regardless of
	// the extension, provider metadata, or response Content-Type.
	if strings.HasPrefix(sniffed, "text/") {
		return ""
	}
	declared, _, _ := mime.ParseMediaType(header)
	for _, value := range []string{sniffed, declared} {
		if strings.HasPrefix(value, "image/") || strings.HasPrefix(value, "video/") || strings.HasPrefix(value, "audio/") {
			return value
		}
	}
	return ""
}

// Store copies a public source into the CDN with a nonrenewable 90-day expiry.
// Bytes stream through a bounded temporary file, never a complete video in memory.
func (r *Resolver) Store(ctx context.Context, asset Asset) (Asset, error) {
	if r.options.CDN == nil || r.persistent == nil {
		return Asset{}, errors.New("durable CDN cache not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, r.options.Timeout)
	defer cancel()
	if _, err := mediafetch.URL(asset.URL); err != nil {
		return Asset{}, err
	}
	// This also covers capability URLs returned by a native connector. Copying
	// their bytes would outlive the original upload's token and deletion checks.
	if fileCapabilityURL(asset.URL) {
		return Asset{}, ErrUnsupported
	}
	sourceDeadline := sourceExpiry(asset.URL)
	if !sourceDeadline.IsZero() && !sourceDeadline.After(time.Now()) {
		return Asset{}, ErrUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return Asset{}, err
	}
	for key, values := range asset.Headers {
		for _, value := range values {
			copyMediaHeader(req.Header, key, value)
		}
	}
	req.Header.Set("Accept-Encoding", "identity")
	// Do not leak source headers (including cookies) through a redirect.
	client := *r.http
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) > 3 {
			return errors.New("too many media redirects")
		}
		if _, err := mediafetch.URL(next.URL.String()); err != nil {
			return err
		}
		if fileCapabilityURL(next.URL.String()) {
			return ErrUnsupported
		}
		if next.URL.Scheme != via[0].URL.Scheme || !strings.EqualFold(next.URL.Host, via[0].URL.Host) {
			for key := range next.Header {
				if key != "Range" && key != "Accept-Encoding" {
					next.Header.Del(key)
				}
			}
		}
		return nil
	}
	res, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Asset{}, ctx.Err()
		}
		return Asset{}, ErrUnavailable
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return Asset{}, &HTTPError{Provider: "media source", Status: res.StatusCode}
	}
	if res.ContentLength > r.options.MaxFileBytes {
		return Asset{}, ErrLimit
	}
	file, err := os.CreateTemp("", "ircord-media-")
	if err != nil {
		return Asset{}, err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(res.Body, r.options.MaxFileBytes+1))
	if err != nil {
		return Asset{}, err
	}
	if size > r.options.MaxFileBytes {
		return Asset{}, ErrLimit
	}
	if size == 0 || res.ContentLength >= 0 && size != res.ContentLength {
		return Asset{}, ErrUnavailable
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return Asset{}, err
	}
	first := make([]byte, 512)
	n, _ := file.Read(first)
	contentType := mediaType(res.Header.Get("Content-Type"), first[:n])
	if contentType == "" || (asset.Kind != "" && !strings.HasPrefix(contentType, asset.Kind+"/")) {
		return Asset{}, ErrUnavailable
	}
	if strings.HasPrefix(contentType, "image/") && (asset.Width == 0 || asset.Height == 0) {
		if _, err = file.Seek(0, io.SeekStart); err != nil {
			return Asset{}, err
		}
		if config, _, err := image.DecodeConfig(io.LimitReader(file, 64<<10)); err == nil {
			asset.Width, asset.Height = config.Width, config.Height
		}
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return Asset{}, err
	}
	// A unique key gives this operation ownership of its upload. A failed
	// gallery can then clean it up without deleting another result's object.
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return Asset{}, err
	}
	key := "link-media/" + hex.EncodeToString(nonce[:])
	if r.options.CDN.URL(key) == "" {
		return Asset{}, errors.New("CDN public URL not configured")
	}
	expires := time.Now().Add(MediaCacheTTL)
	if err := r.reserveObject(ctx, key, expires); err != nil {
		return Asset{}, err
	}
	object, err := r.options.CDN.PutExpiring(ctx, key, file, size, contentType, expires)
	if err != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return Asset{}, errors.Join(err, r.abandonObject(cleanup, key))
	}
	asset = storedAsset(asset, object)
	asset.SHA256 = hex.EncodeToString(hash.Sum(nil))
	asset.ExpiresAt, asset.SourceExpiresAt = expires, sourceDeadline
	return asset, nil
}

func storedAsset(asset Asset, object cdn.Object) Asset {
	asset.SourceURL, asset.URL, asset.ObjectKey = asset.URL, object.URL, object.Key
	asset.Size, asset.ContentType = object.Size, object.ContentType
	asset.Kind = strings.Split(object.ContentType, "/")[0]
	asset.Headers = nil
	return asset
}
