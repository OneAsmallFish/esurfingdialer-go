package cipher

import (
	"crypto/cipher"
	"crypto/des"
	"errors"
)

type DESedeECB struct {
	block1 cipher.Block
	block2 cipher.Block
}

func NewDESedeECB(key1, key2 []byte) (*DESedeECB, error) {
	block1, err := des.NewTripleDESCipher(key1)
	if err != nil {
		return nil, err
	}
	block2, err := des.NewTripleDESCipher(key2)
	if err != nil {
		return nil, err
	}
	return &DESedeECB{block1: block1, block2: block2}, nil
}

func (d *DESedeECB) Encrypt(text string) (string, error) {
	data := padZeroString(text, 16)
	if err := desEDEECBEncrypt(d.block1, data); err != nil {
		return "", err
	}
	if err := desEDEECBEncrypt(d.block2, data); err != nil {
		return "", err
	}
	return encodeHexUpper(data), nil
}

func (d *DESedeECB) Decrypt(hexText string) (string, error) {
	raw, err := decodeHexString(hexText)
	if err != nil {
		return "", err
	}
	if err := desEDEECBDecrypt(d.block2, raw); err != nil {
		return "", err
	}
	if err := desEDEECBDecrypt(d.block1, raw); err != nil {
		return "", err
	}
	return string(trimZeroSuffix(raw)), nil
}

func desEDEECBEncrypt(block cipher.Block, data []byte) error {
	if len(data)%block.BlockSize() != 0 {
		return errors.New("invalid block size")
	}
	encryptECB(block, data, data)
	return nil
}

func desEDEECBDecrypt(block cipher.Block, data []byte) error {
	if len(data)%block.BlockSize() != 0 {
		return errors.New("invalid block size")
	}
	decryptECB(block, data, data)
	return nil
}

func encryptECB(block cipher.Block, dst, src []byte) {
	blockSize := block.BlockSize()
	for i := 0; i < len(src); i += blockSize {
		block.Encrypt(dst[i:], src[i:])
	}
}

func decryptECB(block cipher.Block, dst, src []byte) {
	blockSize := block.BlockSize()
	for i := 0; i < len(src); i += blockSize {
		block.Decrypt(dst[i:], src[i:])
	}
}
