package cipher

import (
	stdcipher "crypto/cipher"
	"crypto/des"
	"errors"
)

type DESedeCBC struct {
	block1 stdcipher.Block
	block2 stdcipher.Block
	iv     []byte
}

func NewDESedeCBC(key1, key2, iv []byte) (*DESedeCBC, error) {
	block1, err := des.NewTripleDESCipher(key1)
	if err != nil {
		return nil, err
	}
	block2, err := des.NewTripleDESCipher(key2)
	if err != nil {
		return nil, err
	}
	return &DESedeCBC{block1: block1, block2: block2, iv: iv}, nil
}

func (d *DESedeCBC) Encrypt(text string) (string, error) {
	data := padZeroString(text, 16)
	if err := desEDECBCEncrypt(d.block1, d.iv, data); err != nil {
		return "", err
	}
	if err := desEDECBCEncrypt(d.block2, d.iv, data); err != nil {
		return "", err
	}
	return encodeHexUpper(data), nil
}

func (d *DESedeCBC) Decrypt(hexText string) (string, error) {
	raw, err := decodeHexString(hexText)
	if err != nil {
		return "", err
	}
	if err := desEDECBCDecrypt(d.block2, d.iv, raw); err != nil {
		return "", err
	}
	if err := desEDECBCDecrypt(d.block1, d.iv, raw); err != nil {
		return "", err
	}
	return string(trimZeroSuffix(raw)), nil
}

func desEDECBCEncrypt(block stdcipher.Block, iv, data []byte) error {
	if len(data)%block.BlockSize() != 0 {
		return errors.New("invalid block size")
	}
	stdcipher.NewCBCEncrypter(block, iv).CryptBlocks(data, data)
	return nil
}

func desEDECBCDecrypt(block stdcipher.Block, iv, data []byte) error {
	if len(data)%block.BlockSize() != 0 {
		return errors.New("invalid block size")
	}
	stdcipher.NewCBCDecrypter(block, iv).CryptBlocks(data, data)
	return nil
}
