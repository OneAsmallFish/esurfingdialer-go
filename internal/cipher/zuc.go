package cipher

import (
	"github.com/emmansun/gmsm/zuc"
)

type ZUC struct {
	key []byte
	iv  []byte
}

func NewZUC(key, iv []byte) *ZUC {
	return &ZUC{key: key, iv: iv}
}

func (z *ZUC) Encrypt(text string) (string, error) {
	data := padZeroString(text, 4)
	out, err := z.process(true, data)
	if err != nil {
		return "", err
	}
	return encodeHexUpper(out), nil
}

func (z *ZUC) Decrypt(hexText string) (string, error) {
	raw, err := decodeHexString(hexText)
	if err != nil {
		return "", err
	}
	out, err := z.process(false, raw)
	if err != nil {
		return "", err
	}
	return string(trimZeroSuffix(out)), nil
}

func (z *ZUC) process(_ bool, input []byte) ([]byte, error) {
	stream, err := zuc.NewCipher(z.key, z.iv)
	if err != nil {
		return nil, err
	}
	stream.XORKeyStream(input, input)
	return input, nil
}
