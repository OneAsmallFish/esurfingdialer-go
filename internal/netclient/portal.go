package netclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	portalUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"
	portalAccept    = "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"
)

type portalConfig struct {
	AuthURL   string
	TicketURL string
	Funcfg    map[string]string
}

func parsePortalConfig(raw string) (string, string, map[string]string, error) {
	cfg := portalConfig{Funcfg: make(map[string]string)}

	authRe := regexp.MustCompile(`(?is)<auth-url>(.*?)</auth-url>`)
	ticketRe := regexp.MustCompile(`(?is)<ticket-url>(.*?)</ticket-url>`)
	funcfgRe := regexp.MustCompile(`(?is)<funcfg[^>]*>(.*?)</funcfg>`)
	itemRe := regexp.MustCompile(`(?is)<([A-Za-z0-9_-]+)\b([^>]*)>`)
	attrRe := regexp.MustCompile(`(?is)\b(enable|url)\s*=\s*(?:"([^"]*)"|'([^']*)')`)

	if m := authRe.FindStringSubmatch(raw); len(m) == 2 {
		cfg.AuthURL = cleanPortalValue(m[1])
	}
	if m := ticketRe.FindStringSubmatch(raw); len(m) == 2 {
		cfg.TicketURL = cleanPortalValue(m[1])
	}
	if m := funcfgRe.FindStringSubmatch(raw); len(m) == 2 {
		block := m[1]
		for _, item := range itemRe.FindAllStringSubmatch(block, -1) {
			if len(item) != 3 {
				continue
			}
			tag := strings.TrimSpace(item[1])
			attrs := make(map[string]string)
			for _, attr := range attrRe.FindAllStringSubmatch(item[2], -1) {
				value := attr[2]
				if value == "" {
					value = attr[3]
				}
				attrs[strings.ToLower(attr[1])] = value
			}
			url := cleanPortalValue(attrs["url"])
			if tag != "" && attrs["enable"] == "1" && url != "" {
				cfg.Funcfg[tag] = url
			}
		}
	}

	return cfg.AuthURL, cfg.TicketURL, cfg.Funcfg, nil
}

func cleanPortalValue(val string) string {
	val = strings.TrimSpace(html.UnescapeString(val))
	val = strings.TrimPrefix(val, "<![CDATA[")
	val = strings.TrimSuffix(val, "]]>")
	return strings.TrimSpace(val)
}

// portalLoginResponse is the JSON returned by /portal/pws?t=li. The portal
// wraps URL-encoded JSON in Base64 and may omit padding, so it is decoded by
// decodePortalResponse instead of handing the response directly to json.Decoder.
type portalLoginResponse struct {
	ErrorNumber         string `json:"errorNumber"`
	PortalLink          string `json:"portalLink"`
	ErrorCodeKey        string `json:"e_c"`
	ErrorDescriptionKey string `json:"e_d"`
	ErrorCode           string `json:"-"`
	ErrorDescription    string `json:"-"`
}

// PortalLogin performs the standalone H3C Portal flow captured from the
// browser: GET the page for its cookie and hidden fields, POST /pws, then
// request afterLogin.jsp when the portal supplies a continuation link.
func (c *Client) PortalLogin(ctx context.Context, rawPortalURL, user, password, userIP, mac string) error {
	if strings.TrimSpace(rawPortalURL) == "" {
		return errors.New("portal URL is empty")
	}
	configuredURL, err := url.Parse(rawPortalURL)
	if err != nil || configuredURL.Scheme == "" || configuredURL.Host == "" {
		return fmt.Errorf("invalid portal URL")
	}
	entryURL, page, err := c.fetchPortalPage(ctx, configuredURL)
	if err != nil {
		return err
	}
	base := *entryURL
	base.Path = strings.TrimSuffix(base.Path, "/") + "/"
	base.RawQuery = ""
	base.Fragment = ""
	entryReferrer := entryURL.String()

	form := parsePortalHiddenFields(string(page))
	form.Set("userName", strings.TrimSpace(user))
	form.Set("userPwd", base64.StdEncoding.EncodeToString([]byte(password)))
	if userIP != "" {
		form.Set("userip", userIP)
	}
	if mac != "" {
		form.Set("usermac", mac)
	}
	if form.Get("language") == "" {
		form.Set("language", "Chinese")
	}
	if form.Get("pwdMode") == "" {
		form.Set("pwdMode", "0")
	}
	if form.Get("dcPwdNeedEncrypt") == "" {
		form.Set("dcPwdNeedEncrypt", "1")
	}

	pwsURL := base
	pwsURL.Path = strings.TrimSuffix(base.Path, "/") + "/pws"
	query := pwsURL.Query()
	query.Set("t", "li")
	query.Set("ifEmailAuth", "false")
	pwsURL.RawQuery = query.Encode()
	postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, pwsURL.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	setPortalHeaders(postReq)
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	postReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	postReq.Header.Set("Referer", entryReferrer)
	resp, err := c.hc.Do(postReq)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		closeRedirectBody(resp.Body)
		return fmt.Errorf("portal login status: %d", resp.StatusCode)
	}
	data, err := readResponseBody(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return err
	}
	result, err := decodePortalResponse(data)
	if err != nil {
		return err
	}
	if result.ErrorNumber != "1" {
		if result.ErrorNumber == "" {
			return errors.New("portal login returned no result")
		}
		details := make([]string, 0, 2)
		if result.ErrorCode != "" {
			details = append(details, "code "+result.ErrorCode)
		}
		if result.ErrorDescription != "" {
			details = append(details, result.ErrorDescription)
		}
		if len(details) == 0 {
			return fmt.Errorf("portal login rejected (error number %s)", result.ErrorNumber)
		}
		return fmt.Errorf("portal login rejected (error number %s: %s)", result.ErrorNumber, strings.Join(details, "; "))
	}

	if result.PortalLink != "" {
		continuation := base
		continuation.Path = strings.TrimSuffix(base.Path, "/") + "/page/afterLogin.jsp"
		q := continuation.Query()
		q.Set("pl", result.PortalLink)
		q.Set("custompath", "")
		q.Set("uamInitCustom", "0")
		q.Set("uamInitLogo", "H3C")
		q.Set("loginType", "3")
		q.Set("v_is_selfLogin", "0")
		q.Set("byodserverip", "")
		q.Set("byodserverhttpport", "8080")
		continuation.RawQuery = q.Encode()
		followReq, err := http.NewRequestWithContext(ctx, http.MethodGet, continuation.String(), nil)
		if err != nil {
			return err
		}
		setPortalHeaders(followReq)
		followReq.Header.Set("Referer", entryReferrer)
		followResp, err := c.hc.Do(followReq)
		if err != nil {
			return err
		}
		if followResp.StatusCode < 200 || followResp.StatusCode >= 300 {
			closeRedirectBody(followResp.Body)
			return fmt.Errorf("portal continuation status: %d", followResp.StatusCode)
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(followResp.Body, maxResponseBodyBytes))
		_ = followResp.Body.Close()
	}
	return nil
}

