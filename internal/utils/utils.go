package utils

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
	"strings"
	"time"
)

func NowString() string {
	return time.Now().In(time.FixedZone("CST-8", 8*60*60)).Format("2006-01-02 15:04:05")
}

func ExtractBetweenTags(input, startTag, endTag string) string {
	start := strings.Index(input, startTag)
	if start == -1 {
		return ""
	}
	start += len(startTag)
	end := strings.Index(input[start:], endTag)
	if end == -1 {
		return ""
	}
	return input[start : start+end]
}

func CleanCDATA(val string) string {
	val = strings.TrimSpace(val)
	val = strings.TrimPrefix(val, "<![CDATA[")
	val = strings.TrimSuffix(val, "]]>")
	return strings.TrimSpace(val)
}

func RandomMAC() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[0] &= 0xFE
	parts := make([]string, 0, 6)
	for _, v := range b {
		parts = append(parts, hex.EncodeToString([]byte{v}))
	}
	return strings.Join(parts, ":"), nil
}

func RandomString(n int) (string, error) {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	out := make([]byte, 0, n)
	for i := 0; i < n; i++ {
		v, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		out = append(out, chars[v.Int64()])
	}
	return string(out), nil
}

func RandomUUIDLower() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	hexStr := hex.EncodeToString(b)
	return strings.ToLower(strings.Join([]string{
		hexStr[0:8],
		hexStr[8:12],
		hexStr[12:16],
		hexStr[16:20],
		hexStr[20:32],
	}, "-")), nil
}
