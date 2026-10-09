package cipher

import (
	"crypto/aes"
	stdcipher "crypto/cipher"
	"errors"
)

type AESCBC struct {
	block1 stdcipher.Block
	block2 stdcipher.Block
	iv     []byte
}

func NewAESCBC(key1, key2, iv []byte) (*AESCBC, error) {
	block1, err := aes.NewCipher(key1)
	if err != nil {
		return nil, err
	}
	block2, err := aes.NewCipher(key2)
	if err != nil {
		return nil, err
	}
	return &AESCBC{block1: block1, block2: block2, iv: iv}, nil
}

func (a *AESCBC) Encrypt(text string) (string, error) {
	paddedLength := len(text)
	if paddedLength%aes.BlockSize != 0 {
		paddedLength += aes.BlockSize - paddedLength%aes.BlockSize
	}
	out := make([]byte, 2*aes.BlockSize+paddedLength)
	copy(out[:aes.BlockSize], a.iv)
	copy(out[aes.BlockSize:2*aes.BlockSize], a.iv)
	copy(out[2*aes.BlockSize:], text)
	if err := aesCBCEncryptNoPadding(a.block1, a.iv, out[2*aes.BlockSize:]); err != nil {
		return "", err
	}
	if err := aesCBCEncryptNoPadding(a.block2, a.iv, out[aes.BlockSize:]); err != nil {
		return "", err
	}
	return encodeHexUpper(out), nil
}

func (a *AESCBC) Decrypt(hexText string) (string, error) {
	raw, err := decodeHexString(hexText)
	if err != nil {
		return "", err
	}
	if len(raw) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}
	r1 := raw[aes.BlockSize:]
	if err := aesCBCDecryptNoPadding(a.block2, a.iv, r1); err != nil {
		return "", err
	}
	if len(r1) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}
	r2 := r1[aes.BlockSize:]
	if err := aesCBCDecryptNoPadding(a.block1, a.iv, r2); err != nil {
		return "", err
	}
	return string(trimZeroSuffix(r2)), nil
}

func aesCBCEncryptNoPadding(block stdcipher.Block, iv, data []byte) error {
	if len(data)%aes.BlockSize != 0 {
		return errors.New("invalid block size")
	}
	stdcipher.NewCBCEncrypter(block, iv).CryptBlocks(data, data)
	return nil
}

func aesCBCDecryptNoPadding(block stdcipher.Block, iv, data []byte) error {
	if len(data)%aes.BlockSize != 0 {
		return errors.New("invalid block size")
	}
	stdcipher.NewCBCDecrypter(block, iv).CryptBlocks(data, data)
	return nil
}
