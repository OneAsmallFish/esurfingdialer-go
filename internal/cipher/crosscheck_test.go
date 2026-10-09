package cipher

// 审查对拍测试（T3.12/T3.13，GO_DIALER_REVIEW_PLAN.md Phase 3）。
// 输入：review/cross-validation/java-crypto.out（Harness.java crypto 模式输出）。
// 输出：review/cross-validation/go-crypto.out（ENC 行，供 Harness decryptfeed 反向喂回）。
//
// 运行：cd go-dialer && go test ./internal/cipher -run 'CrossCheck|GoSelfRoundTrip|WriteGoEnc' -v
// 注意：本测试预期对 ModXTEA/ModXTEAXTEAIV 报 FAIL（见审查 finding F-003），
// FAIL 本身即为证据，全部矩阵行都会打印后再统一判定。

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
)

var crossAlgos = []string{
	"CAFBCBAD-B6E7-4CAB-8A67-14D39F00CE1E",
	"A474B1C2-3DE0-4EA2-8C5F-7093409CE6C4",
	"5BFBA864-BBA9-42DB-8EAD-49B5F412BD81",
	"6E0B65FF-0B5B-459C-8FCE-EC7F2BEA9FF5",
	"B809531F-0007-4B5B-923B-4BD560398113",
	"F3974434-C0DD-4C20-9E87-DDB6814A1C48",
	"ED382482-F72C-4C41-A76D-28EEA0F1F2AF",
	"B3047D4E-67DF-4864-A6A5-DF9B9E525C79",
	"C32C68F9-CA81-4260-A329-BBAFD1A9CCD1",
}

func crossPlains() []string {
	pattern := "0123456789abcdefghijklmnopqrstuvwxyz"
	var sb strings.Builder
	for sb.Len() < 1024 {
		sb.WriteString(pattern)
	}
	return []string{
		"",
		"A",
		"01234567",
		"0123456789abcdef",
		"0123456789abcde",
		"你好世界测试啊",
		sb.String()[:1024],
	}
}

func javaOutPath() string {
	if p := os.Getenv("ESURFING_JAVA_OUT"); p != "" {
		return p
	}
	return "../../review/cross-validation/java-crypto.out"
}

func goEncOutPath() string {
	if p := os.Getenv("ESURFING_GO_OUT"); p != "" {
		return p
	}
	return "../../review/cross-validation/go-crypto.out"
}

// loadJavaEnc 解析 ENC|algo|pN|hex（跳过 ERROR 行）。
func loadJavaEnc(t *testing.T) map[string]string {
	data, err := os.ReadFile(javaOutPath())
	if err != nil {
		t.Fatalf("read java output: %v", err)
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if !strings.HasPrefix(line, "ENC|") {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) != 4 || strings.HasPrefix(parts[3], "ERROR") {
			continue
		}
		out[parts[1]+"|"+parts[2]] = parts[3]
	}
	if len(out) == 0 {
		t.Fatal("no ENC lines parsed from java output")
	}
	return out
}

// TestCrossCheckEncryptVsJava: Go 加密输出与 Java jar 加密输出逐字符比对（T3.12）。
func TestCrossCheckEncryptVsJava(t *testing.T) {
	java := loadJavaEnc(t)
	for _, algo := range crossAlgos {
		c, err := New(algo)
		if err != nil {
			t.Errorf("New(%s): %v", algo, err)
			continue
		}
		for i, p := range crossPlains() {
			key := fmt.Sprintf("%s|p%d", algo, i+1)
			want, ok := java[key]
			if !ok {
				t.Errorf("java output missing %s", key)
				continue
			}
			got, err := c.Encrypt(p)
			if err != nil {
				t.Errorf("ENC-FAIL %s: %v", key, err)
				continue
			}
			if got != want {
				t.Errorf("ENC-MISMATCH %s\n  go  : %s\n  java: %s", key, trunc(got), trunc(want))
			} else {
				t.Logf("ENC-PASS %s", key)
			}
		}
	}
}

// TestCrossCheckDecryptJavaOutput: Java jar 加密输出喂给 Go 解密，应还原明文（T3.13 Java→Go）。
func TestCrossCheckDecryptJavaOutput(t *testing.T) {
	java := loadJavaEnc(t)
	for _, algo := range crossAlgos {
		c, err := New(algo)
		if err != nil {
			t.Errorf("New(%s): %v", algo, err)
			continue
		}
		for i, p := range crossPlains() {
			key := fmt.Sprintf("%s|p%d", algo, i+1)
			enc, ok := java[key]
			if !ok {
				continue
			}
			got, err := c.Decrypt(enc)
			if err != nil {
				t.Errorf("DEC-FAIL %s: %v", key, err)
				continue
			}
			if got != p {
				t.Errorf("DEC-MISMATCH %s\n  got : %q\n  want: %q", key, trunc(got), trunc(p))
			} else {
				t.Logf("DEC-PASS %s", key)
			}
		}
	}
}

// TestGoSelfRoundTrip: Go 自身 encrypt→decrypt 回环（用于暴露 Go 内部不一致，如 F-003）。
func TestGoSelfRoundTrip(t *testing.T) {
	for _, algo := range crossAlgos {
		c, err := New(algo)
		if err != nil {
			t.Errorf("New(%s): %v", algo, err)
			continue
		}
		for i, p := range crossPlains() {
			key := fmt.Sprintf("%s|p%d", algo, i+1)
			enc, err := c.Encrypt(p)
			if err != nil {
				t.Errorf("RT-ENC-FAIL %s: %v", key, err)
				continue
			}
			dec, err := c.Decrypt(enc)
			if err != nil {
				t.Errorf("RT-DEC-FAIL %s: %v", key, err)
				continue
			}
			if dec != p {
				t.Errorf("RT-MISMATCH %s\n  dec : %q\n  want: %q", key, trunc(dec), trunc(p))
			} else {
				t.Logf("RT-PASS %s", key)
			}
		}
	}
}

// TestWriteGoEncOutput: 把 Go 的 ENC 输出写成与 java-crypto.out 同格式文件，
// 供 Harness.java decryptfeed 反向验证（T3.13 Go→Java）。
func TestWriteGoEncOutput(t *testing.T) {
	var sb strings.Builder
	for _, algo := range crossAlgos {
		c, err := New(algo)
		if err != nil {
			t.Fatalf("New(%s): %v", algo, err)
		}
		for i, p := range crossPlains() {
			enc, err := c.Encrypt(p)
			if err != nil {
				t.Fatalf("encrypt %s p%d: %v", algo, i+1, err)
			}
			fmt.Fprintf(&sb, "ENC|%s|p%d|%s\n", algo, i+1, enc)
		}
	}
	if err := os.WriteFile(goEncOutPath(), []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("write go output: %v", err)
	}
	t.Logf("wrote %s", goEncOutPath())
}

func trunc(s string) string {
	const half = 48
	if len(s) <= 2*half+3 {
		return s
	}
	return s[:half] + "..." + s[len(s)-half:]
}

var _ = hex.EncodeToString // 保留 hex 依赖（go-crypto 输出为纯 ASCII hex，无需再编码）