func (c *Client) fetchPortalPage(ctx context.Context, configuredURL *url.URL) (*url.URL, []byte, error) {
	candidates := []string{c.captiveURL, configuredURL.String()}
	var lastErr error
	for i, candidate := range candidates {
		if candidate == "" || (i > 0 && candidate == candidates[0]) {
			continue
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate, nil)
		if err != nil {
			return nil, nil, err
		}
		setPortalHeaders(req)
		resp, err := c.followRedirectsWithReferrer(req, 5, true)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			closeRedirectBody(resp.Body)
			lastErr = fmt.Errorf("portal page status: %d", resp.StatusCode)
			continue
		}
		page, readErr := readResponseBody(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if isStandalonePortal(resp, page) {
			return resp.Request.URL, page, nil
		}
		lastErr = errors.New("portal page does not contain a login form")
	}
	if lastErr == nil {
		lastErr = errors.New("portal page is unavailable")
	}
	return nil, nil, lastErr
}

func setPortalHeaders(req *http.Request) {
	req.Header.Set("User-Agent", portalUserAgent)
	req.Header.Set("Accept", portalAccept)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
}

func (c *Client) userAgent() string {
	if c.st != nil {
		if value := c.st.UserAgent(); value != "" {
			return value
		}
	}
	return userAgent
}

func parsePortalHiddenFields(raw string) url.Values {
	values := make(url.Values)
	inputRe := regexp.MustCompile(`(?is)<input\b[^>]*>`)
	attrRe := regexp.MustCompile(`(?is)([a-zA-Z_:][a-zA-Z0-9_.:-]*)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>` + "`" + `]+))`)
	for _, tag := range inputRe.FindAllString(raw, -1) {
		attrs := make(map[string]string)
		for _, match := range attrRe.FindAllStringSubmatch(tag, -1) {
			value := match[2]
			if value == "" {
				value = match[3]
			}
			if value == "" {
				value = match[4]
			}
			attrs[strings.ToLower(match[1])] = html.UnescapeString(value)
		}
		name := strings.TrimSpace(attrs["name"])
		if name != "" {
			values.Add(name, attrs["value"])
		}
	}
	return values
}

func decodePortalResponse(data []byte) (portalLoginResponse, error) {
	encoded := strings.TrimSpace(string(data))
	if encoded == "" {
		return portalLoginResponse{}, errors.New("empty portal login response")
	}
	encoded += strings.Repeat("=", (4-len(encoded)%4)%4)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(encoded, "="))
	}
	if err != nil {
		return portalLoginResponse{}, errors.New("invalid portal login response encoding")
	}
	unescaped, err := url.QueryUnescape(string(decoded))
	if err != nil {
		return portalLoginResponse{}, errors.New("invalid portal login response escaping")
	}
	decodedJSON := []byte(unescaped)
	var result portalLoginResponse
	if err := json.Unmarshal(decodedJSON, &result); err != nil {
		return portalLoginResponse{}, errors.New("invalid portal login response JSON")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(decodedJSON, &fields); err == nil {
		result.ErrorCode = portalResponseField(fields, result.ErrorCodeKey)
		result.ErrorDescription = portalResponseField(fields, result.ErrorDescriptionKey)
	}
	return result, nil
}

func portalResponseField(fields map[string]json.RawMessage, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	raw, ok := fields[key]
	if !ok {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		value = strings.Trim(string(raw), `"`)
	}
	value = html.UnescapeString(value)
	value = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, value)
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > 160 {
		value = string(runes[:160]) + "..."
	}
	return value
}
