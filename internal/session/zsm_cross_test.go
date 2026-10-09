package session

// T8.3 ZSM 解析对拍（Go 侧）：10 个合成用例与 Harness.java zsm 模式比对。
// 每个用例前将 state.AlgoID 置 "PREV"（与 Java 侧 States.setAlgoId("PREV") 对齐），
// 观察 Initialize 结果与 algoID 的最终值（覆盖 F-002/F-004 的机器验证）。

import (
	"os"
	"strings"
	"testing"

	"esurfingdialer/internal/netclient"
	"esurfingdialer/internal/state"
)

func zsmCases() [][]byte {
	reg := "A474B1C2-3DE0-4EA2-8C5F-7093409CE6C4"
	cat := func(a []byte, b string) []byte { return append(append([]byte{}, a...), b...) }
	return [][]byte{
		cat([]byte{0, 0, 0, 0, 36}, reg), // 1 最小合法
		{0, 0, 0},                        // 2 len<4
		{0, 0, 0, 100, 1, 2, 3, 4, 5, 6}, // 3 keyLen 越界
		cat([]byte{0, 0, 0, 0, 200}, strings.Repeat("\x00", 6)),             // 4 algoIdLen 越界
		cat([]byte{0, 0, 0, 0, 36}, "FFFFFFFF-0000-0000-0000-000000000000"), // 5 未注册 UUID
		{0, 0, 0, 0, 0},                                   // 6 len=5 恰好边界
		cat([]byte{0, 0, 0, 0, 0, 36}, reg),               // 7 algoLen=0 但 layout-2 可解析
		{0, 0, 0, 255, 1, 2, 3, 4},                        // 8 keyLen 高位（Java 负/Go 255）
		cat([]byte{0, 0, 0, 0, 'X', 'X'}, reg),            // 9 仅暴力搜索可命中
		cat([]byte{0, 0, 0, 0, 36}, strings.ToLower(reg)), // 10 小写 UUID
	}
}

func TestZSMParseVsJava(t *testing.T) {
	data, err := os.ReadFile("../../review/cross-validation/java-zsm.out")
	if err != nil {
		t.Fatalf("read java zsm output: %v", err)
	}
	java := map[string][2]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "ZSM|") {
			p := strings.SplitN(line, "|", 4)
			java[p[1]] = [2]string{p[2], p[3]}
		}
	}

	st := state.New()
	cli := netclient.New(st, "", "", "")
	for i, zsm := range zsmCases() {
		st.SetAlgoID("PREV")
		s := New(st, cli)
		initErr := s.Initialize(zsm)
		initialized := "false"
		if s.IsInitialized() {
			initialized = "true"
		}
		algo := st.AlgoID()
		idx := itoaIdx(i + 1)
		jInit, jAlgo := java[idx][0], java[idx][1]
		status := "PASS"
		if initialized != jInit || algo != jAlgo {
			status = "DIFF"
		}
		t.Logf("ZSM #%d: go(init=%s, algo=%s, err=%v) vs java(init=%s, algo=%s) => %s",
			i+1, initialized, algo, initErr, jInit, jAlgo, status)
		if status == "DIFF" {
			t.Errorf("ZSM-DIFF #%d: go=(%s,%s) java=(%s,%s)", i+1, initialized, algo, jInit, jAlgo)
		}
	}
}

func itoaIdx(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoaIdx(n/10) + string(rune('0'+n%10))
}
