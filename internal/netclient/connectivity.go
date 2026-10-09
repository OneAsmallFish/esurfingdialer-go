package netclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"esurfingdialer/internal/utils"
)

type ConnectivityStatus int

const (
	ConnectivitySuccess ConnectivityStatus = iota
	ConnectivityRequireAuthorization
	ConnectivityRequirePortalLogin
	ConnectivityRequestError
)

const (
	captiveURL   = "http://connect.rom.miui.com/generate_204"
	portalStart  = "<!--//config.campus.js.chinatelecom.com"
	portalEnd    = "//config.campus.js.chinatelecom.com-->"
	headerArea   = "area"
	headerSchool = "schoolid"
	headerDomain = "domain"
	authKey      = "Eshore!@#"
)

func (c *Client) DetectConnectivity(ctx context.Context) (ConnectivityStatus, error) {
	// 访问 captiveURL 并手动跟随重定向，解析校园网配置与必要参数
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.captiveURL, nil)
	if err != nil {
		return ConnectivityRequestError, err
	}
	req.Header.Set("User-Agent", c.userAgent())
	req.Header.Set("Accept", requestAccept)
	req.Header.Set("Client-ID", c.st.ClientID())

	resp, err := c.followRedirects(req, 5)
	if err != nil {
		return ConnectivityRequestError, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		closeRedirectBody(resp.Body)
		return ConnectivityRequestError, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	body, err := readResponseBody(resp.Body)
	if err != nil {
		return ConnectivityRequestError, err
	}
	logResponseMetadata(resp, body)

	portalConfig := utils.ExtractBetweenTags(string(body), portalStart, portalEnd)
	if portalConfig == "" {
		if isStandalonePortal(resp, body) || isPortalEndpoint(resp) {
			// 不把独立 Portal 页面误报为已联网；由上层决定是否自动提交凭据。
			c.st.SetAuthURL("")
			c.st.SetTicketURL("")
			return ConnectivityRequirePortalLogin, nil
		}
		return ConnectivitySuccess, nil
	}

	authURL, ticketURL, funcfg, err := parsePortalConfig(portalConfig)
	if err != nil {
		return ConnectivityRequestError, err
	}
	if authURL == "" || ticketURL == "" {
		return ConnectivityRequestError, errors.New("missing auth-url or ticket-url")
	}

	c.st.SetAuthURL(authURL)
	c.st.SetTicketURL(ticketURL)
	for k, v := range funcfg {
		c.st.SetExtraCfgURLItem(k, v)
	}

	parsed, err := url.Parse(ticketURL)
	if err != nil {
		return ConnectivityRequestError, err
	}
	userIP := parsed.Query().Get("wlanuserip")
	acIP := parsed.Query().Get("wlanacip")
	if userIP == "" || acIP == "" {
		return ConnectivityRequestError, errors.New("missing wlanuserip or wlanacip")
	}

	c.st.SetUserIP(userIP)
	c.st.SetAcIP(acIP)

	return ConnectivityRequireAuthorization, nil
}

// DetectStandalonePortal fetches the configured H3C portal and reports
// whether it looks like a login page. It performs no credential submission.
func (c *Client) DetectStandalonePortal(ctx context.Context) (bool, error) {
	if c.portalURL == "" {
		return false, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.portalURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", c.userAgent())
	req.Header.Set("Accept", requestAccept)
	resp, err := c.followRedirects(req, 5)
	if err != nil {
		return false, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		closeRedirectBody(resp.Body)
		return false, fmt.Errorf("portal page status: %d", resp.StatusCode)
	}
	body, err := readResponseBody(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return false, err
	}
	return isStandalonePortal(resp, body), nil
}

func isStandalonePortal(resp *http.Response, body []byte) bool {
	text := strings.ToLower(string(body))
	// The endpoint can return an empty 200 when no browser portal context is
	// active. Only a real form proves that credential submission is possible.
	if isPortalEndpoint(resp) {
		return strings.Contains(text, "id_username") ||
			strings.Contains(text, "id_userName") ||
			strings.Contains(text, "/portal/pws")
	}
	// H3C Portal 的页面可能由 captive redirect 直接返回，URL 不一定保留内网地址。
	return strings.Contains(text, "/portal/pws") &&
		(strings.Contains(text, "id_userName") || strings.Contains(text, "id_username") || strings.Contains(text, "username"))
}

func isPortalEndpoint(resp *http.Response) bool {
	if resp == nil || resp.Request == nil || resp.Request.URL == nil {
		return false
	}
	host := strings.ToLower(resp.Request.URL.Hostname())
	path := strings.ToLower(resp.Request.URL.Path)
	return host == "172.17.18.2" && strings.HasPrefix(path, "/portal")
}

func (c *Client) followRedirects(req *http.Request, max int) (*http.Response, error) {
	return c.followRedirectsWithReferrer(req, max, false)
}

func (c *Client) followRedirectsWithReferrer(req *http.Request, max int, setReferrer bool) (*http.Response, error) {
	// 手动跟随 302，收集 CDC-* 头并继续请求
	cur := req
	for i := 0; i <= max; i++ {
		resp, err := c.hc.Do(cur)
		if err != nil {
			return nil, err
		}

		c.updateRedirectHeaders(resp)

		if resp.StatusCode < 300 || resp.StatusCode >= 400 {
			return resp, nil
		}
		if i == max {
			return resp, nil
		}

		loc := resp.Header.Get("Location")
		if loc == "" {
			return resp, nil
		}

		nextURL, err := cur.URL.Parse(loc)
		if err != nil {
			closeRedirectBody(resp.Body)
			return nil, err
		}

		closeRedirectBody(resp.Body)
		previousURL := cur.URL.String()
		headers := cur.Header.Clone()
		cur, err = http.NewRequestWithContext(cur.Context(), http.MethodGet, nextURL.String(), nil)
		if err != nil {
			return nil, err
		}
		cur.Header = headers
		if cur.Header.Get("User-Agent") == "" {
			cur.Header.Set("User-Agent", c.userAgent())
		}
		if cur.Header.Get("Accept") == "" {
			cur.Header.Set("Accept", requestAccept)
		}
		if setReferrer {
			cur.Header.Set("Referer", previousURL)
		}
		cur.Header.Set("Client-ID", c.st.ClientID())
		c.applyCDCHeaders(cur)
	}

	return nil, errors.New("redirect loop exceeded")
}

func (c *Client) updateRedirectHeaders(resp *http.Response) {
	// 从响应头中提取 area/schoolid/domain
	area := strings.TrimSpace(resp.Header.Get(headerArea))
	school := strings.TrimSpace(resp.Header.Get(headerSchool))
	domain := strings.TrimSpace(resp.Header.Get(headerDomain))

	if area != "" {
		c.st.SetArea(area)
	}
	if school != "" {
		c.st.SetSchoolID(school)
	}
	if domain != "" {
		c.st.SetDomain(domain)
	}
}

func (c *Client) applyCDCHeaders(req *http.Request) {
	if v := c.st.SchoolID(); v != "" {
		req.Header.Set("CDC-SchoolId", v)
	}
	if v := c.st.Domain(); v != "" {
		req.Header.Set("CDC-Domain", v)
	}
	if v := c.st.Area(); v != "" {
		req.Header.Set("CDC-Area", v)
	}
}
