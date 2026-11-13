package linkmedia

import (
	"context"
	"errors"
	"net/url"
	"path"
	"strings"
	"time"
)

func directMediaFile(u *url.URL) bool {
	ext := strings.ToLower(path.Ext(u.Path))
	// Page routes stay on discovery. Other explicit file extensions route to
	// validation, including documents, archives, text, and dangerous formats.
	switch ext {
	case "", ".html", ".htm", ".php", ".asp", ".aspx", ".jsp":
		return false
	}
	return len(ext) <= 16 && strings.Trim(ext[1:], "abcdefghijklmnopqrstuvwxyz0123456789") == ""
}

// DirectFileCandidate is only a routing hint, never evidence about file bytes.
func DirectFileCandidate(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (fileCapabilityURL(raw) || directMediaFile(u))
}

// An active-content filename is an additional conservative rejection hint.
// It cannot make unknown bytes eligible for publication.
func unsafeFileName(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return true
	}
	switch strings.ToLower(path.Ext(u.Path)) {
	case ".svg", ".html", ".htm", ".xhtml", ".js", ".mjs", ".cjs", ".sh", ".bash", ".zsh", ".ps1", ".bat", ".cmd", ".vbs", ".hta", ".jar", ".apk", ".msi", ".exe", ".dll", ".com", ".scr", ".wasm":
		return true
	}
	return false
}

// Minted IRCord file URLs carry deletion authority at their source. Recognize
// the exact route shape across origins without granting any destination access.
func fileCapabilityURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	parts := strings.Split(u.Path, "/")
	if len(parts) != 5 || parts[0] != "" || parts[1] != "files" || parts[4] == "" {
		return false
	}
	return len(parts[2]) == 32 && len(parts[3]) == 48 &&
		strings.Trim(parts[2], "0123456789abcdef") == "" && // gitleaks:allow -- hexadecimal validation alphabet
		strings.Trim(parts[3], "0123456789abcdef") == "" // gitleaks:allow -- hexadecimal validation alphabet
}

// File capabilities are never copied: their original delivery API owns account,
// token, expiry, and deletion checks. All other ready files require CDN ingest.
func (r *Resolver) resolveFile(ctx context.Context, raw string) (Result, error) {
	u, _ := url.Parse(raw)
	name := path.Base(u.Path)
	if name == "." || name == "/" || name == "" {
		name = u.Hostname()
	}
	name = cleanText(name, 240)
	asset, err := r.ingest(ctx, Asset{URL: raw}, extensionType(strings.ToLower(path.Ext(u.Path))))
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	if errors.Is(err, ErrBusy) {
		return Result{}, err
	}
	// The extension was only a hint. A web page gets a page card.
	if errors.Is(err, errHTMLDocument) {
		return r.resolvePage(ctx, raw)
	}
	if err == nil && asset.Kind != "file" {
		return Result{URL: raw, Provider: "media", Assets: []Asset{asset}}, nil
	}
	reason := ""
	if err != nil {
		reason = failureReason(err)
		asset.URL = ""
		asset.ObjectKey = ""
		asset.Headers = nil
	}
	until := asset.ExpiresAt
	if until.IsZero() {
		until = time.Now().Add(24 * time.Hour)
	}
	if reason == "fetch_failed" || reason == "storage_unavailable" {
		until = time.Now().Add(time.Minute)
	}
	return Result{URL: raw, Provider: "file", CacheUntil: until, Preview: &Preview{
		Kind: "file", Site: u.Hostname(), Title: name, Reason: reason, ExpiresAt: until,
		File: &FileInfo{Asset: asset, Name: name, Reason: reason},
	}}, nil
}

func extensionType(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".pdf":
		return "application/pdf"
	case ".doc":
		return "application/msword"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".ppt":
		return "application/vnd.ms-powerpoint"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	}
	return ""
}
