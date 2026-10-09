package app

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"esurfingdialer/internal/logger"
	"esurfingdialer/internal/netclient"
	"esurfingdialer/internal/utils"
)

const userAgent = "CCTP/android64_vpn/2093"

func (d *Dialer) authorize(ctx context.Context) {
	// 认证主流程：短信校验 -> 初始化会话 -> 获取票据 -> 登录 -> 保活
	code := strings.TrimSpace(d.cfg.SMSCode)
	if code == "" {
		code = d.checkSMSVerify(ctx)
	}
	logger.Infof("SMS Code is: %s", code)

	if err := d.initSession(ctx); err != nil {
		if d.retryCount >= 5 {
			logger.Errorf("session init failed: %v", err)
			logger.Errorf("session initialization is still failing; will keep running and retry with backoff")
		}
		d.retryCount++
		return
	}
	if !d.sess.IsInitialized() {
		d.retryCount++
		return
	}

	logger.Infof("Algo Id: %s", d.st.AlgoID())
	logger.Infof("Client IP: %s", d.st.UserIP())
	logger.Infof("AC IP: %s", d.st.AcIP())

	ticket, err := d.getTicket(ctx)
	if err != nil {
		logger.Errorf("get ticket error: %v", err)
		d.retryCount++
		return
	}
	d.st.SetTicket(ticket)
	logger.Infof("Ticket: %s", d.st.Ticket())

	if err := d.login(ctx, code); err != nil {
		logger.Errorf("login error: %v", err)
		d.retryCount++
		return
	}

	d.retryCount = 0
	d.tick = time.Now()
	d.st.SetLogged(true)
	logger.Infof("The login has been authorized.")
}

func (d *Dialer) initSession(ctx context.Context) error {
	// 票据接口先提交 algoId，服务端返回 zsm 决定算法实现
	payload := d.st.AlgoID()
	response, err := d.cli.PostResponse(ctx, d.st.TicketURL(), payload, nil)
	if errors.Is(err, netclient.ErrDynamicZSMModule) {
		if dynamicErr := d.sess.InitializeDynamicModule(response.Body); dynamicErr != nil {
			logger.Errorf("dynamic ZSM module initialization failed: error-code=%s bytes=%d: %v", response.ErrorCode, len(response.Body), dynamicErr)
			return dynamicErr
		}
		logger.Infof("initialized dynamic ZSM module: algo-id=%s bytes=%d", d.st.AlgoID(), len(response.Body))
		return nil
	}
	if err != nil {
		return err
	}
	if err := d.sess.Initialize(response.Body); err == nil {
		return nil
	} else if d.cfg.LegacyUserAgent == "" || d.cfg.LegacyUserAgent == d.st.UserAgent() {
		return err
	} else {
		// 返校实测中 1.7(2089) 与 1.8/1.9.4(2093) 收到的会话模块不同。
		// 仅在当前 ZSM 无法初始化时重试一次旧 UA，避免改变正常路径。
		currentUA := d.st.UserAgent()
		logger.Warnf("session init failed with user-agent %s; retrying legacy user-agent %s", currentUA, d.cfg.LegacyUserAgent)
		d.st.SetUserAgent(d.cfg.LegacyUserAgent)
		legacyResponse, postErr := d.cli.PostResponse(ctx, d.st.TicketURL(), payload, nil)
		if errors.Is(postErr, netclient.ErrDynamicZSMModule) {
			if dynamicErr := d.sess.InitializeDynamicModule(legacyResponse.Body); dynamicErr != nil {
				logger.Errorf("legacy dynamic ZSM module initialization failed: error-code=%s bytes=%d: %v", legacyResponse.ErrorCode, len(legacyResponse.Body), dynamicErr)
				d.st.SetUserAgent(currentUA)
				return dynamicErr
			}
			logger.Infof("initialized legacy dynamic ZSM module: algo-id=%s bytes=%d", d.st.AlgoID(), len(legacyResponse.Body))
			return nil
		}
		if postErr != nil {
			d.st.SetUserAgent(currentUA)
			return err
		}
		if legacyErr := d.sess.Initialize(legacyResponse.Body); legacyErr != nil {
			d.st.SetUserAgent(currentUA)
			return legacyErr
		}
		logger.Infof("session initialized with legacy user-agent %s", d.cfg.LegacyUserAgent)
		return nil
	}
}

