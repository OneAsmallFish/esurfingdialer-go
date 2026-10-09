package app

// T7.2：验证 Dialer 的运行与退出路径共用同一把锁保护可变字段。

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"esurfingdialer/internal/config"
	"esurfingdialer/internal/netclient"
	"esurfingdialer/internal/session"
	"esurfingdialer/internal/state"
)

func TestDialerTermURLRace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	st := state.New()
	st.SetClientID("race-test")
	cli := netclient.New(st, "", "", "")
	sess := session.New(st, cli)
	// 合成 zsm：keyLen=0、algoLen=36、algoId=AESECB（合法注册项，layout-1 可解析）
	zsm := append([]byte{0, 0, 0, 0, 36}, []byte("A474B1C2-3DE0-4EA2-8C5F-7093409CE6C4")...)
	if err := sess.Initialize(zsm); err != nil {
		t.Fatalf("init session: %v", err)
	}

	d := NewDialer(st, cli, sess, config.Config{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			// 模拟 runLoop 持锁调用 login 的写侧。
			d.mu.Lock()
			d.termURL = fmt.Sprintf("%s/term", srv.URL)
			d.keepURL = srv.URL
			d.mu.Unlock()
		}
	}()
	for i := 0; i < 200; i++ {
		// 模拟 shutdownLoop 持锁调用 term 的读侧。
		d.mu.Lock()
		_ = d.term(ctx)
		d.mu.Unlock()
	}
	<-done
}
