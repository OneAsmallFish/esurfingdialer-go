package netclient

import (
	"net/http"
	"net/url"
	"testing"
)

func TestResponseURLExcludesQueryAndUserInfo(t *testing.T) {
	u, err := url.Parse("http://user:secret@example.test/ticket.cgi?wlanuserip=1.2.3.4&token=secret")
	if err != nil {
		t.Fatal(err)
	}
	resp := &http.Response{Request: &http.Request{URL: u}}

	if got, want := responseURL(resp), "http://example.test/ticket.cgi"; got != want {
		t.Fatalf("responseURL() = %q, want %q", got, want)
	}
}

func TestResponseURLHandlesMissingRequest(t *testing.T) {
	if got := responseURL(&http.Response{}); got != "" {
		t.Fatalf("responseURL() = %q, want empty string", got)
	}
}