func (d *Dialer) getTicket(ctx context.Context) (string, error) {
	// 获取 ticket，payload 需加密后提交
	payload := fmt.Sprintf(
		"<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<request>\n    <user-agent>%s</user-agent>\n    <client-id>%s</client-id>\n    <local-time>%s</local-time>\n    <host-name>%s</host-name>\n    <ipv4>%s</ipv4>\n    <ipv6></ipv6>\n    <mac>%s</mac>\n    <ostag>%s</ostag>\n    <gwip>%s</gwip>\n</request>",
		d.userAgent(),
		d.st.ClientID(),
		utils.NowString(),
		d.st.HostName(),
		d.st.UserIP(),
		d.st.Mac(),
		d.st.HostName(),
		d.st.AcIP(),
	)

	enc, err := d.sess.Encrypt(payload)
	if err != nil {
		return "", err
	}
	data, err := d.cli.Post(ctx, d.st.TicketURL(), enc, nil)
	if err != nil {
		return "", err
	}
	dec, err := d.sess.Decrypt(string(data))
	if err != nil {
		return "", err
	}
	return extractTag(dec, "ticket"), nil
}

func (d *Dialer) login(ctx context.Context, code string) error {
	// 登录并解析 keep-url/term-url/keep-retry
	verify := ""
	if strings.TrimSpace(code) != "" {
		verify = fmt.Sprintf("<verify>%s</verify>", code)
	}
	payload := fmt.Sprintf(
		"<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<request>\n    <user-agent>%s</user-agent>\n    <client-id>%s</client-id>\n    <ticket>%s</ticket>\n    <local-time>%s</local-time>\n    <userid>%s</userid>\n    <passwd>%s</passwd>\n    %s\n</request>",
		d.userAgent(),
		d.st.ClientID(),
		d.st.Ticket(),
		utils.NowString(),
		d.cfg.User,
		d.cfg.Password,
		verify,
	)

	enc, err := d.sess.Encrypt(payload)
	if err != nil {
		return err
	}
	data, err := d.cli.Post(ctx, d.st.AuthURL(), enc, nil)
	if err != nil {
		return err
	}
	dec, err := d.sess.Decrypt(string(data))
	if err != nil {
		return err
	}

	d.keepURL = extractTag(dec, "keep-url")
	d.termURL = extractTag(dec, "term-url")
	keepRetry := extractTag(dec, "keep-retry")
	if d.keepURL == "" {
		d.termURL = ""
		d.keepRetrySec = 0
		return summarizeLoginFailure(dec)
	}

	d.keepRetrySec = 0
	if keepRetry != "" {
		if val, err := strconv.Atoi(strings.TrimSpace(keepRetry)); err == nil {
			d.keepRetrySec = val
		}
	}

	logger.Infof("Keep Url: %s", d.keepURL)
	logger.Infof("Term Url: %s", d.termURL)
	logger.Infof("Keep Retry: %s", keepRetry)
	return nil
}

func (d *Dialer) maybeHeartbeat(ctx context.Context) {
	// 依据 keepRetrySec 触发保活
	if d.keepURL == "" || d.keepRetrySec <= 0 {
		d.st.SetLogged(false)
		return
	}
	if time.Since(d.tick) < time.Duration(d.keepRetrySec)*time.Second {
		return
	}
	if err := d.heartbeat(ctx, d.st.Ticket()); err != nil {
		logger.Errorf("heartbeat error: %v", err)
		d.st.SetLogged(false)
		return
	}
	logger.Infof("Send Keep Packet")
	logger.Infof("Next Retry: %d", d.keepRetrySec)
	d.tick = time.Now()
}

