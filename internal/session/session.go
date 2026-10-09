package session

import (
	"encoding/binary"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"esurfingdialer/internal/cipher"
	"esurfingdialer/internal/netclient"
	"esurfingdialer/internal/state"
)

type Session struct {
	st  *state.State
	cli *netclient.Client

	initialized bool
	cipher      cipher.Interface
	dynamic     bool
}

func New(st *state.State, cli *netclient.Client) *Session {
	return &Session{st: st, cli: cli}
}

func (s *Session) Initialize(zsm []byte) error {
	// A failed reinitialization must not leave the previous cipher usable.
	s.initialized = false
	s.cipher = nil
	s.dynamic = false
	ok, algoID := parseAlgoID(zsm)
	if !ok {
		return errors.New("invalid zsm data")
	}
	c, err := cipher.New(algoID)
	if err != nil {
		return err
	}
	s.st.SetAlgoID(algoID)
	s.cipher = c
	s.initialized = true
	return nil
}

// InitializeDynamicModule accepts the Android ZSM v2 container returned with
// Error-Code: 200. The native module's captured aid selects the same cipher
// implementation as the fixed Go factory; its additional behavior is applied
// by normalizeNativePayload before encryption.
func (s *Session) InitializeDynamicModule(module []byte) error {
	s.initialized = false
	s.cipher = nil
	s.dynamic = false
	aid, err := parseDynamicModule(module)
	if err != nil {
		return err
	}
	c, err := cipher.New(aid)
	if err != nil {
		return err
	}
	s.st.SetAlgoID(aid)
	s.cipher = c
	s.dynamic = true
	s.initialized = true
	return nil
}

func (s *Session) IsInitialized() bool {
	return s.initialized
}

func (s *Session) Reset() {
	s.initialized = false
	s.cipher = nil
	s.dynamic = false
}

func (s *Session) Encrypt(text string) (string, error) {
	if !s.initialized || s.cipher == nil {
		return "", errors.New("session not initialized")
	}
	if s.dynamic {
		text = normalizeNativePayload(text, time.Now().UTC())
	}
	return s.cipher.Encrypt(text)
}

var nativeLocalTimeTag = regexp.MustCompile(`(?s)<local-time>.*?</local-time>`)

const nativeSysinfo = "<sysinfo><sysname>Linux</sysname><machine>armv8l</machine><ifname>wlan0</ifname><gsm.network.type>Unknown</gsm.network.type><ro.build.version.sdk>23</ro.build.version.sdk></sysinfo>"

func normalizeNativePayload(text string, now time.Time) string {
	text = strings.Replace(text, "?>\n<request>", "?><request>", 1)
	text = nativeLocalTimeTag.ReplaceAllString(text, "<local-time>"+now.Format("2006-01-02 15:04:05")+"</local-time>")
	if strings.Contains(text, "</request>") && !strings.Contains(text, "<sysinfo>") {
		text = strings.Replace(text, "</request>", nativeSysinfo+"</request>", 1)
	}
	return text
}

func parseDynamicModule(module []byte) (string, error) {
	if len(module) < 4 {
		return "", errors.New("invalid dynamic zsm module: short header")
	}
	keyLen := int(module[3])
	aidLenPos := 4 + keyLen
	if aidLenPos >= len(module) {
		return "", errors.New("invalid dynamic zsm module: key length")
	}
	aidLen := int(module[aidLenPos])
	aidPos := aidLenPos + 1
	metaEnd := aidPos + aidLen
	if metaEnd+9 > len(module) {
		return "", errors.New("invalid dynamic zsm module: aid length")
	}
	props := module[metaEnd : metaEnd+5]
	packed := binary.LittleEndian.Uint32(module[metaEnd+5 : metaEnd+9])
	variant := packed >> 28
	decodedSize := packed & 0x0fffffff
	if props[0] >= 225 || decodedSize == 0 || decodedSize > 0x08000000 || variant > 3 {
		return "", fmt.Errorf("invalid dynamic zsm module: variant=%d size=%d", variant, decodedSize)
	}
	return string(module[aidPos:metaEnd]), nil
}

func (s *Session) Decrypt(hex string) (string, error) {
	if !s.initialized || s.cipher == nil {
		return "", errors.New("session not initialized")
	}
	return s.cipher.Decrypt(hex)
}

func parseAlgoID(zsm []byte) (bool, string) {
	// zsm 结构包含 key 与 algoId，这里只提取 algoId
	if len(zsm) < 4 {
		return false, ""
	}
	pos := 4
	keyLen := int(zsm[3])
	if pos+keyLen > len(zsm) {
		return false, ""
	}
	pos += keyLen
	if pos >= len(zsm) {
		return false, ""
	}

	// jar 字节码只接受 keyLen + key + algoLen + algoId 布局。
	algo, ok := parseAlgoByLayout(zsm, pos)
	return ok, algo
}

func parseAlgoByLayout(zsm []byte, pos int) (string, bool) {
	if pos >= len(zsm) {
		return "", false
	}
	algoLen := int(zsm[pos])
	pos++
	if pos+algoLen > len(zsm) {
		return "", false
	}
	return string(zsm[pos : pos+algoLen]), true
}
