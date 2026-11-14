package storage

import (
	"bytes"
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLocalStorageRejectsTraversalAndRoundTrips(t *testing.T) {
	s := Local{Root: t.TempDir()}
	ctx := context.Background()
	for _, key := range []string{"../escape", "..\\escape", "C:escape", "/escape", ""} {
		if s.Put(ctx, key, []byte("x"), "application/pdf") == nil {
			t.Fatalf("accepted %s", key)
		}
	}
	want := []byte("%PDF-1.4 test")
	if e := s.Put(ctx, "object.pdf", want, "application/pdf"); e != nil {
		t.Fatal(e)
	}
	b, e := s.Get(ctx, "object.pdf")
	if e != nil || !bytes.Equal(b, want) {
		t.Fatalf("get %v", e)
	}
	if e = s.Delete(ctx, "object.pdf"); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get(ctx, "object.pdf"); e == nil {
		t.Fatal("deleted file remains")
	}
}
func TestR2AdapterSignsRequestsAndKeepsBucketPrivate(t *testing.T) {
	stored := []byte{}
	operations := []string{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
			t.Error("unsigned S3 request")
		}
		if r.Header.Get("x-amz-acl") == "public-read" {
			t.Error("public ACL")
		}
		if r.URL.Path != "/private-bucket/object.pdf" {
			t.Error(r.URL.Path)
		}
		operations = append(operations, r.Method)
		switch r.Method {
		case "PUT":
			stored, _ = io.ReadAll(r.Body)
		case "GET":
			w.Header().Set("Content-Type", "application/pdf")
			w.Write(stored)
		case "DELETE":
			w.WriteHeader(204)
		}
	}))
	defer upstream.Close()
	client := s3.NewFromConfig(aws.Config{Region: "auto", Credentials: credentials.NewStaticCredentialsProvider("test-key", "test-secret", "")}, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(upstream.URL)
		o.UsePathStyle = true
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	r := R2{Client: client, Bucket: "private-bucket"}
	ctx := context.Background()
	if e := r.Put(ctx, "object.pdf", []byte("pdf bytes"), "application/pdf"); e != nil {
		t.Fatal(e)
	}
	b, e := r.Get(ctx, "object.pdf")
	if e != nil || string(b) != "pdf bytes" {
		t.Fatalf("read: %s %v", b, e)
	}
	if e = r.Delete(ctx, "object.pdf"); e != nil {
		t.Fatal(e)
	}
	if strings.Join(operations, ",") != "PUT,GET,DELETE" {
		t.Fatal(operations)
	}
}
