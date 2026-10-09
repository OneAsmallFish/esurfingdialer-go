package netclient

// T8.4 MD5 校验头对拍（Go 侧）：与 Harness.java md5 模式（DigestUtils.md5Hex）比对
// 空串/ASCII/中文 三输入（小写 hex、UTF-8 字节）。

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

func TestMd5HexVsJava(t *testing.T) {
	data, err := os.ReadFile("../../review/cross-validation/java-md5.out")
	if err != nil {
		t.Fatalf("read java md5 output: %v", err)
	}
	java := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "MD5|") {
			p := strings.SplitN(line, "|", 3)
			java[p[1]] = p[2]
		}
	}
	for _, s := range []string{"", "hello", "你好世界"} {
		sum := md5.Sum([]byte(s))
		got := hex.EncodeToString(sum[:])
		want := java[hex.EncodeToString([]byte(s))]
		if got != want {
			t.Errorf("MD5-MISMATCH %q: go=%s java=%s", s, got, want)
		} else {
			t.Logf("MD5-PASS %q -> %s", s, got)
		}
	}
}
