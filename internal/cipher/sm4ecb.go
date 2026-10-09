package cipher

import (
	"crypto/cipher"
	"errors"

	"github.com/emmansun/gmsm/sm4"
)

type SM4ECB struct {
	block cipher.Block
}

func NewSM4ECB(key []byte) (*SM4ECB, error) {
	block, err := sm4.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return &SM4ECB{block: block}, nil
}

func (s *SM4ECB) Encrypt(text string) (string, error) {
	data := padZeroThenPKCS5String(text, sm4.BlockSize)

	if err := s.ecbEncrypt(data); err != nil {
		return "", err
	}
	return encodeHexUpper(data), nil
}

func (s *SM4ECB) Decrypt(hexText string) (string, error) {
	raw, err := decodeHexString(hexText)
	if err != nil {
		return "", err
	}
	if err := s.ecbDecrypt(raw); err != nil {
		return "", err
	}
	plain, err := pkcs5Unpad(raw, sm4.BlockSize)
	if err != nil {
		return "", err
	}
	return string(trimZeroSuffix(plain)), nil
}

func (s *SM4ECB) ecbEncrypt(data []byte) error {
	if len(data)%sm4.BlockSize != 0 {
		return errors.New("invalid block size")
	}
	for i := 0; i < len(data); i += sm4.BlockSize {
		s.block.Encrypt(data[i:], data[i:])
	}
	return nil
}

func (s *SM4ECB) ecbDecrypt(data []byte) error {
	if len(data)%sm4.BlockSize != 0 {
		return errors.New("invalid block size")
	}
	for i := 0; i < len(data); i += sm4.BlockSize {
		s.block.Decrypt(data[i:], data[i:])
	}
	return nil
}
