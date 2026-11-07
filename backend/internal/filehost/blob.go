// Package filehost stores private upload metadata and issues shareable download
// capabilities. IRC/application credentials are never part of a download URL.
package filehost

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Blob interface {
	Put(context.Context, string, io.Reader, int64, string) error
	Open(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
	Name() string
}
type Local struct{ Root string }

func (b Local) Name() string { return "local" }
func (b Local) path(key string) (string, error) {
	if len(key) != 32 || strings.Trim(key, "0123456789abcdef") != "" { // gitleaks:allow -- hexadecimal validation alphabet
		return "", fmt.Errorf("invalid object key")
	}
	return filepath.Join(b.Root, key), nil
}
func (b Local) Put(ctx context.Context, key string, r io.Reader, size int64, mime string) error {
	path, err := b.path(key)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(b.Root, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(b.Root, ".upload-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err = f.Chmod(0600); err != nil {
		return err
	}
	n, err := io.Copy(f, r)
	if err != nil {
		return err
	}
	if n != size {
		return io.ErrUnexpectedEOF
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func (b Local) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	path, err := b.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}
func (b Local) Delete(ctx context.Context, key string) error {
	path, err := b.path(key)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

type S3 struct {
	Client *minio.Client
	Bucket string
	Prefix string
}

func NewS3(endpoint, bucket, region, access, secret string, secure bool) (*S3, error) {
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(access, secret, ""), Secure: secure, Region: region, BucketLookup: minio.BucketLookupPath})
	if err != nil {
		return nil, err
	}
	if bucket == "" {
		return nil, fmt.Errorf("S3 bucket is required")
	}
	return &S3{client, bucket, "ircord/"}, nil
}
func (b *S3) Name() string { return "s3" }
func (b *S3) Put(ctx context.Context, key string, r io.Reader, size int64, mime string) error {
	_, err := b.Client.PutObject(ctx, b.Bucket, b.Prefix+key, r, size, minio.PutObjectOptions{ContentType: mime, DisableMultipart: true})
	return err
}
func (b *S3) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	object, err := b.Client.GetObject(ctx, b.Bucket, b.Prefix+key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	if _, err = object.Stat(); err != nil {
		object.Close()
		return nil, err
	}
	return object, nil
}
func (b *S3) Delete(ctx context.Context, key string) error {
	return b.Client.RemoveObject(ctx, b.Bucket, b.Prefix+key, minio.RemoveObjectOptions{})
}