func (d *Dialer) heartbeat(ctx context.Context, ticket string) error {
	// 发送保活并更新间隔
	payload := fmt.Sprintf(
		"<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<request>\n    <user-agent>%s</user-agent>\n    <client-id>%s</client-id>\n    <local-time>%s</local-time>\n    <host-name>%s</host-name>\n    <ipv4>%s</ipv4>\n    <ticket>%s</ticket>\n    <ipv6></ipv6>\n    <mac>%s</mac>\n    <ostag>%s</ostag>\n</request>",
		d.userAgent(),
		d.st.ClientID(),
		utils.NowString(),
		d.st.HostName(),
		d.st.UserIP(),
		ticket,
		d.st.Mac(),
		d.st.HostName(),
	)

	enc, err := d.sess.Encrypt(payload)
	if err != nil {
		return err
	}
	data, err := d.cli.Post(ctx, d.keepURL, enc, nil)
	if err != nil {
		return err
	}
	dec, err := d.sess.Decrypt(string(data))
	if err != nil {
		return err
	}
	interval := extractTag(dec, "interval")
	d.keepRetrySec = 0
	if interval != "" {
		if val, err := strconv.Atoi(strings.TrimSpace(interval)); err == nil {
			d.keepRetrySec = val
		}
	}
	return nil
}

func (d *Dialer) term(ctx context.Context) error {
	// 退出时下线
	if d.termURL == "" {
		return nil
	}
	payload := fmt.Sprintf(
		"<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<request>\n    <user-agent>%s</user-agent>\n    <client-id>%s</client-id>\n    <local-time>%s</local-time>\n    <host-name>%s</host-name>\n    <ipv4>%s</ipv4>\n    <ticket>%s</ticket>\n    <ipv6></ipv6>\n    <mac>%s</mac>\n    <ostag>%s</ostag>\n</request>",
		d.userAgent(),
		d.st.ClientID(),
		utils.NowString(),
		d.st.HostName(),
		d.st.UserIP(),
		d.st.Ticket(),
		d.st.Mac(),
		d.st.HostName(),
	)

	enc, err := d.sess.Encrypt(payload)
	if err != nil {
		return err
	}
	_, err = d.cli.Post(ctx, d.termURL, enc, nil)
	return err
}

func (d *Dialer) userAgent() string {
	if value := d.st.UserAgent(); value != "" {
		return value
	}
	return userAgent
}

func (d *Dialer) checkSMSVerify(ctx context.Context) string {
	if !d.cli.CheckVerifyCodeStatus(ctx, d.cfg.User) {
		return ""
	}
	if !d.cli.GetVerifyCode(ctx, d.cfg.User) {
		return ""
	}

	logger.Infof("This login requires a SMS verification code.")
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Input Code: ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return ""
		}
		code := strings.TrimSpace(line)
		if code != "" {
			return code
		}
	}
}

func summarizeLoginFailure(response string) error {
	tags := []string{
		"error-code",
		"errorcode",
		"code",
		"result-code",
		"result",
		"message",
		"error-message",
		"description",
		"reason",
	}
	seen := make(map[string]struct{}, len(tags))
	fields := make([]string, 0, len(tags))
	for _, tag := range tags {
		value := sanitizeDiagnosticField(extractTag(response, tag))
		if value == "" {
			continue
		}
		entry := tag + "=" + value
		if _, ok := seen[entry]; ok {
			continue
		}
		seen[entry] = struct{}{}
		fields = append(fields, entry)
	}
	sum := sha256.Sum256([]byte(response))
	if len(fields) == 0 {
		return fmt.Errorf("login response missing keep-url: bytes=%d sha256=%x", len(response), sum)
	}
	return fmt.Errorf("login rejected: %s; bytes=%d sha256=%x", strings.Join(fields, ", "), len(response), sum)
}

func sanitizeDiagnosticField(value string) string {
	value = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, strings.TrimSpace(value))
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > 160 {
		value = string(runes[:160]) + "..."
	}
	return value
}

func extractTag(input, tag string) string {
	decoder := xml.NewDecoder(strings.NewReader(input))
	depth := 0
	var text strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			return ""
		}
		switch value := token.(type) {
		case xml.StartElement:
			if depth > 0 {
				depth++
			} else if strings.EqualFold(value.Name.Local, tag) {
				depth = 1
			}
		case xml.CharData:
			if depth > 0 {
				text.Write(value)
			}
		case xml.EndElement:
			if depth > 0 {
				depth--
				if depth == 0 {
					return strings.Join(strings.Fields(text.String()), " ")
				}
			}
		}
	}
}
