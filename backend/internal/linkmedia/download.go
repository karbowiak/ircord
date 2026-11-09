package linkmedia

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
	"ircord/backend/internal/cdn"
)

const sniffBytes = 64 << 10
const maxImageDimension = 8192
const maxImagePixels = 12_000_000

type fileFailure struct {
	reason string
	cause  error
}

func (e *fileFailure) Error() string              { return e.reason }
func (e *fileFailure) Unwrap() error              { return e.cause }
func rejectFile(reason string, cause error) error { return &fileFailure{reason: reason, cause: cause} }

func failureReason(err error) string {
	var failure *fileFailure
	if errors.As(err, &failure) {
		return failure.reason
	}
	if errors.Is(err, ErrRestricted) {
		return "restricted"
	}
	if errors.Is(err, ErrLimit) {
		return "too_large"
	}
	return "fetch_failed"
}

// mediaType uses observed signatures only. MIME identification is not malware
// scanning, and neither response headers nor filename extensions bless bytes.
func mediaType(_ string, data []byte) string { return mimetype.Detect(data).String() }

// probeAsset remains for inactive legacy fetchers. Native resolution uses one
// complete ingest and never probes and refetches the same source.
func (r *Resolver) probeAsset(ctx context.Context, raw string) (Asset, error) {
	res, err := r.fetch(ctx, raw, "*/*")
	if err != nil {
		return Asset{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Asset{}, ErrUnavailable
	}
	prefix, err := io.ReadAll(io.LimitReader(res.Body, sniffBytes))
	if err != nil || len(prefix) == 0 {
		return Asset{}, ErrUnavailable
	}
	ct := mediaType("", prefix)
	kind := allowedKind(ct)
	if kind == "" || kind == "file" {
		return Asset{}, ErrUnsupported
	}
	asset := Asset{URL: raw, Kind: kind, ContentType: ct, Size: -1}
	if config, _, err := image.DecodeConfig(bytes.NewReader(prefix)); err == nil && imageBounds(config) {
		asset.Width, asset.Height = config.Width, config.Height
	}
	return asset, nil
}

func (r *Resolver) probe(ctx context.Context, raw string) (string, int64, error) {
	a, err := r.probeAsset(ctx, raw)
	return a.ContentType, a.Size, err
}

// Store downloads once into bounded temporary storage, validates, hashes, and
// publishes. Failed downloads return only observed metadata, with no URL.
func (r *Resolver) Store(ctx context.Context, requested Asset) (Asset, error) {
	return r.ingest(ctx, requested, "")
}

func (r *Resolver) ingest(ctx context.Context, requested Asset, expected string) (Asset, error) {
	return r.ingestLimited(ctx, requested, expected, r.options.MaxFileBytes)
}

// A gallery shares one download budget. Limit each item by the bytes remaining
// without changing resolver options that are shared by concurrent requests.
func (r *Resolver) ingestLimited(ctx context.Context, requested Asset, expected string, limit int64) (Asset, error) {
	ctx, cancel := context.WithTimeout(ctx, r.options.Timeout)
	defer cancel()
	asset := Asset{Kind: "file", Size: -1}
	limit = min(limit, r.options.MaxFileBytes)
	if limit <= 0 {
		return asset, rejectFile("too_large", ErrLimit)
	}
	if fileCapabilityURL(requested.URL) {
		return asset, rejectFile("restricted", ErrRestricted)
	}
	if deadline := sourceExpiry(requested.URL); !deadline.IsZero() && !deadline.After(time.Now()) {
		return asset, rejectFile("fetch_failed", ErrUnavailable)
	}
	res, err := r.fetch(ctx, requested.URL, "*/*")
	if err != nil {
		return asset, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		if res.StatusCode == 401 || res.StatusCode == 403 {
			return asset, rejectFile("restricted", ErrRestricted)
		}
		return asset, ErrUnavailable
	}
	prefix, err := io.ReadAll(io.LimitReader(res.Body, min(sniffBytes, limit+1)))
	prefixSize := int64(len(prefix))
	if err != nil || len(prefix) == 0 {
		return asset, rejectFile("invalid_file", ErrUnavailable)
	}
	asset.ContentType = mediaType("", prefix)
	kind := allowedKind(asset.ContentType)
	if unsafeFileName(requested.URL) {
		return asset, rejectFile("unsupported_type", ErrUnsupported)
	}
	// CDNs can negotiate another safe raster format at a .jpeg URL. The
	// filename suggests a family; detected bytes decide the concrete format.
	if expected != "" && allowedKind(expected) != kind {
		return asset, rejectFile("type_mismatch", ErrUnsupported)
	}
	if kind == "" {
		return asset, rejectFile("unsupported_type", ErrUnsupported)
	}
	if requested.Kind != "" && requested.Kind != kind {
		return asset, rejectFile("type_mismatch", ErrUnsupported)
	}
	asset.Kind = kind
	if res.ContentLength > limit || prefixSize > limit {
		return asset, rejectFile("too_large", ErrLimit)
	}
	if kind == "image" {
		config, _, decodeErr := image.DecodeConfig(bytes.NewReader(prefix))
		if decodeErr != nil {
			return asset, rejectFile("invalid_file", ErrUnavailable)
		}
		if !imageBounds(config) {
			return asset, rejectFile("too_large", ErrLimit)
		}
		asset.Width, asset.Height = config.Width, config.Height
	}
	file, err := os.CreateTemp("", "ircord-native-media-")
	if err != nil {
		return asset, rejectFile("storage_unavailable", ErrUnavailable)
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err = file.Write(prefix); err != nil {
		return asset, rejectFile("storage_unavailable", ErrUnavailable)
	}
	n, err := io.Copy(file, io.LimitReader(res.Body, limit-prefixSize+1))
	completeSize := prefixSize + n
	if completeSize > limit {
		return asset, rejectFile("too_large", ErrLimit)
	}
	if err != nil {
		return asset, rejectFile("fetch_failed", ErrUnavailable)
	}
	if res.ContentLength >= 0 && completeSize != res.ContentLength {
		return asset, rejectFile("invalid_file", ErrUnavailable)
	}
	// A prefix or a truncated transfer cannot establish the whole-file size.
	asset.Size = completeSize
	if err = validateFile(file, &asset); err != nil {
		return asset, err
	}
	if r.options.CDN == nil || r.persistent == nil {
		return asset, rejectFile("storage_unavailable", ErrUnavailable)
	}
	// Hash only the fully validated file. No response body is fetched again.
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return asset, ErrUnavailable
	}
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return asset, ErrUnavailable
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return asset, ErrUnavailable
	}
	var nonce [32]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return asset, ErrUnavailable
	}
	key := "link-media/" + hex.EncodeToString(nonce[:])
	if r.options.CDN.URL(key) == "" {
		return asset, rejectFile("storage_unavailable", ErrUnavailable)
	}
	expires := time.Now().Add(MediaCacheTTL)
	if err = r.reserveObject(ctx, key, expires); err != nil {
		if errors.Is(err, ErrBusy) {
			return asset, err
		}
		return asset, rejectFile("storage_unavailable", ErrUnavailable)
	}
	var object cdn.Object
	if kind == "file" {
		object, err = r.options.CDN.PutExpiringAttachment(ctx, key, file, asset.Size, asset.ContentType, expires)
	} else {
		object, err = r.options.CDN.PutExpiring(ctx, key, file, asset.Size, asset.ContentType, expires)
	}
	if err != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = r.abandonObject(cleanup, key)
		return asset, rejectFile("storage_unavailable", ErrUnavailable)
	}
	asset.SourceURL = requested.URL
	asset.SourceExpiresAt = sourceExpiry(requested.URL)
	asset.URL, asset.ObjectKey = object.URL, object.Key
	asset.ExpiresAt = expires
	asset.SHA256 = hex.EncodeToString(hash.Sum(nil))
	return asset, nil
}

