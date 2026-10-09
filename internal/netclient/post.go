package netclient

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"esurfingdialer/internal/logger"
)

const responseLogPrefixBytes = 64

var ErrDynamicZSMModule = errors.New("server supplied a dynamic ZSM module")

type HTTPResponse struct {
	Body        []byte
	URL         string
	StatusCode  int
	ContentType string
	ErrorCode   string
	SubError    string
}

func (c *Client) Post(ctx context.Context, url, payload string, extraHeaders map[string]string) ([]byte, error) {
	response, err := c.PostResponse(ctx, url, payload, extraHeaders)
	if err != nil {
		return nil, err
	}
	return response.Body, nil
}

// PostResponse exposes bounded response metadata so callers can distinguish a
// dynamic algorithm-module response from a normal ZSM payload.
func (c *Client) PostResponse(ctx context.Context, url, payload string, extraHeaders map[string]string) (HTTPResponse, error) {
	resp, err := c.postFollowingRedirects(ctx, url, payload, func(req *http.Request) {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		c.AddCommonHeaders(req, payload)
		for k, v := range extraHeaders {
			req.Header.Set(k, v)
		}
	})
	if err != nil {
		return HTTPResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		closeRedirectBody(resp.Body)
		return HTTPResponse{}, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	data, err := readResponseBody(resp.Body)
	if err != nil {
		return HTTPResponse{}, err
	}
	logResponseMetadata(resp, data)
	response := HTTPResponse{
		Body:        data,
		URL:         responseURL(resp),
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		ErrorCode:   strings.TrimSpace(resp.Header.Get("Error-Code")),
		SubError:    strings.TrimSpace(resp.Header.Get("Sub-Error")),
	}
	if response.ErrorCode == "200" {
		return response, ErrDynamicZSMModule
	}
	return response, nil
}

func logResponseMetadata(resp *http.Response, data []byte) {
	sum := sha256.Sum256(data)
	prefix := data
	if len(prefix) > responseLogPrefixBytes {
		prefix = prefix[:responseLogPrefixBytes]
	}
	logger.Debugf(
		"HTTP response url=%s status=%d content-type=%q error-code=%q sub-error=%q bytes=%d sha256=%x prefix-hex=%x",
		responseURL(resp),
		resp.StatusCode,
		resp.Header.Get("Content-Type"),
		resp.Header.Get("Error-Code"),
		resp.Header.Get("Sub-Error"),
		len(data),
		sum,
		prefix,
	)
}

func responseURL(resp *http.Response) string {
	if resp == nil || resp.Request == nil || resp.Request.URL == nil {
		return ""
	}
	u := resp.Request.URL
	return (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}).String()
}

func (c *Client) postFollowingRedirects(
	ctx context.Context,
	rawURL, payload string,
	setHeaders func(*http.Request),
) (*http.Response, error) {
	const maxRedirects = 5

	currentURL := rawURL
	for redirects := 0; ; redirects++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, currentURL, strings.NewReader(payload))
		if err != nil {
			return nil, err
		}
		setHeaders(req)
		if redirects > 0 {
			c.applyCDCHeaders(req)
		}
		logRequestMetadata(req, payload)

		resp, err := c.hc.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 300 || resp.StatusCode >= 400 {
			return resp, nil
		}

		c.updateRedirectHeaders(resp)
		location := resp.Header.Get("Location")
		if location == "" || redirects == maxRedirects {
			return resp, nil
		}

		nextURL, err := req.URL.Parse(location)
		if err != nil {
			closeRedirectBody(resp.Body)
			return nil, err
		}
		closeRedirectBody(resp.Body)
		currentURL = nextURL.String()
	}
}

func logRequestMetadata(req *http.Request, payload string) {
	sum := sha256.Sum256([]byte(payload))
	logger.Debugf(
		"HTTP request method=%s url=%s body-bytes=%d body-sha256=%x content-type=%q ua=%q accept=%q client-id=%x algo-id=%q cdc-checksum=%q cdc-area=%q cdc-school=%q cdc-domain=%q",
		req.Method,
		requestURL(req),
		len(payload),
		sum,
		req.Header.Get("Content-Type"),
		req.Header.Get("User-Agent"),
		req.Header.Get("Accept"),
		sha256.Sum256([]byte(req.Header.Get("Client-ID"))),
		req.Header.Get("Algo-ID"),
		req.Header.Get("CDC-Checksum"),
		req.Header.Get("CDC-Area"),
		req.Header.Get("CDC-SchoolId"),
		req.Header.Get("CDC-Domain"),
	)
}

func requestURL(req *http.Request) string {
	if req == nil || req.URL == nil {
		return ""
	}
	u := req.URL
	return (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}).String()
}
