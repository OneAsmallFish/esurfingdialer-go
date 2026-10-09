package netclient

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type smsRequest struct {
	SchoolID  string `json:"schoolid"`
	UserName  string `json:"username"`
	Timestamp string `json:"timestamp"`
	Token     string `json:"authenticator"`
}

type smsResponse struct {
	ResCode string `json:"rescode"`
}

func (c *Client) CheckVerifyCodeStatus(ctx context.Context, username string) bool {
	return c.requestVerifyCode(ctx, username, "QueryVerificateCodeStatus", "11062000")
}

func (c *Client) GetVerifyCode(ctx context.Context, username string) bool {
	return c.requestVerifyCode(ctx, username, "QueryAuthCode", "0")
}

func (c *Client) requestVerifyCode(ctx context.Context, username, reqType, success string) bool {
	url := c.st.ExtraCfgURLItem(reqType)
	if strings.TrimSpace(url) == "" {
		return false
	}

	// token=MD5(schoolId+timestamp+AUTH_KEY) 大写
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	schoolID := c.st.SchoolID()
	token := md5.Sum([]byte(schoolID + timestamp + authKey))
	reqBody := smsRequest{
		SchoolID:  schoolID,
		UserName:  username,
		Timestamp: timestamp,
		Token:     strings.ToUpper(hex.EncodeToString(token[:])),
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return false
	}

	payload := string(bodyBytes)
	resp, err := c.postFollowingRedirects(ctx, url, payload, func(req *http.Request) {
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "okhttp/3.4.1")
		req.Header.Set("Content-Type", "application/json")
	})
	if err != nil {
		return false
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		closeRedirectBody(resp.Body)
		return false
	}
	defer resp.Body.Close()

	body, err := readResponseBody(resp.Body)
	if err != nil {
		return false
	}
	var model smsResponse
	if err := json.Unmarshal(body, &model); err != nil {
		return false
	}
	return model.ResCode == success
}
