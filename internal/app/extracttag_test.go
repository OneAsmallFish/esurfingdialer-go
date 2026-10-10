package app

// T8.2 XML 响应解析等价类对拍（Go 侧）：10 个样例跑 extractTag，与 Harness.java parse 模式
// （Jsoup.parse(x, xmlParser()).getElementsByTag(tag).first()?.text()）输出比对。

import (
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"testing"
)

func parseSamples() []string {
	return []string{
		"<ticket>abc</ticket>",
		"<ticket><![CDATA[abc]]></ticket>",
		`<ticket id="1">abc</ticket>`,
		"<a><ticket>x</ticket><ticket>y</ticket></a>",
		"no target tag here",
		"<ticket> a b </ticket>",
		"<ticket>a&amp;b</ticket>",
		"<ticket/>",
		"<ticket>ab\ncd</ticket>",
		"<Ticket>abc</Ticket>",
	}
}

func TestExtractTagVsJsoup(t *testing.T) {
	data, err := os.ReadFile("../testdata/cross-validation/java-parse.out")
	if err != nil {
		t.Fatalf("read java parse output: %v", err)
	}
	java := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "PARSE|") {
			p := strings.SplitN(line, "|", 3)
			java[p[1]] = p[2]
		}
	}
	for i, s := range parseSamples() {
		idx := strconv.Itoa(i + 1)
		got := hex.EncodeToString([]byte(extractTag(s, "ticket")))
		want := java[idx]
		if got != want {
			wantPlain, _ := hex.DecodeString(want)
			t.Errorf("PARSE-MISMATCH #%d\n  input: %q\n  go   : %q\n  jsoup: %q",
				i+1, s, extractTag(s, "ticket"), string(wantPlain))
		} else {
			t.Logf("PARSE-PASS #%d", i+1)
		}
	}
}
