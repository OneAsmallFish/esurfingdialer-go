package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"esurfingdialer/internal/cipher"
	"esurfingdialer/internal/config"
	"esurfingdialer/internal/netclient"
	"esurfingdialer/internal/session"
	"esurfingdialer/internal/state"
)

func TestMaybeHeartbeatInvalidIntervalClearsLogged(t *testing.T) {
	st := state.New()
	st.SetLogged(true)
	d := &Dialer{
		st:           st,
		keepURL:      "https://example.invalid/keep",
		keepRetrySec: 0,
	}

	d.maybeHeartbeat(context.Background())
	if st.IsLogged() {
		t.Fatal("invalid heartbeat interval left dialer logged in")
	}
}

func TestEmptyHeartbeatIntervalClearsLoggedOnNextCycle(t *testing.T) {
	serverCipher, err := cipher.New(tAlgo)
	if err != nil {
		t.Fatal(err)
	}
	emptyInterval, err := serverCipher.Encrypt("<response><interval></interval></response>")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(emptyInterval))
	}))
	defer srv.Close()

	st := state.New()
	st.SetClientID(tClientID)
	st.SetLogged(true)
	cli := netclient.New(st, "", "", "")
	sess := session.New(st, cli)
	zsm := append([]byte{0, 0, 0, 0, 36}, []byte(tAlgo)...)
	if err := sess.Initialize(zsm); err != nil {
		t.Fatalf("init session: %v", err)
	}
	d := NewDialer(st, cli, sess, config.Config{})
	d.keepURL = srv.URL
	d.keepRetrySec = 1
	d.tick = time.Now().Add(-2 * time.Second)

	d.maybeHeartbeat(context.Background())
	if !st.IsLogged() {
		t.Fatal("successful heartbeat cleared logged state too early")
	}
	if d.keepRetrySec != 0 {
		t.Fatalf("empty interval did not reset keepRetrySec: %d", d.keepRetrySec)
	}
	d.maybeHeartbeat(context.Background())
	if st.IsLogged() {
		t.Fatal("next heartbeat cycle did not clear logged state")
	}
}

func TestRunLoopUsesKeepInsteadOfConnectivityProbeForLoggedSession(t *testing.T) {
	requestSeen := make(chan struct{}, 1)
	st := state.New()
	st.SetLogged(true)
	cli := netclient.New(st, "", "", "")
	cli.HTTPClient().Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		requestSeen <- struct{}{}
		return nil, errors.New("unexpected request")
	})
	sess := session.New(st, cli)
	validZSM := append([]byte{0, 0, 0, 0, 36}, []byte(tAlgo)...)
	if err := sess.Initialize(validZSM); err != nil {
		t.Fatalf("initialize session: %v", err)
	}
	d := NewDialer(st, cli, sess, config.Config{})
	d.keepURL = "http://keep.example"
	d.keepRetrySec = 480
	d.tick = time.Now()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		d.runLoop(ctx)
		close(done)
	}()

	select {
	case <-requestSeen:
		t.Fatal("logged session performed a connectivity probe before KEEP was due")
	case <-time.After(100 * time.Millisecond):
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("run loop did not stop after cancellation")
	}
}

