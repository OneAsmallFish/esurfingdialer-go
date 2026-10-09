package netclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSMSRequestJSONFieldNames(t *testing.T) {
	got, err := json.Marshal(smsRequest{
		SchoolID:  "x",
		UserName:  "y",
		Timestamp: "t",
		Token:     "a",
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	const want = `{"schoolid":"x","username":"y","timestamp":"t","authenticator":"a"}`
	if string(got) != want {
		t.Fatalf("SMS JSON mismatch:\n got: %s\nwant: %s", got, want)
	}
}

func TestSMSPostFollowsRedirect(t *testing.T) {
	var firstBody string
	var finalBody string
	var finalArea string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		switch r.URL.Path {
		case "/start":
			firstBody = string(body)
			w.Header().Set("area", "sms-area")
			w.Header().Set("Location", "/final")
			w.WriteHeader(http.StatusFound)
		case "/final":
			finalBody = string(body)
			finalArea = r.Header.Get("CDC-Area")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"rescode":"0"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c, st := newTestClient(t)
	st.SetSchoolID("school")
	st.SetExtraCfgURLItem("QueryAuthCode", srv.URL+"/start")
	if !c.GetVerifyCode(context.Background(), "user") {
		t.Fatal("GetVerifyCode returned false after redirect")
	}
	if firstBody == "" || finalBody != firstBody {
		t.Fatalf("SMS body not preserved: first=%q final=%q", firstBody, finalBody)
	}
	if st.Area() != "sms-area" || finalArea != "sms-area" {
		t.Fatalf("SMS area not captured/forwarded: state=%q header=%q", st.Area(), finalArea)
	}
}
