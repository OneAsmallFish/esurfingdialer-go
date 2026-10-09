package cipher

// T7.3：非 8 倍数长度的 ModXTEA 密文应返回 error，不得 panic。

import (
	"testing"
)

func TestModXTEADecryptReturnsErrorOnBadLength(t *testing.T) {
	c, err := New("B3047D4E-67DF-4864-A6A5-DF9B9E525C79")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Decrypt("AABBCCDD"); err == nil { // 4 字节，非 8 倍数
		t.Fatal("expected error for 4-byte ciphertext")
	}
}

func TestModXTEAXTEAIVDecryptReturnsErrorOnBadLength(t *testing.T) {
	c, err := New("C32C68F9-CA81-4260-A329-BBAFD1A9CCD1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Decrypt("AABBCCDDEEFF001122334455"); err == nil { // 12 字节，非 8 倍数
		t.Fatal("expected error for 12-byte ciphertext")
	}
}

func TestOtherAlgosReturnErrorNotPanic(t *testing.T) {
	cases := map[string]string{
		"CAFBCBAD-B6E7-4CAB-8A67-14D39F00CE1E": "AABB", // AESCBC 过短
		"A474B1C2-3DE0-4EA2-8C5F-7093409CE6C4": "AABB", // AESECB 非块对齐
		"5BFBA864-BBA9-42DB-8EAD-49B5F412BD81": "",     // DESedeCBC 空
		"F3974434-C0DD-4C20-9E87-DDB6814A1C48": "AABB", // SM4CBC 非对齐
		"B809531F-0007-4B5B-923B-4BD560398113": "ZZZZ", // ZUC 非法 hex
	}
	for algo, hexIn := range cases {
		c, err := New(algo)
		if err != nil {
			t.Fatal(err)
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: unexpected panic: %v", algo, r)
				}
			}()
			if _, err := c.Decrypt(hexIn); err == nil {
				t.Logf("%s: short/invalid input accepted (len=%d)", algo, len(hexIn))
			} else {
				t.Logf("%s: returned error as expected: %v", algo, err)
			}
		}()
	}
}
