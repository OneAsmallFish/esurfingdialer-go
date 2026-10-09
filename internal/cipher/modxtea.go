package cipher

import "errors"

type ModXTEA struct {
	key1 []uint32
	key2 []uint32
	key3 []uint32
}

func NewModXTEA(key1, key2, key3 []uint32) *ModXTEA {
	return &ModXTEA{key1: key1, key2: key2, key3: key3}
}

func (m *ModXTEA) Encrypt(text string) (string, error) {
	blocks := padZeroString(text, 8)
	for i := 0; i < len(blocks); i += 8 {
		v0 := modXTEAGetInt(blocks, i)
		v1 := modXTEAGetInt(blocks, i+4)
		r0, r1 := modXTEAEncryptBlock(v0, v1, m.key1)
		r0, r1 = modXTEAEncryptBlock(r0, r1, m.key2)
		r0, r1 = modXTEAEncryptBlock(r0, r1, m.key3)
		modXTEASetInt(blocks, i, r0)
		modXTEASetInt(blocks, i+4, r1)
	}
	return encodeHexUpper(blocks), nil
}

func (m *ModXTEA) Decrypt(hexText string) (string, error) {
	blocks, err := decodeHexString(hexText)
	if err != nil {
		return "", err
	}
	if len(blocks)%8 != 0 {
		return "", errors.New("invalid ciphertext length")
	}
	for i := 0; i < len(blocks); i += 8 {
		v0 := modXTEAGetInt(blocks, i)
		v1 := modXTEAGetInt(blocks, i+4)
		r0, r1 := modXTEADecryptBlock(v0, v1, m.key3)
		r0, r1 = modXTEADecryptBlock(r0, r1, m.key2)
		r0, r1 = modXTEADecryptBlock(r0, r1, m.key1)
		modXTEASetInt(blocks, i, r0)
		modXTEASetInt(blocks, i+4, r1)
	}
	return string(trimZeroSuffix(blocks)), nil
}
