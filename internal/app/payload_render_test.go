package app

// T8.1 XML payload 渲染对拍（Go 侧）。
// 方法：httptest mock 服务器承载真实构造路径——测试在 app 包内构造真实 Dialer，
// 预置固定状态值（clientID/ip/mac/host/ticket），调用真实的 getTicket/login/heartbeat/term，
// 服务器用同算法（AESECB）解密捕获请求体，得到 Go 代码真实渲染的 payload 明文。
// 唯一动态值 <local-time>（utils.NowString 内联调用）以正则归一化为固定占位后，
// 与 Harness.java xml 模式（真实 kotlin trimIndent 渲染 + 相同占位值）的输出做字节级比对。

import (
	"context"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"

	"esurfingdialer/internal/cipher"
	"esurfingdialer/internal/config"
	"esurfingdialer/internal/netclient"
	"esurfingdialer/internal/session"
	"esurfingdialer/internal/state"
)

const (
	tAlgo      = "A474B1C2-3DE0-4EA2-8C5F-7093409CE6C4" // AESECB（Phase 3 对拍已证与 jar 逐位一致）
	tClientID  = "11111111-2222-3333-4444-555555555555"
	tHost      = "TESTHOSTAB"
	tIP        = "1.2.3.4"
	tAcIP      = "5.6.7.8"
	tMac       = "aa:bb:cc:dd:ee:ff"
	tTicket    = "FIXEDTICKET123"
	tUser      = "testuser"
	tPass      = "testpass"
	tTimeFixed = "1970-01-01 00:00:00"
)

var tTimeRe = regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`)

func TestPayloadRenderVsJava(t *testing.T) {
	srvCipher, err := cipher.New(tAlgo)
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	captured := map[string][]string{} // path -> 按序的已归一化明文

	newHandler := func(respXML string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			plain, err := srvCipher.Decrypt(string(body))
			if err != nil {
				t.Errorf("decrypt request (%s): %v", r.URL.Path, err)
				w.WriteHeader(500)
				return
			}
			mu.Lock()
			captured[r.URL.Path] = append(captured[r.URL.Path], tTimeRe.ReplaceAllString(plain, tTimeFixed))
			mu.Unlock()
			enc, _ := srvCipher.Encrypt(respXML)
			_, _ = w.Write([]byte(enc))
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ticket", newHandler("<response><ticket>"+tTicket+"</ticket></response>"))
	mux.HandleFunc("/auth", newHandler("<response><keep-url>k</keep-url><term-url>t</term-url><keep-retry>15</keep-retry></response>"))
	mux.HandleFunc("/keep", newHandler("<response><interval>480</interval></response>"))
	mux.HandleFunc("/term", newHandler("<response></response>"))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	st := state.New()
	st.SetClientID(tClientID)
	st.SetUserIP(tIP)
	st.SetAcIP(tAcIP)
	st.SetMac(tMac)
	st.SetHostName(tHost)
	st.SetTicketURL(srv.URL + "/ticket")
	st.SetAuthURL(srv.URL + "/auth")

	cli := netclient.New(st, "", "", "")
	sess := session.New(st, cli)
	zsm := append([]byte{0, 0, 0, 0, 36}, []byte(tAlgo)...)
	if err := sess.Initialize(zsm); err != nil {
		t.Fatalf("init session: %v", err)
	}

	d := NewDialer(st, cli, sess, config.Config{User: tUser, Password: tPass})
	ctx := context.Background()

	// 真实构造路径逐个触发
	tk, err := d.getTicket(ctx)
	if err != nil || tk != tTicket {
		t.Fatalf("getTicket: %q %v", tk, err)
	}
	st.SetTicket(tk)
	if err := d.login(ctx, ""); err != nil {
		t.Fatalf("login(noverify): %v", err)
	}
	if err := d.login(ctx, "654321"); err != nil {
		t.Fatalf("login(verify): %v", err)
	}
	d.keepURL = srv.URL + "/keep"
	if err := d.heartbeat(ctx, tk); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	d.termURL = srv.URL + "/term"
	if err := d.term(ctx); err != nil {
		t.Fatalf("term: %v", err)
	}

	// 与 java-xml.out 字节级比对
	java := map[string]string{}
	data, err := os.ReadFile("../testdata/cross-validation/java-xml.out")
	if err != nil {
		t.Fatalf("read java xml output: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "XML|") {
			p := strings.SplitN(line, "|", 3)
			java[p[1]] = p[2]
		}
	}

	mu.Lock()
	defer mu.Unlock()
	compare := func(name, plain string) {
		t.Helper()
		want := java[name]
		got := hex.EncodeToString([]byte(plain))
		if got != want {
			wantPlain, _ := hex.DecodeString(want)
			t.Errorf("XML-MISMATCH %s\n  go  : %s\n  java: %s",
				name, showStr(plain), showStr(string(wantPlain)))
		} else {
			t.Logf("XML-PASS %s", name)
		}
	}
	compare("getTicket", first(captured["/ticket"]))
	compare("login-noverify", first(captured["/auth"]))
	compare("login-verify", captured["/auth"][1])
	compare("heartbeat", first(captured["/keep"]))
	compare("term", first(captured["/term"]))
}

func first(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}

func showStr(s string) string {
	return strings.ReplaceAll(s, "\n", "\\n")
}
