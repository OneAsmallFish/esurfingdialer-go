package netclient

import (
	"crypto/md5"
	"encoding/hex"
	"net"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"strings"
	"time"

	"esurfingdialer/internal/logger"
	"esurfingdialer/internal/state"
)

const (
	userAgent     = "CCTP/android64_vpn/2093"
	requestAccept = "text/html,text/xml,application/xhtml+xml,application/x-javascript,*/*"
)

type Client struct {
	st         *state.State
	hc         *http.Client
	captiveURL string
	portalURL  string
}

// SetPortalURL configures an optional one-time standalone portal probe. It is
// kept separate from the captive connectivity URL because the two services
// have different session and cookie semantics.
func (c *Client) SetPortalURL(rawURL string) {
	c.portalURL = strings.TrimSpace(rawURL)
}

func New(st *state.State, bindInterface, bindSourceIP, bindFWMark string) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	mark := parseFWMark(bindFWMark)
	setSocketOptions(dialer, bindInterface, mark)
	bindToSourceIP(dialer, bindSourceIP)
	transport.DialContext = dialer.DialContext

	hc := &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
		Jar:       mustCookieJar(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return &Client{st: st, hc: hc, captiveURL: captiveURL}
}

func mustCookieJar() http.CookieJar {
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}
	return jar
}

func (c *Client) HTTPClient() *http.Client {
	return c.hc
}

func (c *Client) AddCommonHeaders(req *http.Request, payload string) {
	req.Header.Set("User-Agent", c.userAgent())
	req.Header.Set("Accept", requestAccept)

	sum := md5.Sum([]byte(payload))
	req.Header.Set("CDC-Checksum", hex.EncodeToString(sum[:]))
	req.Header.Set("Client-ID", sanitizeHeaderValue(c.st.ClientID()))
	req.Header.Set("Algo-ID", sanitizeHeaderValue(c.st.AlgoID()))
}

func sanitizeHeaderValue(val string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return val
	}
	return strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' {
			return -1
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, val)
}

func parseFWMark(val string) uint32 {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0
	}
	parsed, err := strconv.ParseUint(val, 0, 32)
	if err != nil {
		logger.Warnf("invalid fwmark %q, using 0: %v", val, err)
		return 0
	}
	return uint32(parsed)
}
