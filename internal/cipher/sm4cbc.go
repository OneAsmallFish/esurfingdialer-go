package cipher

import (
	stdcipher "crypto/cipher"
	"errors"

	"github.com/emmansun/gmsm/sm4"
)

type SM4CBC struct {
	block stdcipher.Block
	iv    []byte
}

func NewSM4CBC(key, iv []byte) (*SM4CBC, error) {
	block, err := sm4.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return &SM4CBC{block: block, iv: iv}, nil
}

func (s *SM4CBC) Encrypt(text string) (string, error) {
	data := padZeroThenPKCS5String(text, sm4.BlockSize)

	if err := sm4CBCEncrypt(data, s.block, s.iv); err != nil {
		return "", err
	}
	return encodeHexUpper(data), nil
}

func (s *SM4CBC) Decrypt(hexText string) (string, error) {
	raw, err := decodeHexString(hexText)
	if err != nil {
		return "", err
	}
	if err := sm4CBCDecrypt(raw, s.block, s.iv); err != nil {
		return "", err
	}
	plain, err := pkcs5Unpad(raw, sm4.BlockSize)
	if err != nil {
		return "", err
	}
	return string(trimZeroSuffix(plain)), nil
}

func sm4CBCEncrypt(data []byte, block stdcipher.Block, iv []byte) error {
	if len(data)%sm4.BlockSize != 0 {
		return errors.New("invalid block size")
	}
	stdcipher.NewCBCEncrypter(block, iv).CryptBlocks(data, data)
	return nil
}

func sm4CBCDecrypt(data []byte, block stdcipher.Block, iv []byte) error {
	if len(data)%sm4.BlockSize != 0 {
		return errors.New("invalid block size")
	}
	stdcipher.NewCBCDecrypter(block, iv).CryptBlocks(data, data)
	return nil
}
