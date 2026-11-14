package filehost

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

type Upload struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
}
type Record struct {
	Upload
	Owner     string `json:"-"`
	Backend   string `json:"-"`
	ObjectKey string `json:"-"`
	TokenHash []byte `json:"-"`
	NetworkID string `json:"-"`
}
type Provider struct {
	CanDelete bool     `json:"canDelete"`
	Provider  string   `json:"provider"`
	Available bool     `json:"available"`
	MaxBytes  int64    `json:"maxBytes"`
	Types     []string `json:"types"`
	Reason    string   `json:"reason,omitempty"`
	URL       string   `json:"-"`
	User      string   `json:"-"`
	Password  string   `json:"-"`
}
type Options struct {
	AvatarBlob   Blob
	Store        *storage.Store
	Blob         Blob
	PublicURL    string
	MaxBytes     int64
	Types        []string
	Client       *http.Client
	TrustedHosts []string
}
type Service struct {
	options Options
}

var ErrNotFound = errors.New("upload not found")

func New(options Options) (*Service, error) {
	if options.MaxBytes <= 0 {
		options.MaxBytes = 25 << 20
	}
	if options.Types == nil {
		options.Types = []string{"image/*", "video/*", "audio/*", "text/plain", "application/pdf", "application/zip", "application/octet-stream"}
	}
	if options.Client == nil {
		options.Client = &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	client := *options.Client
	client.Jar = nil
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	options.Client = &client
	if options.Store == nil {
		return nil, fmt.Errorf("upload storage is required")
	}
	_, err := options.Store.DB.Exec(`CREATE TABLE IF NOT EXISTS file_metadata(id TEXT PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,network TEXT NOT NULL,payload BLOB NOT NULL);`)
	if err != nil {
		return nil, err
	}
	return &Service{options: options}, nil
}
func random(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func (s *Service) LocalProvider() Provider {
	if s.options.Blob == nil {
		return Provider{Provider: "unavailable", Types: []string{}, Reason: "IRCord file hosting is disabled and no compatible upstream filehost is available."}
	}
	return Provider{CanDelete: true, Provider: s.options.Blob.Name(), Available: true, MaxBytes: s.options.MaxBytes, Types: s.options.Types}
}
func (s *Service) Resolve(ctx context.Context, host, advertised, user, password string) Provider {
	local := s.LocalProvider()
	if local.Available {
		return local
	}
	if advertised == "" {
		return local
	}
	u, err := url.Parse(advertised)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
		local.Reason = "The upstream filehost must use HTTPS."
		return local
	}
	if user != "" {
		trusted := strings.EqualFold(u.Hostname(), host)
		for _, h := range s.options.TrustedHosts {
			if strings.EqualFold(h, u.Hostname()) {
				trusted = true
			}
		}
		if !trusted {
			local.Reason = "The upstream filehost credential destination needs administrator approval."
			return local
		}
	}
	probe, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(probe, "OPTIONS", u.String(), nil)
	res, err := s.options.Client.Do(req)
	if err != nil {
		local.Reason = "The upstream filehost is unavailable."
		return local
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		local.Reason = "The upstream filehost does not support discovery."
		return local
	}
	types := []string{}
	for _, t := range strings.Split(res.Header.Get("Accept-Post"), ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			types = append(types, t)
		}
	}
	if len(types) == 0 {
		types = s.options.Types
	}
	return Provider{Provider: "upstream", Available: true, MaxBytes: s.options.MaxBytes, Types: types, URL: u.String(), User: user, Password: password}
}
func accepted(types []string, contentType string) bool {
	for _, t := range types {
		if t == "*/*" || t == contentType || strings.HasSuffix(t, "/*") && strings.HasPrefix(contentType, strings.TrimSuffix(t, "*")) {
			return true
		}
	}
	return false
}
func (s *Service) Upload(ctx context.Context, owner, network, name string, data []byte, provider Provider) (Upload, error) {
	if !provider.Available {
		return Upload{}, fmt.Errorf("%s", provider.Reason)
	}
	if len(data) == 0 || int64(len(data)) > provider.MaxBytes {
		return Upload{}, fmt.Errorf("file exceeds the upload limit or is empty")
	}
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "." || len(name) > 200 || strings.ContainsAny(name, "\r\n\x00") {
		return Upload{}, fmt.Errorf("invalid filename")
	}
	contentType := strings.Split(http.DetectContentType(data), ";")[0]
	if !accepted(provider.Types, contentType) {
		return Upload{}, fmt.Errorf("unsupported file type")
	}
	upload := Upload{ID: random(16), Name: name, Size: int64(len(data)), ContentType: contentType}
	if strings.HasPrefix(contentType, "image/") {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width > 16384 || cfg.Height > 16384 || int64(cfg.Width)*int64(cfg.Height) > 64_000_000 {
			return Upload{}, fmt.Errorf("invalid image dimensions")
		}
		upload.Width = cfg.Width
		upload.Height = cfg.Height
	}
	token := random(24)
	hash := sha256.Sum256([]byte(token))
	objectKey := upload.ID
	if provider.Provider == "upstream" {
		req, err := http.NewRequestWithContext(ctx, "POST", provider.URL, bytes.NewReader(data))
		if err != nil {
			return Upload{}, err
		}
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
		if provider.User != "" {
			req.SetBasicAuth(provider.User, provider.Password)
		}
		res, err := s.options.Client.Do(req)
		if err != nil {
			return Upload{}, fmt.Errorf("upstream upload failed")
		}
		defer res.Body.Close()
		if res.StatusCode != 201 {
			return Upload{}, fmt.Errorf("upstream upload was rejected (%d)", res.StatusCode)
		}
		location, err := url.Parse(res.Header.Get("Location"))
		if err != nil || res.Header.Get("Location") == "" {
			return Upload{}, fmt.Errorf("upstream filehost omitted its download URL")
		}
		base, _ := url.Parse(provider.URL)
		location = base.ResolveReference(location)
		if location.Scheme != "https" || location.Host == "" || location.User != nil {
			return Upload{}, fmt.Errorf("invalid upstream download URL")
		}
		upload.URL = location.String()
		objectKey = ""
	} else {
		blob := s.blob(provider.Provider)
		if blob == nil {
			return Upload{}, fmt.Errorf("file provider unavailable")
		}
		upload.URL = strings.TrimRight(s.options.PublicURL, "/") + "/files/" + upload.ID + "/" + token + "/" + url.PathEscape(name)
		if err := blob.Put(ctx, objectKey, bytes.NewReader(data), upload.Size, contentType); err != nil {
			return Upload{}, fmt.Errorf("file storage is unavailable")
		}
	}
	raw, _ := json.Marshal(upload)
	sum := sha256.Sum256(data)
	tx, err := s.options.Store.DB.Begin()
	if err == nil {
		defer tx.Rollback()
		_, err = tx.Exec("INSERT INTO files(id,owner,name,mime,size,sha256,backend,object_key,token_hash,created_ns) VALUES(?,?,?,?,?,?,?,?,?,?)", upload.ID, owner, name, contentType, upload.Size, hex.EncodeToString(sum[:]), provider.Provider, objectKey, hash[:], time.Now().UnixNano())
		if err == nil {
			_, err = tx.Exec("INSERT INTO file_metadata(id,network,payload) VALUES(?,?,?)", upload.ID, network, raw)
		}
		if err == nil {
			err = tx.Commit()
		}
	}
	if err != nil {
		if objectKey != "" {
			_ = s.blob(provider.Provider).Delete(ctx, objectKey)
		}
		return Upload{}, fmt.Errorf("file metadata is unavailable")
	}
	return upload, nil
}
func (s *Service) Record(owner, id string) (Record, error) {
	var record Record
	var raw []byte
	err := s.options.Store.DB.QueryRow("SELECT f.owner,f.backend,f.object_key,f.token_hash,m.network,m.payload FROM files f JOIN file_metadata m ON m.id=f.id WHERE f.id=? AND (?='' OR f.owner=?) AND f.deleted=0", id, owner, owner).Scan(&record.Owner, &record.Backend, &record.ObjectKey, &record.TokenHash, &record.NetworkID, &raw)
	if err != nil {
		return record, ErrNotFound
	}
	err = json.Unmarshal(raw, &record.Upload)
	return record, err
}
func (s *Service) Attachment(owner, network, id string) (*model.Attachment, error) {
	r, err := s.Record(owner, id)
	if err != nil || r.NetworkID != "" && r.NetworkID != network {
		return nil, ErrNotFound
	}
	return &model.Attachment{Name: r.Name, URL: r.URL, Width: r.Width, Height: r.Height, Alt: r.Name, ContentType: r.ContentType, Size: r.Size}, nil
}
func (s *Service) Download(ctx context.Context, id, token string) (Record, io.ReadCloser, error) {
	r, err := s.Record("", id)
	if err != nil {
		return r, nil, err
	}
	hash := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare(hash[:], r.TokenHash) != 1 || r.Backend == "upstream" || s.blob(r.Backend) == nil {
		return r, nil, ErrNotFound
	}
	body, err := s.blob(r.Backend).Open(ctx, r.ObjectKey)
	return r, body, err
}
func (s *Service) Delete(ctx context.Context, owner, id string) error {
	r, err := s.Record(owner, id)
	if err != nil {
		return err
	}
	if r.Backend == "upstream" {
		return fmt.Errorf("upstream filehost does not advertise a deletion protocol")
	}
	if s.blob(r.Backend) == nil {
		return fmt.Errorf("original file storage is unavailable")
	}
	if err = s.blob(r.Backend).Delete(ctx, r.ObjectKey); err != nil {
		return err
	}
	_, err = s.options.Store.DB.Exec("DELETE FROM files WHERE owner=? AND id=?", owner, id)
	return err
}
func (s *Service) DeleteAccount(ctx context.Context, owner string) error {
	rows, err := s.options.Store.DB.Query("SELECT id,backend FROM files WHERE owner=?", owner)
	if err != nil {
		return err
	}
	type item struct{ id, backend string }
	items := []item{}
	for rows.Next() {
		var v item
		if err = rows.Scan(&v.id, &v.backend); err != nil {
			rows.Close()
			return err
		}
		items = append(items, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	// One file that can't be removed doesn't stop the others; the caller retries the rest.
	var failed []error
	for _, v := range items {
		if v.backend == "upstream" {
			continue
		}
		if s.blob(v.backend) == nil {
			// Its storage is not configured now. Keep the record, so the object is removed once
			// that storage is configured again; the account's deletion waits until then.
			var key string
			_ = s.options.Store.DB.QueryRow("SELECT object_key FROM files WHERE owner=? AND id=?", owner, v.id).Scan(&key)
			slog.Warn("Deleting an account is waiting for file storage that is not configured", "account", owner, "file", v.id, "storage", v.backend, "object", key)
			failed = append(failed, fmt.Errorf("file %s is in %s storage, which is not configured", v.id, v.backend))
			continue
		}
		if err = s.Delete(ctx, owner, v.id); err != nil {
			failed = append(failed, err)
		}
	}
	return errors.Join(failed...)
}

func (s *Service) blob(provider string) Blob {
	if provider == "avatar" {
		return s.options.AvatarBlob
	}
	if s.options.Blob != nil && s.options.Blob.Name() == provider {
		return s.options.Blob
	}
	return nil
}
func (s *Service) AvatarProvider() Provider {
	return Provider{CanDelete: true, Provider: "avatar", Available: s.options.AvatarBlob != nil, MaxBytes: 128 << 10, Types: []string{"image/png", "image/jpeg", "image/webp"}, Reason: "Profile image storage is unavailable."}
}

// DeletableUpload returns the id of the owner's stored upload served at value,
// or "" when Delete would refuse it (another owner, an upstream filehost, or a
// storage backend that is no longer configured).
func (s *Service) DeletableUpload(owner, value string) string {
	var id, backend string
	err := s.options.Store.DB.QueryRow("SELECT f.id,f.backend FROM file_metadata m JOIN files f ON f.id=m.id WHERE f.owner=? AND f.deleted=0 AND json_extract(m.payload,'$.url')=? LIMIT 1", owner, value).Scan(&id, &backend)
	if err != nil || backend == "upstream" || s.blob(backend) == nil {
		return ""
	}
	return id
}

// HostedFile names a deleted upload and the account that uploaded it.
type HostedFile struct{ ID, Owner string }

// DeleteHostedURL deletes the stored upload served at value, whoever uploaded it.
// It is for server owners removing a moderated message's files. It reports false
// when value is not a file this server stores and can delete: unknown URLs,
// avatars, upstream filehosts, and storage that is no longer configured.
func (s *Service) DeleteHostedURL(ctx context.Context, value string) (HostedFile, bool, error) {
	var f HostedFile
	var backend string
	err := s.options.Store.DB.QueryRowContext(ctx, "SELECT f.id,f.owner,f.backend FROM file_metadata m JOIN files f ON f.id=m.id WHERE f.deleted=0 AND json_extract(m.payload,'$.url')=? LIMIT 1", value).Scan(&f.ID, &f.Owner, &backend)
	if errors.Is(err, sql.ErrNoRows) || err == nil && (backend == "upstream" || backend == "avatar" || s.blob(backend) == nil) {
		return HostedFile{}, false, nil
	}
	if err != nil {
		return HostedFile{}, false, err
	}
	if err = s.Delete(ctx, f.Owner, f.ID); err != nil {
		return HostedFile{}, false, err
	}
	return f, true, nil
}
func (s *Service) OwnsURL(owner, value string) bool {
	var n int
	_ = s.options.Store.DB.QueryRow("SELECT count(*) FROM file_metadata m JOIN files f ON f.id=m.id WHERE f.owner=? AND f.deleted=0 AND json_extract(m.payload,'$.url')=?", owner, value).Scan(&n)
	return n == 1
}
