package session

import (
	"strings"
	"testing"
	"time"

	"esurfingdialer/internal/cipher"
	"esurfingdialer/internal/netclient"
	"esurfingdialer/internal/state"
)

const dynamicAid = "B809531F-0007-4B5B-923B-4BD560398113"

func dynamicModuleFixture(aid string) []byte {
	module := make([]byte, 0, 128)
	module = append(module, '0', '0', '1', byte(64))
	module = append(module, strings.Repeat("K", 64)...)
	module = append(module, byte(len(aid)))
	module = append(module, aid...)
	module = append(module, 0x5d, 0, 0, 0, 1)
	module = append(module, 0xa8, 0x98, 0, 0)
	module = append(module, 0x00)
	return module
}

func TestNormalizeNativePayload(t *testing.T) {
	input := "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<request>\n<local-time>ignored</local-time>\n</request>"
	want := "<?xml version=\"1.0\" encoding=\"utf-8\"?><request>\n<local-time>2026-08-31 12:34:56</local-time>\n<sysinfo><sysname>Linux</sysname><machine>armv8l</machine><ifname>wlan0</ifname><gsm.network.type>Unknown</gsm.network.type><ro.build.version.sdk>23</ro.build.version.sdk></sysinfo></request>"
	got := normalizeNativePayload(input, time.Date(2026, 8, 31, 12, 34, 56, 0, time.UTC))
	if got != want {
		t.Fatalf("normalized payload mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestInitializeDynamicModuleUsesNativePayloadCompatibility(t *testing.T) {
	st := state.New()
	cli := netclient.New(st, "", "", "")
	s := New(st, cli)
	if err := s.InitializeDynamicModule(dynamicModuleFixture(dynamicAid)); err != nil {
		t.Fatalf("InitializeDynamicModule: %v", err)
	}
	plain := "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<request>\n<client-id>vector</client-id>\n</request>"
	got, err := s.Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	want, err := cipher.NewZUC(
		[]byte{79, 63, 37, 112, 83, 43, 75, 89, 59, 93, 91, 33, 58, 65, 122, 72},
		[]byte{65, 60, 122, 85, 74, 33, 72, 61, 93, 45, 36, 69, 69, 60, 87, 121},
	).Encrypt(normalizeNativePayload(plain, time.Now().UTC()))
	if err != nil {
		t.Fatal(err)
	}
	if st.AlgoID() != dynamicAid || !s.IsInitialized() || got != want {
		t.Fatalf("dynamic state/cipher mismatch: algo=%q initialized=%v equal=%v", st.AlgoID(), s.IsInitialized(), got == want)
	}
}