func allowedKind(ct string) string {
	ct, _, _ = mime.ParseMediaType(ct)
	switch ct {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/bmp":
		return "image"
	case "video/mp4", "video/quicktime", "video/webm", "video/ogg":
		return "video"
	case "audio/mpeg", "audio/mp3", "audio/mp4", "audio/x-m4a", "audio/wav", "audio/x-wav", "audio/flac", "audio/x-flac", "audio/ogg":
		return "audio"
	case "application/pdf", "application/msword", "application/vnd.ms-excel", "application/vnd.ms-powerpoint", "application/vnd.ms-publisher",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"application/vnd.oasis.opendocument.text", "application/vnd.oasis.opendocument.spreadsheet", "application/vnd.oasis.opendocument.presentation",
		"application/vnd.oasis.opendocument.graphics", "application/vnd.oasis.opendocument.formula", "application/vnd.oasis.opendocument.chart",
		"application/epub+zip", "text/rtf", "application/rtf",
		"text/plain", "text/csv", "text/tab-separated-values", "application/json", "application/x-ndjson",
		"application/zip", "application/gzip", "application/x-tar", "application/x-7z-compressed", "application/vnd.rar", "application/x-bzip2", "application/x-xz", "application/zstd":
		return "file"
	}
	return ""
}

func imageBounds(config image.Config) bool {
	return config.Width > 0 && config.Height > 0 && config.Width <= maxImageDimension && config.Height <= maxImageDimension && int64(config.Width)*int64(config.Height) <= maxImagePixels
}

func validateFile(file *os.File, asset *Asset) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return rejectFile("invalid_file", ErrUnavailable)
	}
	if asset.Kind == "image" {
		if asset.ContentType == "image/gif" {
			if err := validateGIF(file, asset.Size); err != nil {
				return err
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				return rejectFile("invalid_file", err)
			}
			if _, err := gif.DecodeAll(file); err != nil {
				return rejectFile("invalid_file", ErrUnavailable)
			}
		} else {
			img, _, err := image.Decode(file)
			if err != nil || img.Bounds().Dx() != asset.Width || img.Bounds().Dy() != asset.Height {
				return rejectFile("invalid_file", ErrUnavailable)
			}
		}
		return nil
	}
	// Nonimage media is identified from bytes, fully received under hard size
	// and time limits, and delivered only in native media elements. Documents,
	// text, and archives are attachment-only. No codecs run and no archives are
	// unpacked here; signature identification does not certify content safety.
	return nil
}

// storedAsset is retained for inactive connector source compatibility.
func storedAsset(asset Asset, object cdn.Object) Asset {
	asset.SourceURL, asset.URL, asset.ObjectKey = asset.URL, object.URL, object.Key
	asset.Size, asset.ContentType = object.Size, object.ContentType
	asset.Kind = strings.Split(object.ContentType, "/")[0]
	asset.Headers = nil
	return asset
}
