package netclient

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"esurfingdialer/internal/state"
)

func encodePortalTestResponse(t *testing.T, value any) string {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString([]byte(url.QueryEscape(string(payload))))
}

func TestPortalLoginMirrorsBrowserFlow(t *testing.T) {
	st := state.New()
	st.SetUserAgent("CCTP/test")
	c := New(st, "", "", "")
	var gotForm url.Values
	var gotCookie string
	var continuation bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/generate_204":
			if !strings.HasPrefix(r.UserAgent(), "Mozilla/5.0") {
				t.Errorf("captive user agent = %q", r.UserAgent())
			}
			http.SetCookie(w, &http.Cookie{Name: "captive-context", Value: "ok", Path: "/"})
			http.Redirect(w, r, "/portal/?wlanuserip=100.2.3.4&wlanacname=ac-test", http.StatusFound)
		case "/portal/":
			if r.Method != http.MethodGet {
				t.Fatalf("portal page method = %s", r.Method)
			}
			if r.URL.Query().Get("wlanuserip") != "100.2.3.4" || r.URL.Query().Get("wlanacname") != "ac-test" {
				t.Errorf("portal query was not retained: %s", r.URL.RawQuery)
			}
			if !strings.Contains(r.Header.Get("Cookie"), "captive-context=ok") {
				t.Errorf("captive cookie was not retained: %q", r.Header.Get("Cookie"))
			}
			if !strings.HasPrefix(r.UserAgent(), "Mozilla/5.0") {
				t.Errorf("portal user agent = %q", r.UserAgent())
			}
			http.SetCookie(w, &http.Cookie{Name: "portal-token", Value: "ok", Path: "/portal"})
			io.WriteString(w, `<form action="/portal/pws?t=li"><input id="id_userName" name="userName"><input type="hidden" name="shopId" value="shop-1"><input type='hidden' name='customPageId' value='13'><input type="hidden" name="userip" value="100.2.3.4"></form>`)
		case "/portal/pws":
			if r.Method != http.MethodPost {
				t.Fatalf("pws method = %s", r.Method)
			}
			gotCookie = r.Header.Get("Cookie")
			if !strings.HasPrefix(r.UserAgent(), "Mozilla/5.0") {
				t.Errorf("pws user agent = %q", r.UserAgent())
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			gotForm, err = url.ParseQuery(string(body))
			if err != nil {
				t.Fatal(err)
			}
			w.Header().Set("Content-Type", "text/plain")
			io.WriteString(w, encodePortalTestResponse(t, map[string]any{
				"errorNumber": "1",
				"portalLink":  "continuation-token",
			}))
		case "/portal/page/afterLogin.jsp":
			continuation = true
			if r.URL.Query().Get("pl") != "continuation-token" {
				t.Errorf("continuation pl = %q", r.URL.Query().Get("pl"))
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c.captiveURL = srv.URL + "/generate_204"

	if err := c.PortalLogin(t.Context(), srv.URL+"/portal/", "user-1", "pass-1", "100.2.3.4", "02:00:00:00:00:01"); err != nil {
		t.Fatalf("PortalLogin: %v", err)
	}
	if !strings.Contains(gotCookie, "portal-token=ok") {
		t.Fatalf("portal cookie was not retained: %q", gotCookie)
	}
	if gotForm.Get("userName") != "user-1" || gotForm.Get("userPwd") != base64.StdEncoding.EncodeToString([]byte("pass-1")) {
		t.Fatalf("credentials not encoded as browser: user=%q pwd=%q", gotForm.Get("userName"), gotForm.Get("userPwd"))
	}
	if gotForm.Get("shopId") != "shop-1" || gotForm.Get("customPageId") != "13" || gotForm.Get("userip") != "100.2.3.4" {
		t.Fatalf("hidden form fields were not preserved: %v", gotForm)
	}
	if !continuation {
		t.Fatal("portal continuation was not requested")
	}
}

func TestDecodePortalResponseRejectsMalformedData(t *testing.T) {
	if _, err := decodePortalResponse([]byte("not-base64")); err == nil {
		t.Fatal("malformed response was accepted")
	}
	encoded := encodePortalTestResponse(t, map[string]string{"errorNumber": "2"})
	result, err := decodePortalResponse([]byte(encoded))
	if err != nil || result.ErrorNumber != "2" {
		t.Fatalf("decode response = %#v, err=%v", result, err)
	}
}

func TestDecodePortalResponseResolvesDynamicErrorFields(t *testing.T) {
	encoded := encodePortalTestResponse(t, map[string]string{
		"errorNumber":           "7",
		"e_c":                   "errorCodeField",
		"e_d":                   "errorDescriptionField",
		"errorCodeField":        "E63632",
		"errorDescriptionField": "terminal limit reached",
	})
	result, err := decodePortalResponse([]byte(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if result.ErrorCode != "E63632" || result.ErrorDescription != "terminal limit reached" {
		t.Fatalf("resolved portal error = %#v", result)
	}
}

func TestStandalonePortalDetection(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://172.17.18.2:9092/portal/", nil)
	resp := &http.Response{Request: req}
	if !isPortalEndpoint(resp) {
		t.Fatal("portal endpoint was not recognized")
	}
	if !isStandalonePortal(resp, []byte(`<form action="/portal/pws?t=li"><input id="id_userName"></form>`)) {
		t.Fatal("portal host was not detected")
	}
	if isStandalonePortal(resp, nil) {
		t.Fatal("empty portal response was misclassified")
	}
	req = httptest.NewRequest(http.MethodGet, "http://captive.example/", nil)
	resp.Request = req
	if !isStandalonePortal(resp, []byte(`<form action="/portal/pws?t=li"><input id="id_userName"></form>`)) {
		t.Fatal("portal form marker was not detected")
	}
	if isStandalonePortal(resp, []byte("<html><title>Example</title></html>")) {
		t.Fatal("ordinary page was misclassified as portal")
	}
}
