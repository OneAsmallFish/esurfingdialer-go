package cipher

import "errors"

type ModXTEAXTEAIV struct {
	key1 []uint32
	key2 []uint32
	key3 []uint32
	iv   []uint32
}

func NewModXTEAXTEAIV(key1, key2, key3, iv []uint32) *ModXTEAXTEAIV {
	return &ModXTEAXTEAIV{key1: key1, key2: key2, key3: key3, iv: iv}
}

func (m *ModXTEAXTEAIV) Encrypt(text string) (string, error) {
	blocks := padZeroString(text, 8)
	prev := []uint32{m.iv[0], m.iv[1]}
	for i := 0; i < len(blocks); i += 8 {
		v0 := modXTEAGetInt(blocks, i)
		v1 := modXTEAGetInt(blocks, i+4)
		v0 ^= prev[0]
		v1 ^= prev[1]
		r0, r1 := modXTEAEncryptBlock(v0, v1, m.key3)
		r0, r1 = modXTEAEncryptBlock(r0, r1, m.key2)
		r0, r1 = modXTEAEncryptBlock(r0, r1, m.key1)
		modXTEASetInt(blocks, i, r0)
		modXTEASetInt(blocks, i+4, r1)
		prev[0], prev[1] = r0, r1
	}
	return encodeHexUpper(blocks), nil
}

func (m *ModXTEAXTEAIV) Decrypt(hexText string) (string, error) {
	blocks, err := decodeHexString(hexText)
	if err != nil {
		return "", err
	}
	if len(blocks)%8 != 0 {
		return "", errors.New("invalid ciphertext length")
	}
	prev := []uint32{m.iv[0], m.iv[1]}
	for i := 0; i < len(blocks); i += 8 {
		v0 := modXTEAGetInt(blocks, i)
		v1 := modXTEAGetInt(blocks, i+4)
		r0, r1 := modXTEADecryptBlock(v0, v1, m.key1)
		r0, r1 = modXTEADecryptBlock(r0, r1, m.key2)
		r0, r1 = modXTEADecryptBlock(r0, r1, m.key3)
		r0 ^= prev[0]
		r1 ^= prev[1]
		modXTEASetInt(blocks, i, r0)
		modXTEASetInt(blocks, i+4, r1)
		prev[0], prev[1] = v0, v1
	}
	return string(trimZeroSuffix(blocks)), nil
}
