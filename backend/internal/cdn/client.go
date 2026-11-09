// Package cdn provides object storage access independently of message processing.
package cdn

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Options struct {
	Endpoint, Bucket, Region, AccessKey, SecretKey, SessionToken string
	Prefix, PublicURL                                            string
	Transport                                                    http.RoundTripper
}

type Object struct {
	Key, URL, ContentType, ETag string
	Size                        int64
	Modified                    time.Time
}

type ReadOptions struct {
	Offset int64
	Length int64 // zero reads through the end of the object
}

type Client struct {
	s3                        *minio.Client
	bucket, prefix, publicURL string
	namespace                 string
}

func New(o Options) (*Client, error) {
	u, err := baseURL(o.Endpoint)
	if err != nil || u.Path != "" && u.Path != "/" || o.Bucket == "" {
		return nil, errors.New("CDN requires an HTTP(S) storage endpoint and bucket")
	}
	public := ""
	if o.PublicURL != "" {
		p, err := baseURL(o.PublicURL)
		if err != nil {
			return nil, errors.New("invalid CDN public URL")
		}
		public = strings.TrimRight(p.String(), "/")
	}
	prefix := strings.Trim(o.Prefix, "/")
	if prefix != "" {
		if !validKey(prefix) {
			return nil, errors.New("invalid CDN prefix")
		}
		prefix += "/"
	}
	s3, err := minio.New(u.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(o.AccessKey, o.SecretKey, o.SessionToken),
		Secure: u.Scheme == "https", Region: o.Region, Transport: o.Transport,
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, errors.New("invalid CDN storage configuration")
	}
	identity := sha256.Sum256([]byte(u.String() + "\x00" + o.Bucket + "\x00" + prefix + "\x00" + public))
	return &Client{s3: s3, bucket: o.Bucket, prefix: prefix, publicURL: public, namespace: hex.EncodeToString(identity[:])}, nil
}

// Namespace isolates durable metadata when the configured bucket or origin changes.
// It contains no storage credentials and is never part of a public object key.
func (c *Client) Namespace() string { return c.namespace }

func baseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("invalid base URL")
	}
	return u, nil
}

func validKey(key string) bool {
	if key == "" || len(key) > 1024 || strings.ContainsAny(key, "\\%?#\r\n\x00") || strings.HasPrefix(key, "/") || path.Clean(key) != key {
		return false
	}
	for _, part := range strings.Split(key, "/") {
		if part == "." || part == ".." {
			return false
		}
	}
	return true
}

// URL maps a logical object key to the configured delivery origin. An empty
// result means storage is configured without public delivery.
func (c *Client) URL(key string) string {
	if !validKey(key) || c.publicURL == "" {
		return ""
	}
	u, _ := url.Parse(c.publicURL)
	u.Path = strings.TrimRight(u.Path, "/") + "/" + c.prefix + key
	return u.String()
}

func (c *Client) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) (Object, error) {
	return c.put(ctx, key, body, size, minio.PutObjectOptions{ContentType: contentType, DisableMultipart: true})
}

// PutExpiring records an absolute delivery expiry. Object deletion is owned by
// the caller's durable cleanup and an operator-managed bucket lifecycle rule.
func (c *Client) PutExpiring(ctx context.Context, key string, body io.Reader, size int64, contentType string, expires time.Time) (Object, error) {
	if !expires.After(time.Now()) {
		return Object{}, errors.New("CDN object expiry must be in the future")
	}
	return c.put(ctx, key, body, size, minio.PutObjectOptions{
		ContentType: contentType, DisableMultipart: true, Expires: expires,
		// A fixed max-age would let a late CDN fill outlive the object's deadline.
		CacheControl: "public, max-age=0, must-revalidate",
	})
}

func (c *Client) put(ctx context.Context, key string, body io.Reader, size int64, opts minio.PutObjectOptions) (Object, error) {
	if !validKey(key) || size < 0 {
		return Object{}, errors.New("invalid CDN object key or size")
	}
	info, err := c.s3.PutObject(ctx, c.bucket, c.prefix+key, body, size, opts)
	if err != nil {
		return Object{}, err
	}
	return Object{Key: key, URL: c.URL(key), Size: info.Size, ContentType: opts.ContentType, ETag: info.ETag}, nil
}

func (c *Client) Stat(ctx context.Context, key string) (Object, error) {
	if !validKey(key) {
		return Object{}, errors.New("invalid CDN object key")
	}
	info, err := c.s3.StatObject(ctx, c.bucket, c.prefix+key, minio.StatObjectOptions{})
	if err != nil {
		return Object{}, err
	}
	return c.object(key, info), nil
}

func (c *Client) Open(ctx context.Context, key string, read ReadOptions) (io.ReadCloser, Object, error) {
	if !validKey(key) || read.Offset < 0 || read.Length < 0 {
		return nil, Object{}, errors.New("invalid CDN read")
	}
	opts := minio.GetObjectOptions{}
	if read.Offset > 0 || read.Length > 0 {
		end := int64(0)
		if read.Length > 0 {
			end = read.Offset + read.Length - 1
			if end < read.Offset {
				return nil, Object{}, errors.New("invalid CDN range")
			}
		}
		if err := opts.SetRange(read.Offset, end); err != nil {
			return nil, Object{}, err
		}
	}
	// Stat separately: calling Stat on a lazy minio.Object clears its Range.
	info, err := c.s3.StatObject(ctx, c.bucket, c.prefix+key, minio.StatObjectOptions{})
	if err != nil {
		return nil, Object{}, err
	}
	body, err := c.s3.GetObject(ctx, c.bucket, c.prefix+key, opts)
	if err != nil {
		return nil, Object{}, err
	}
	return body, c.object(key, info), nil
}

func (c *Client) object(key string, info minio.ObjectInfo) Object {
	return Object{Key: key, URL: c.URL(key), Size: info.Size, ContentType: info.ContentType, ETag: info.ETag, Modified: info.LastModified}
}

func (c *Client) Delete(ctx context.Context, key string) error {
	if !validKey(key) {
		return errors.New("invalid CDN object key")
	}
	return c.s3.RemoveObject(ctx, c.bucket, c.prefix+key, minio.RemoveObjectOptions{})
}
