// Package storage abstracts object storage for file attachments. It prefers a
// MinIO/S3 backend; if MinIO is unreachable (or unconfigured) it falls back to
// a local filesystem store so the feature still works in bare dev environments.
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Store interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Backend() string
}

// New returns a MinIO-backed store when a bucket can be reached/created, else a
// filesystem-backed store rooted at STORAGE_DIR (or a temp dir). The second
// return value names the active backend ("minio" or "filesystem").
func New(endpoint, accessKey, secretKey, bucket string, secure bool) (Store, string) {
	if endpoint != "" && accessKey != "" {
		s, err := newMinio(endpoint, accessKey, secretKey, bucket, secure)
		if err == nil {
			return s, "minio"
		}
		fmt.Printf("storage: minio unavailable (%v), falling back to filesystem\n", err)
	}
	return newFS(), "filesystem"
}

/* ------------------------------- MinIO --------------------------------- */

type minioStore struct {
	client *minio.Client
	bucket string
}

func newMinio(endpoint, accessKey, secretKey, bucket string, secure bool) (*minioStore, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, fmt.Errorf("new client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("bucket check: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("make bucket: %w", err)
		}
	}
	return &minioStore{client: client, bucket: bucket}, nil
}

func (m *minioStore) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := m.client.PutObject(ctx, m.bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (m *minioStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := m.client.GetObject(ctx, m.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	// Probe so a missing object errors here rather than on first read.
	if _, err := obj.Stat(); err != nil {
		obj.Close()
		return nil, err
	}
	return obj, nil
}

func (m *minioStore) Delete(ctx context.Context, key string) error {
	return m.client.RemoveObject(ctx, m.bucket, key, minio.RemoveObjectOptions{})
}

func (m *minioStore) Backend() string { return "minio" }

/* ----------------------------- Filesystem ------------------------------ */

type fsStore struct{ root string }

func newFS() *fsStore {
	root := os.Getenv("STORAGE_DIR")
	if root == "" {
		root = filepath.Join(os.TempDir(), "garageflow-uploads")
	}
	_ = os.MkdirAll(root, 0o755)
	return &fsStore{root: root}
}

func (f *fsStore) path(key string) string {
	// keys use "/" separators; keep them as nested dirs, guard against traversal.
	clean := filepath.Clean("/" + strings.ReplaceAll(key, "..", ""))
	return filepath.Join(f.root, clean)
}

func (f *fsStore) Put(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	p := f.path(key)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	out, err := os.Create(p)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, r)
	return err
}

func (f *fsStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	return os.Open(f.path(key))
}

func (f *fsStore) Delete(_ context.Context, key string) error {
	err := os.Remove(f.path(key))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (f *fsStore) Backend() string { return "filesystem" }
