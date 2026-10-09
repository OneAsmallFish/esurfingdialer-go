package netclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type countingReadCloser struct {
	reader io.Reader
	read   int
	closed bool
}

func (r *countingReadCloser) Read(data []byte) (int, error) {
	n, err := r.reader.Read(data)
	r.read += n
	return n, err
}

func (r *countingReadCloser) Close() error {
	r.closed = true
	return nil
}

func TestReadResponseBodyLimit(t *testing.T) {
	withinLimit := bytes.Repeat([]byte{'x'}, maxResponseBodyBytes)
	got, err := readResponseBody(bytes.NewReader(withinLimit))
	if err != nil {
		t.Fatalf("read limit-sized response: %v", err)
	}
	if len(got) != maxResponseBodyBytes {
		t.Fatalf("got %d bytes, want %d", len(got), maxResponseBodyBytes)
	}

	tooLarge := strings.NewReader(string(withinLimit) + "x")
	if _, err := readResponseBody(tooLarge); err == nil {
		t.Fatal("oversized response was accepted")
	}
}

func TestPostRejectsOversizedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte{'x'}, maxResponseBodyBytes+1))
	}))
	defer srv.Close()

	client, _ := newTestClient(t)
	if _, err := client.Post(context.Background(), srv.URL, "payload", nil); err == nil {
		t.Fatal("Post accepted an oversized response")
	}
}

func TestPostResponseClassifiesDynamicZSMModule(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Error-Code", "200")
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("module"))
	}))
	defer srv.Close()

	client, _ := newTestClient(t)
	response, err := client.PostResponse(context.Background(), srv.URL, "payload", nil)
	if !errors.Is(err, ErrDynamicZSMModule) {
		t.Fatalf("PostResponse error = %v, want ErrDynamicZSMModule", err)
	}
	if response.ErrorCode != "200" || response.ContentType != "application/octet-stream" || string(response.Body) != "module" {
		t.Fatalf("unexpected response metadata: %+v", response)
	}
}

func TestCloseRedirectBodyDrainsWithLimit(t *testing.T) {
	small := &countingReadCloser{reader: strings.NewReader("redirect body")}
	closeRedirectBody(small)
	if !small.closed || small.read != len("redirect body") {
		t.Fatalf("small body not drained and closed: read=%d closed=%v", small.read, small.closed)
	}

	large := &countingReadCloser{reader: strings.NewReader(strings.Repeat("x", maxRedirectDrainBytes+100))}
	closeRedirectBody(large)
	if !large.closed || large.read != maxRedirectDrainBytes {
		t.Fatalf("large body drain was not bounded: read=%d closed=%v", large.read, large.closed)
	}
}
