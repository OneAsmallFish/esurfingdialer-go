package netclient

// T4.7 重定向行为 mock 测试（GO_DIALER_REVIEW_PLAN.md Phase 4）。
// 验证 Go 侧 followRedirects 与 net-matrix.md T4.2 矩阵的每一条行为一致：
//  1) 302 带 area/schoolid/domain 头 → 捕获进 state，后续请求补 CDC-* 头
//  2) 302 不带这些头 → state 不被污染，但已设置过的值仍会附带
//  3) 3xx 无 Location → 返回该响应（不报错）
//  4) 超过 5 次重定向 → 返回第 6 个（仍为重定向的）响应，不报错，共 6 次请求
//  5) POST（c.Post）手动跟随 302，并在每跳重发原 body 与请求头

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"esurfingdialer/internal/state"
)

func newTestClient(t *testing.T) (*Client, *state.State) {
	t.Helper()
	st := state.New()
	st.SetClientID("test-client-id")
	st.SetAlgoID("00000000-0000-0000-0000-000000000000")
	return New(st, "", "", ""), st
}

func TestRedirectCapturesHeadersAndAddsCDC(t *testing.T) {
	var gotSecond map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			w.Header().Set("area", "test-area")
			w.Header().Set("schoolid", "test-school")
			w.Header().Set("domain", "test-domain")
			w.Header().Set("Location", "/final")
			w.WriteHeader(302)
			return
		}
		gotSecond = map[string]string{
			"CDC-Area":     r.Header.Get("CDC-Area"),
			"CDC-SchoolId": r.Header.Get("CDC-SchoolId"),
			"CDC-Domain":   r.Header.Get("CDC-Domain"),
			"User-Agent":   r.Header.Get("User-Agent"),
			"Client-ID":    r.Header.Get("Client-ID"),
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c, st := newTestClient(t)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/start", nil)
	resp, err := c.followRedirects(req, 5)
	if err != nil {
		t.Fatalf("followRedirects: %v", err)
	}
	resp.Body.Close()

	if st.Area() != "test-area" || st.SchoolID() != "test-school" || st.Domain() != "test-domain" {
		t.Errorf("state not captured: area=%q school=%q domain=%q", st.Area(), st.SchoolID(), st.Domain())
	}
	if gotSecond["CDC-Area"] != "test-area" || gotSecond["CDC-SchoolId"] != "test-school" || gotSecond["CDC-Domain"] != "test-domain" {
		t.Errorf("CDC headers not sent on follow-up: %v", gotSecond)
	}
	if gotSecond["User-Agent"] != userAgent || gotSecond["Client-ID"] != "test-client-id" {
		t.Errorf("UA/Client-ID missing on follow-up: %v", gotSecond)
	}
	t.Logf("OK: %v", gotSecond)
}

func TestRedirectNoHeadersNoPollution(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			w.Header().Set("Location", "/final")
			w.WriteHeader(302)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c, st := newTestClient(t)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/start", nil)
	resp, err := c.followRedirects(req, 5)
	if err != nil {
		t.Fatalf("followRedirects: %v", err)
	}
	resp.Body.Close()
	if st.Area() != "" || st.SchoolID() != "" || st.Domain() != "" {
		t.Errorf("state polluted: %q/%q/%q", st.Area(), st.SchoolID(), st.Domain())
	}
	t.Log("OK: no pollution")
}

func TestRedirectMissingLocationReturnsResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(302) // 无 Location
	}))
	defer srv.Close()

	c, _ := newTestClient(t)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/start", nil)
	resp, err := c.followRedirects(req, 5)
	if err != nil {
		t.Fatalf("followRedirects: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 302 {
		t.Errorf("expected 302 returned as-is, got %d", resp.StatusCode)
	}
	t.Log("OK: 302 without Location returned as-is")
}

func TestRedirectLoopCapAtFiveFollows(t *testing.T) {
	count := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.Header().Set("Location", "/loop")
		w.WriteHeader(302)
	}))
	defer srv.Close()

	c, _ := newTestClient(t)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/start", nil)
	resp, err := c.followRedirects(req, 5)
	if err != nil {
		t.Fatalf("followRedirects: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 302 {
		t.Errorf("expected final 302, got %d", resp.StatusCode)
	}
	if count != 6 {
		t.Errorf("expected 6 requests (1 + 5 follows), got %d", count)
	}
	t.Logf("OK: capped at 5 follows, %d requests", count)
}

func TestPostFollowsRedirectAndResendsBody(t *testing.T) {
	var gotBody string
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			w.Header().Set("Location", "/elsewhere")
			w.WriteHeader(302)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		gotBody = string(body)
		gotContentType = r.Header.Get("Content-Type")
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c, _ := newTestClient(t)
	got, err := c.Post(context.Background(), srv.URL+"/start", "payload", nil)
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if string(got) != "ok" || gotBody != "payload" {
		t.Fatalf("unexpected response/body: response=%q body=%q", got, gotBody)
	}
	if gotContentType != "application/x-www-form-urlencoded" {
		t.Fatalf("Content-Type not preserved: %q", gotContentType)
	}
}

func TestPostRedirectCapturesAreaAndAddsCDCHeader(t *testing.T) {
	var gotArea string
	var firstArea string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			firstArea = r.Header.Get("CDC-Area")
			w.Header().Set("area", "test-area")
			w.Header().Set("Location", "/final")
			w.WriteHeader(http.StatusFound)
			return
		}
		gotArea = r.Header.Get("CDC-Area")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, st := newTestClient(t)
	st.SetArea("stale-area")
	if _, err := c.Post(context.Background(), srv.URL+"/start", "payload", nil); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if st.Area() != "test-area" || gotArea != "test-area" {
		t.Fatalf("area not captured/forwarded: state=%q header=%q", st.Area(), gotArea)
	}
	if firstArea != "" {
		t.Fatalf("CDC-Area unexpectedly sent on first request: %q", firstArea)
	}
}

func TestPostRedirectWithoutLocationReturnsStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()

	c, _ := newTestClient(t)
	if _, err := c.Post(context.Background(), srv.URL, "payload", nil); err == nil || !strings.Contains(err.Error(), "unexpected status: 302") {
		t.Fatalf("expected unexpected-status error, got %v", err)
	}
}

func TestPostRedirectCapsAtFiveFollows(t *testing.T) {
	count := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.Header().Set("Location", "/loop")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()

	c, _ := newTestClient(t)
	if _, err := c.Post(context.Background(), srv.URL, "payload", nil); err == nil || !strings.Contains(err.Error(), "unexpected status: 302") {
		t.Fatalf("expected final redirect status error, got %v", err)
	}
	if count != 6 {
		t.Fatalf("expected 6 requests (1 + 5 follows), got %d", count)
	}
}
