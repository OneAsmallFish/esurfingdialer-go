package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"esurfingdialer/internal/config"
	"esurfingdialer/internal/netclient"
	"esurfingdialer/internal/session"
	"esurfingdialer/internal/state"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestShutdownStopsOnlyAfterTermCompletes(t *testing.T) {
	requestStarted := make(chan struct{})
	allowResponse := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-allowResponse
		w.WriteHeader(http.StatusOK)
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
	d.termURL = srv.URL
	shutdownReturned := make(chan struct{})
	go func() {
		d.shutdown(context.Background())
		close(shutdownReturned)
	}()

	<-requestStarted
	select {
	case <-st.Done():
		t.Fatal("state stopped before term completed")
	default:
	}

	close(allowResponse)
	<-shutdownReturned
	select {
	case <-st.Done():
	default:
		t.Fatal("state did not stop after term completed")
	}
}

func TestShutdownTermsInitializedSessionWhenLocalLoggedBitIsFalse(t *testing.T) {
	requestSeen := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestSeen)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	st := state.New()
	st.SetClientID(tClientID)
	st.SetLogged(false)
	cli := netclient.New(st, "", "", "")
	sess := session.New(st, cli)
	zsm := append([]byte{0, 0, 0, 0, 36}, []byte(tAlgo)...)
	if err := sess.Initialize(zsm); err != nil {
		t.Fatalf("init session: %v", err)
	}

	d := NewDialer(st, cli, sess, config.Config{})
	d.termURL = srv.URL
	d.shutdown(context.Background())

	select {
	case <-requestSeen:
	default:
		t.Fatal("shutdown skipped TERM when local logged bit was false")
	}
}

func TestShutdownHookCancelsActiveRequestBeforeTerm(t *testing.T) {
	requestStarted := make(chan struct{})
	requestCanceled := make(chan struct{})
	st := state.New()
	cli := netclient.New(st, "", "", "")
	cli.HTTPClient().Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		close(requestStarted)
		<-req.Context().Done()
		close(requestCanceled)
		return nil, req.Context().Err()
	})
	d := NewDialer(st, cli, session.New(st, cli), config.Config{PortalURL: "http://portal.example"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signals := make(chan os.Signal, 1)
	d.Start(ctx)
	d.StartShutdownHook(ctx, signals, cancel)
	<-requestStarted
	signals <- syscall.SIGTERM

	select {
	case <-requestCanceled:
	case <-time.After(time.Second):
		t.Fatal("shutdown signal did not cancel the active request")
	}
	select {
	case <-st.Done():
	case <-time.After(time.Second):
		t.Fatal("shutdown signal did not stop the dialer")
	}
	d.Wait()
}

func TestRunLoopDoesNotAuthorizeAfterShutdown(t *testing.T) {
	requestStarted := make(chan struct{})
	allowResponse := make(chan struct{})
	var requests atomic.Int32
	portal := `<!--//config.campus.js.chinatelecom.com
<auth-url>http://auth.example</auth-url>
<ticket-url>http://ticket.example?wlanuserip=1.2.3.4&amp;wlanacip=5.6.7.8</ticket-url>
//config.campus.js.chinatelecom.com-->`

	st := state.New()
	cli := netclient.New(st, "", "", "")
	cli.HTTPClient().Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if requests.Add(1) == 1 {
			close(requestStarted)
			<-allowResponse
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(portal)),
			Request:    req,
		}, nil
	})
	sess := session.New(st, cli)
	d := NewDialer(st, cli, sess, config.Config{SMSCode: "preset"})

	runReturned := make(chan struct{})
	go func() {
		d.runLoop(context.Background())
		close(runReturned)
	}()
	<-requestStarted
	d.shutdown(context.Background())
	close(allowResponse)
	<-runReturned

	if got := requests.Load(); got != 1 {
		t.Fatalf("shutdown was followed by %d unexpected request(s)", got-1)
	}
}

func TestRunLoopCancellationClosesDone(t *testing.T) {
	requestSeen := make(chan struct{}, 1)
	st := state.New()
	cli := netclient.New(st, "", "", "")
	cli.HTTPClient().Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		select {
		case requestSeen <- struct{}{}:
		default:
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    req,
		}, nil
	})
	d := NewDialer(st, cli, session.New(st, cli), config.Config{})
	ctx, cancel := context.WithCancel(context.Background())
	go d.runLoop(ctx)
	<-requestSeen
	cancel()

	select {
	case <-st.Done():
	case <-time.After(time.Second):
		t.Fatal("context cancellation did not stop the dialer")
	}
}
