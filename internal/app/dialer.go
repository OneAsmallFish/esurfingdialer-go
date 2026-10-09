package app

import (
	"context"
	"os"
	"sync"
	"time"

	"esurfingdialer/internal/config"
	"esurfingdialer/internal/logger"
	"esurfingdialer/internal/netclient"
	"esurfingdialer/internal/session"
	"esurfingdialer/internal/state"
	"esurfingdialer/internal/utils"
)

const shutdownTermTimeout = 4 * time.Second

type Dialer struct {
	mu   sync.Mutex
	wg   sync.WaitGroup
	st   *state.State
	cli  *netclient.Client
	sess *session.Session
	cfg  config.Config

	newClientID func() (string, error)

	keepURL          string
	termURL          string
	keepRetrySec     int
	tick             time.Time
	retryCount       int
	portalLogged     bool
	portalAttempted  bool
	portalWaitLogged bool
}

func NewDialer(st *state.State, cli *netclient.Client, sess *session.Session, cfg config.Config) *Dialer {
	return &Dialer{
		st:          st,
		cli:         cli,
		sess:        sess,
		cfg:         cfg,
		newClientID: utils.RandomUUIDLower,
	}
}

func (d *Dialer) Start(ctx context.Context) {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.runLoop(ctx)
	}()
}

func (d *Dialer) StartShutdownHook(ctx context.Context, signals <-chan os.Signal, stop context.CancelFunc) {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.shutdownLoop(ctx, signals, stop)
	}()
}

func (d *Dialer) Wait() {
	d.wg.Wait()
}

func (d *Dialer) runLoop(ctx context.Context) {
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()
	wait := func(delay time.Duration) bool {
		timer.Reset(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			d.st.SetRunning(false)
			return false
		case <-timer.C:
			return true
		}
	}

	portalProbeDone := false
	for d.st.IsRunning() {
		select {
		case <-ctx.Done():
			d.st.SetRunning(false)
			return
		default:
		}

		// Once an authorization session is established, KEEP is the
		// authoritative liveness signal. Captive-portal probes on this network
		// can transiently time out or return a redirect even while KEEP is
		// healthy; treating those responses as an immediate logout caused
		// repeated TERM/login storms, especially with multiple sessions.
		d.mu.Lock()
		if d.st.IsLogged() && d.sess.IsInitialized() {
			d.maybeHeartbeat(ctx)
			d.mu.Unlock()
			if !wait(time.Second) {
				return
			}
			continue
		}
		d.mu.Unlock()

		if !portalProbeDone && d.cfg.PortalURL != "" {
			portalProbeDone = true
			required, portalErr := d.cli.DetectStandalonePortal(ctx)
			if portalErr != nil {
				logger.Errorf("standalone portal probe failed: %v", portalErr)
			} else if required {
				d.mu.Lock()
				d.handlePortal(ctx)
				d.mu.Unlock()
				if !wait(time.Second) {
					return
				}
				continue
			}
		}

		status, err := d.cli.DetectConnectivity(ctx)
		if err != nil {
			if ctx.Err() != nil {
				d.st.SetRunning(false)
				return
			}
			if !d.st.IsRunning() {
				return
			}
			logger.Errorf("connectivity error: %v", err)
			if !wait(5 * time.Second) {
				return
			}
			continue
		}
		if ctx.Err() != nil {
			d.st.SetRunning(false)
			return
		}
		if !d.st.IsRunning() {
			return
		}

		switch status {
		case netclient.ConnectivitySuccess:
			d.mu.Lock()
			if d.st.IsRunning() && ctx.Err() == nil {
				d.handleConnectivitySuccess(ctx)
			}
			d.mu.Unlock()
			if !wait(time.Second) {
				return
			}
		case netclient.ConnectivityRequireAuthorization:
			d.mu.Lock()
			if !d.st.IsRunning() || ctx.Err() != nil {
				d.mu.Unlock()
				return
			}
			d.beginAuthorization(ctx)
			d.mu.Unlock()
			if !wait(authRetryDelay(d.retryCount)) {
				return
			}
		case netclient.ConnectivityRequirePortalLogin:
			d.mu.Lock()
			if !d.st.IsRunning() || ctx.Err() != nil {
				d.mu.Unlock()
				return
			}
			d.st.SetLogged(false)
			d.handlePortal(ctx)
			d.mu.Unlock()
			if !wait(time.Second) {
				return
			}
		case netclient.ConnectivityRequestError:
			if !wait(5 * time.Second) {
				return
			}
		}
	}
}

func (d *Dialer) handleConnectivitySuccess(ctx context.Context) {
	if !d.sess.IsInitialized() {
		return
	}
	if d.st.IsLogged() {
		d.maybeHeartbeat(ctx)
		return
	}
	// A failed heartbeat clears the local flag while the remote session and
	// its TERM/KEEP endpoints remain available. Re-authorize once on the next
	// successful probe instead of leaving the dialer in a permanent half-state.
	if d.shouldReauthorize() {
		d.beginAuthorization(ctx)
	}
}

