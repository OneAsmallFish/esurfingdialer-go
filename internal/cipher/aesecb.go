package cipher

import (
	"crypto/aes"
	stdcipher "crypto/cipher"
	"errors"
)

type AESECB struct {
	block1 stdcipher.Block
	block2 stdcipher.Block
}

func NewAESECB(key1, key2 []byte) (*AESECB, error) {
	block1, err := aes.NewCipher(key1)
	if err != nil {
		return nil, err
	}
	block2, err := aes.NewCipher(key2)
	if err != nil {
		return nil, err
	}
	return &AESECB{block1: block1, block2: block2}, nil
}

func (a *AESECB) Encrypt(text string) (string, error) {
	data := padZeroString(text, aes.BlockSize)
	if err := aesECBEncrypt(a.block1, data); err != nil {
		return "", err
	}
	if err := aesECBEncrypt(a.block2, data); err != nil {
		return "", err
	}
	return encodeHexUpper(data), nil
}

func (a *AESECB) Decrypt(hexText string) (string, error) {
	raw, err := decodeHexString(hexText)
	if err != nil {
		return "", err
	}
	if err := aesECBDecrypt(a.block2, raw); err != nil {
		return "", err
	}
	if err := aesECBDecrypt(a.block1, raw); err != nil {
		return "", err
	}
	return string(trimZeroSuffix(raw)), nil
}

func aesECBEncrypt(block stdcipher.Block, data []byte) error {
	if len(data)%aes.BlockSize != 0 {
		return errors.New("invalid block size")
	}
	for i := 0; i < len(data); i += aes.BlockSize {
		block.Encrypt(data[i:], data[i:])
	}
	return nil
}

func aesECBDecrypt(block stdcipher.Block, data []byte) error {
	if len(data)%aes.BlockSize != 0 {
		return errors.New("invalid block size")
	}
	for i := 0; i < len(data); i += aes.BlockSize {
		block.Decrypt(data[i:], data[i:])
	}
	return nil
}