func TestRunLoopSendsDueKeepWithoutConnectivityProbe(t *testing.T) {
	serverCipher, err := cipher.New(tAlgo)
	if err != nil {
		t.Fatal(err)
	}
	keepResponse, err := serverCipher.Encrypt("<response><interval>480</interval></response>")
	if err != nil {
		t.Fatal(err)
	}
	requests := make(chan string, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.URL.Path
		_, _ = w.Write([]byte(keepResponse))
	}))
	defer srv.Close()

	st := state.New()
	st.SetLogged(true)
	cli := netclient.New(st, "", "", "")
	sess := session.New(st, cli)
	validZSM := append([]byte{0, 0, 0, 0, 36}, []byte(tAlgo)...)
	if err := sess.Initialize(validZSM); err != nil {
		t.Fatalf("initialize session: %v", err)
	}
	d := NewDialer(st, cli, sess, config.Config{})
	d.keepURL = srv.URL + "/keep"
	d.keepRetrySec = 1
	d.tick = time.Now().Add(-2 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		d.runLoop(ctx)
		close(done)
	}()

	select {
	case path := <-requests:
		if path != "/keep" {
			t.Fatalf("first logged-session request path = %q, want /keep", path)
		}
	case <-time.After(time.Second):
		t.Fatal("due KEEP request was not sent")
	}
	select {
	case path := <-requests:
		t.Fatalf("unexpected connectivity request after KEEP: %q", path)
	case <-time.After(100 * time.Millisecond):
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("run loop did not stop after cancellation")
	}
}

func TestHeartbeatFailureLeavesConnectivitySuccessEligibleForReauthorization(t *testing.T) {
	st := state.New()
	st.SetLogged(true)
	cli := netclient.New(st, "", "", "")
	sess := session.New(st, cli)
	zsm := append([]byte{0, 0, 0, 0, 36}, []byte(tAlgo)...)
	if err := sess.Initialize(zsm); err != nil {
		t.Fatalf("init session: %v", err)
	}
	d := NewDialer(st, cli, sess, config.Config{})
	d.keepURL = "https://example.invalid/keep"
	d.termURL = "https://example.invalid/term"
	d.keepRetrySec = 1
	d.tick = time.Now().Add(-2 * time.Second)

	d.maybeHeartbeat(context.Background())
	if st.IsLogged() {
		t.Fatal("failed heartbeat left logged state set")
	}
	if !d.shouldReauthorize() {
		t.Fatal("heartbeat failure did not leave the session eligible for reauthorization")
	}
}

func TestAuthorizeKeepsRunningAfterRepeatedInitFailure(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte{0, 0, 0})
	}))
	defer srv.Close()

	st := state.New()
	st.SetTicketURL(srv.URL)
	cli := netclient.New(st, "", "", "")
	sess := session.New(st, cli)
	d := NewDialer(st, cli, sess, config.Config{SMSCode: "preset"})

	for attempt := 1; attempt <= 5; attempt++ {
		d.authorize(context.Background())
		if !st.IsRunning() {
			t.Fatalf("dialer stopped on attempt %d", attempt)
		}
	}
	d.authorize(context.Background())
	if !st.IsRunning() {
		t.Fatal("dialer stopped after repeated init failure")
	}
	if requests != 6 {
		t.Fatalf("expected 6 init attempts, got %d", requests)
	}
}

func TestBeginAuthorizationResetsCompletedSession(t *testing.T) {
	const currentUA = "CCTP/test-2093"
	const oldClientID = "old-client-id"
	const newClientID = "new-client-id"
	var initPayload, initClientID, termClientID string
	initSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read init payload: %v", err)
		}
		initPayload = string(body)
		initClientID = r.Header.Get("Client-ID")
		_, _ = w.Write([]byte("not-zsm"))
	}))
	defer initSrv.Close()
	termSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		termClientID = r.Header.Get("Client-ID")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer termSrv.Close()

	st := state.New()
	st.SetClientID(oldClientID)
	st.SetLogged(true)
	st.SetUserAgent("CCTP/test-2089")
	st.SetTicket("old-ticket")
	st.SetTicketURL(initSrv.URL)
	cli := netclient.New(st, "", "", "")
	sess := session.New(st, cli)
	validZSM := append([]byte{0, 0, 0, 0, 36}, []byte(tAlgo)...)
	if err := sess.Initialize(validZSM); err != nil {
		t.Fatalf("initialize old session: %v", err)
	}
	d := NewDialer(st, cli, sess, config.Config{SMSCode: "preset", UserAgent: currentUA})
	d.newClientID = func() (string, error) { return newClientID, nil }
	d.keepURL = "https://example.invalid/keep"
	d.termURL = termSrv.URL
	d.keepRetrySec = 480

	d.beginAuthorization(context.Background())

	if initPayload != initialAlgoID {
		t.Fatalf("init payload = %q, want fresh algo id %q", initPayload, initialAlgoID)
	}
	if termClientID != oldClientID {
		t.Fatalf("TERM Client-ID = %q, want old id %q", termClientID, oldClientID)
	}
	if initClientID != newClientID || st.ClientID() != newClientID {
		t.Fatalf("authorization Client-ID header/state = %q/%q, want %q", initClientID, st.ClientID(), newClientID)
	}
	if st.UserAgent() != currentUA {
		t.Fatalf("user agent = %q, want configured %q", st.UserAgent(), currentUA)
	}
	if st.Ticket() != "" || d.keepURL != "" || d.termURL != "" || d.keepRetrySec != 0 {
		t.Fatalf("old authorization state was retained")
	}
	if sess.IsInitialized() {
		t.Fatal("old cipher remained initialized")
	}
}

