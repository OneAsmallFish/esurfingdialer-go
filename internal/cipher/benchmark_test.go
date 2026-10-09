package cipher

import (
	"strings"
	"testing"
)

var benchmarkAlgorithms = []struct {
	name string
	id   string
}{
	{"AESCBC", "CAFBCBAD-B6E7-4CAB-8A67-14D39F00CE1E"},
	{"AESECB", "A474B1C2-3DE0-4EA2-8C5F-7093409CE6C4"},
	{"DESedeCBC", "5BFBA864-BBA9-42DB-8EAD-49B5F412BD81"},
	{"DESedeECB", "6E0B65FF-0B5B-459C-8FCE-EC7F2BEA9FF5"},
	{"ZUC", "B809531F-0007-4B5B-923B-4BD560398113"},
	{"SM4CBC", "F3974434-C0DD-4C20-9E87-DDB6814A1C48"},
	{"SM4ECB", "ED382482-F72C-4C41-A76D-28EEA0F1F2AF"},
	{"ModXTEA", "B3047D4E-67DF-4864-A6A5-DF9B9E525C79"},
	{"ModXTEAXTEAIV", "C32C68F9-CA81-4260-A329-BBAFD1A9CCD1"},
}

func BenchmarkCipherRoundTrip512B(b *testing.B) {
	plain := strings.Repeat("0123456789abcdef", 32)
	for _, algorithm := range benchmarkAlgorithms {
		b.Run(algorithm.name, func(b *testing.B) {
			implementation, err := New(algorithm.id)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				encrypted, err := implementation.Encrypt(plain)
				if err != nil {
					b.Fatal(err)
				}
				decrypted, err := implementation.Decrypt(encrypted)
				if err != nil {
					b.Fatal(err)
				}
				if decrypted != plain {
					b.Fatal("round trip mismatch")
				}
			}
		})
	}
}

func BenchmarkEncodeHexUpper1KiB(b *testing.B) {
	input := make([]byte, 1024)
	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	for b.Loop() {
		_ = encodeHexUpper(input)
	}
}
