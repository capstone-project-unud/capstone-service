package storage

import (
	"bytes"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const MaxFile = 10 * 1024 * 1024

type Store interface {
	Put(context.Context, string, []byte, string) error
	Get(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}
type Local struct{ Root string }

func (l Local) path(key string) (string, error) {
	if key == "" || filepath.Base(key) != key || strings.ContainsAny(key, "/\\:") {
		return "", fmt.Errorf("invalid storage key")
	}
	return filepath.Join(l.Root, key), nil
}
func (l Local) Put(_ context.Context, key string, b []byte, _ string) error {
	p, e := l.path(key)
	if e != nil {
		return e
	}
	return os.WriteFile(p, b, 0600)
}
func (l Local) Get(_ context.Context, key string) ([]byte, error) {
	p, e := l.path(key)
	if e != nil {
		return nil, e
	}
	f, e := os.Open(p)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, MaxFile+1))
}
func (l Local) Delete(_ context.Context, key string) error {
	p, e := l.path(key)
	if e != nil {
		return e
	}
	return os.Remove(p)
}

type R2 struct {
	Client *s3.Client
	Bucket string
}

func (r R2) Put(ctx context.Context, key string, b []byte, mime string) error {
	_, e := r.Client.PutObject(ctx, &s3.PutObjectInput{Bucket: &r.Bucket, Key: &key, Body: bytes.NewReader(b), ContentType: &mime})
	return e
}
func (r R2) Get(ctx context.Context, key string) ([]byte, error) {
	o, e := r.Client.GetObject(ctx, &s3.GetObjectInput{Bucket: &r.Bucket, Key: &key})
	if e != nil {
		return nil, e
	}
	defer o.Body.Close()
	return io.ReadAll(io.LimitReader(o.Body, MaxFile+1))
}
func (r R2) Delete(ctx context.Context, key string) error {
	_, e := r.Client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &r.Bucket, Key: &key})
	return e
}
func New() (Store, error) {
	mode := os.Getenv("STORAGE_DRIVER")
	if mode == "local" {
		root := os.Getenv("UPLOAD_DIR")
		if root == "" {
			root = "data/uploads"
		}
		if e := os.MkdirAll(root, 0700); e != nil {
			return nil, e
		}
		return Local{root}, nil
	}
	if mode != "r2" {
		return nil, fmt.Errorf("set STORAGE_DRIVER to local or r2")
	}
	account, bucket, key, secret := os.Getenv("R2_ACCOUNT_ID"), os.Getenv("R2_BUCKET"), os.Getenv("R2_ACCESS_KEY_ID"), os.Getenv("R2_SECRET_ACCESS_KEY")
	if account == "" || bucket == "" || key == "" || secret == "" || strings.ContainsAny(account, "/.:\\") {
		return nil, fmt.Errorf("complete R2 configuration required")
	}
	client := s3.NewFromConfig(aws.Config{Region: "auto", Credentials: credentials.NewStaticCredentialsProvider(key, secret, "")}, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("https://" + account + ".r2.cloudflarestorage.com")
		o.UsePathStyle = true
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	return R2{client, bucket}, nil
}