func TestBeginAuthorizationClearsNegotiationStateFromFailedAttempt(t *testing.T) {
	const newClientID = "new-client-id"
	var initPayload, initClientID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read init payload: %v", err)
		}
		initPayload = string(body)
		initClientID = r.Header.Get("Client-ID")
		_, _ = w.Write([]byte("not-zsm"))
	}))
	defer srv.Close()

	st := state.New()
	st.SetClientID("failed-attempt-client-id")
	st.SetTicket("failed-attempt-ticket")
	st.SetTicketURL(srv.URL)
	cli := netclient.New(st, "", "", "")
	sess := session.New(st, cli)
	validZSM := append([]byte{0, 0, 0, 0, 36}, []byte(tAlgo)...)
	if err := sess.Initialize(validZSM); err != nil {
		t.Fatalf("initialize failed-attempt session: %v", err)
	}
	d := NewDialer(st, cli, sess, config.Config{SMSCode: "preset"})
	d.newClientID = func() (string, error) { return newClientID, nil }

	d.beginAuthorization(context.Background())

	if initPayload != initialAlgoID {
		t.Fatalf("retry init payload = %q, want reset algo id %q", initPayload, initialAlgoID)
	}
	if initClientID != newClientID || st.ClientID() != newClientID {
		t.Fatalf("retry Client-ID header/state = %q/%q, want %q", initClientID, st.ClientID(), newClientID)
	}
	if st.Ticket() != "" {
		t.Fatalf("failed-attempt ticket was retained: %q", st.Ticket())
	}
	if sess.IsInitialized() {
		t.Fatal("failed-attempt cipher remained initialized")
	}
}

func TestBeginAuthorizationStopsBeforeRequestWhenClientIDRefreshFails(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	st := state.New()
	st.SetClientID("old-client-id")
	st.SetTicketURL(srv.URL)
	cli := netclient.New(st, "", "", "")
	d := NewDialer(st, cli, session.New(st, cli), config.Config{SMSCode: "preset"})
	d.newClientID = func() (string, error) { return "", errors.New("entropy unavailable") }

	d.beginAuthorization(context.Background())

	if requests != 0 {
		t.Fatalf("authorization continued after Client-ID refresh failure: requests=%d", requests)
	}
	if st.ClientID() != "old-client-id" {
		t.Fatalf("failed refresh changed Client-ID to %q", st.ClientID())
	}
	if d.retryCount != 1 {
		t.Fatalf("retry count = %d, want 1", d.retryCount)
	}
}