func (d *Dialer) shouldReauthorize() bool {
	return d.sess.IsInitialized() && !d.st.IsLogged() && (d.keepURL != "" || d.termURL != "")
}

func (d *Dialer) beginAuthorization(ctx context.Context) {
	if d.st.IsLogged() || d.keepURL != "" || d.termURL != "" {
		d.termBeforeAuthorizationReset()
	}
	// A Client-ID and its negotiated Algo-ID/cipher belong to the same
	// authorization attempt. Always clear stale negotiation state before
	// assigning a fresh Client-ID, including retries that failed before the
	// server supplied KEEP/TERM endpoints.
	d.resetAuthorizationSession()
	d.st.SetLogged(false)
	if err := d.rotateClientID(); err != nil {
		logger.Errorf("refresh Client-ID before authorization: %v", err)
		d.retryCount++
		return
	}
	d.authorize(ctx)
}

func (d *Dialer) rotateClientID() error {
	newClientID := d.newClientID
	if newClientID == nil {
		newClientID = utils.RandomUUIDLower
	}
	clientID, err := newClientID()
	if err != nil {
		return err
	}
	d.st.SetClientID(clientID)
	return nil
}

func (d *Dialer) termBeforeAuthorizationReset() {
	if !d.sess.IsInitialized() || d.termURL == "" {
		return
	}
	// Keep this best-effort cleanup independent from the worker context. A
	// shutdown signal can cancel that context in the small window just before
	// reauthorization resets the old cipher and TERM URL.
	termCtx, cancel := context.WithTimeout(context.Background(), shutdownTermTimeout)
	defer cancel()
	if err := d.term(termCtx); err != nil {
		logger.Warnf("pre-authorization term error: %v", err)
	}
}

func (d *Dialer) resetAuthorizationSession() {
	d.sess.Reset()
	d.st.SetAlgoID(initialAlgoID)
	d.st.SetTicket("")
	if d.cfg.UserAgent != "" {
		d.st.SetUserAgent(d.cfg.UserAgent)
	}
	d.keepURL = ""
	d.termURL = ""
	d.keepRetrySec = 0
	d.tick = time.Time{}
}

func authRetryDelay(retryCount int) time.Duration {
	if retryCount <= 0 {
		return time.Second
	}
	delay := time.Second << min(retryCount-1, 5)
	if delay > 30*time.Second {
		return 30 * time.Second
	}
	return delay
}

func (d *Dialer) handlePortal(ctx context.Context) {
	if !d.cfg.PortalAutoLogin {
		logger.Warnf("standalone portal detected; configure portal_auto_login to enable automatic login")
		return
	}
	if d.portalLogged {
		logger.Warnf("standalone portal is still present after the one-time automatic login")
		return
	}
	if d.portalAttempted {
		if !d.portalWaitLogged {
			d.portalWaitLogged = true
			logger.Warnf("standalone portal login was already attempted; waiting for manual recovery or restart")
		}
		return
	}
	d.portalAttempted = true
	portalUser, portalPassword := d.cfg.PortalUser, d.cfg.PortalPassword
	if portalUser == "" {
		portalUser = d.cfg.User
	}
	if portalPassword == "" {
		portalPassword = d.cfg.Password
	}
	if err := d.cli.PortalLogin(ctx, d.cfg.PortalURL, portalUser, portalPassword, d.st.UserIP(), d.cfg.PortalMAC); err != nil {
		logger.Errorf("standalone portal login failed: %v", err)
		return
	}
	d.portalLogged = true
	logger.Infof("standalone portal login completed; waiting for CCTP redirect")
}

func (d *Dialer) shutdownLoop(ctx context.Context, signals <-chan os.Signal, stop context.CancelFunc) {
	select {
	case <-ctx.Done():
		return
	case <-signals:
	}
	// Interrupt in-flight HTTP requests first.  The TERM request gets its own
	// context below, so cancellation of the worker does not cancel logout.
	if stop != nil {
		stop()
	}
	d.shutdown(context.Background())
}

func (d *Dialer) shutdown(ctx context.Context) {
	termCtx, cancel := context.WithTimeout(ctx, shutdownTermTimeout)
	defer cancel()

	d.mu.Lock()
	defer d.mu.Unlock()
	// A failed heartbeat clears the local logged bit, but the remote session
	// may still be alive.  An initialized session with a TERM URL is enough to
	// make a best-effort logout.
	if d.sess.IsInitialized() && d.termURL != "" {
		if err := d.term(termCtx); err != nil {
			logger.Errorf("term error: %v", err)
		}
	}
	d.st.SetRunning(false)
}