func TestLoginRejectsMissingKeepURLWithSanitizedServerFields(t *testing.T) {
	serverCipher, err := cipher.New(tAlgo)
	if err != nil {
		t.Fatal(err)
	}
	plain := `<response><error-code>63022</error-code><message>E63022: 在线用户数量限制</message><userid>sensitive-account</userid></response>`
	encrypted, err := serverCipher.Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(encrypted))
	}))
	defer srv.Close()

	st := state.New()
	st.SetClientID(tClientID)
	st.SetAuthURL(srv.URL)
	st.SetTicket("test-ticket")
	cli := netclient.New(st, "", "", "")
	sess := session.New(st, cli)
	validZSM := append([]byte{0, 0, 0, 0, 36}, []byte(tAlgo)...)
	if err := sess.Initialize(validZSM); err != nil {
		t.Fatalf("initialize session: %v", err)
	}
	d := NewDialer(st, cli, sess, config.Config{User: "account", Password: "password"})

	err = d.login(context.Background(), "")
	if err == nil {
		t.Fatal("login accepted response without keep-url")
	}
	message := err.Error()
	if !strings.Contains(message, "error-code=63022") || !strings.Contains(message, "在线用户数量限制") {
		t.Fatalf("login error omitted safe server fields: %v", err)
	}
	if strings.Contains(message, "sensitive-account") || strings.Contains(message, "test-ticket") {
		t.Fatalf("login error leaked non-diagnostic response data: %v", err)
	}
	if d.keepURL != "" || d.termURL != "" || d.keepRetrySec != 0 {
		t.Fatalf("rejected login retained keep state")
	}
	if !st.IsRunning() {
		t.Fatal("rejected login stopped the dialer instead of allowing backoff retry")
	}
}

func TestSummarizeLoginFailureWithoutKnownFieldsUsesDigest(t *testing.T) {
	err := summarizeLoginFailure(`<response><userid>sensitive-account</userid></response>`)
	if err == nil {
		t.Fatal("missing keep-url response was accepted")
	}
	message := err.Error()
	if !strings.Contains(message, "bytes=") || !strings.Contains(message, "sha256=") {
		t.Fatalf("digest metadata missing: %v", err)
	}
	if strings.Contains(message, "sensitive-account") {
		t.Fatalf("unknown response data leaked: %v", err)
	}
}

func TestAuthRetryDelayCapsAtThirtySeconds(t *testing.T) {
	if got := authRetryDelay(1); got != time.Second {
		t.Fatalf("first retry delay = %v, want 1s", got)
	}
	if got := authRetryDelay(6); got != 30*time.Second {
		t.Fatalf("sixth retry delay = %v, want 30s cap", got)
	}
	if got := authRetryDelay(7); got != 30*time.Second {
		t.Fatalf("capped retry delay = %v, want 30s", got)
	}
}

func TestInitSessionRetriesWithLegacyUserAgent(t *testing.T) {
	const currentUA = "CCTP/test-2093"
	const legacyUA = "CCTP/test-2089"
	var uas []string
	validZSM := append([]byte{0, 0, 0, 0, 36}, []byte(tAlgo)...)
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		uas = append(uas, r.Header.Get("User-Agent"))
		if requests == 1 {
			_, _ = w.Write([]byte("not-zsm"))
			return
		}
		_, _ = w.Write(validZSM)
	}))
	defer srv.Close()

	st := state.New()
	st.SetUserAgent(currentUA)
	st.SetAlgoID("00000000-0000-0000-0000-000000000000")
	st.SetTicketURL(srv.URL)
	cli := netclient.New(st, "", "", "")
	sess := session.New(st, cli)
	d := NewDialer(st, cli, sess, config.Config{LegacyUserAgent: legacyUA})
	if err := d.initSession(context.Background()); err != nil {
		t.Fatalf("initSession: %v", err)
	}
	if requests != 2 || len(uas) != 2 || uas[0] != currentUA || uas[1] != legacyUA {
		t.Fatalf("unexpected retry sequence: requests=%d uas=%v", requests, uas)
	}
	if st.UserAgent() != legacyUA || !sess.IsInitialized() {
		t.Fatalf("legacy session state not retained: ua=%q initialized=%v", st.UserAgent(), sess.IsInitialized())
	}
}
