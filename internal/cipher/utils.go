package cipher

import (
	"encoding/hex"
	"errors"
	"strings"
)

func padZeroString(text string, blockSize int) []byte {
	length := len(text)
	if blockSize > 0 && length%blockSize != 0 {
		length += blockSize - length%blockSize
	}
	out := make([]byte, length)
	copy(out, text)
	return out
}

func padZeroThenPKCS5String(text string, blockSize int) []byte {
	zeroPaddedLength := len(text)
	if zeroPaddedLength%blockSize != 0 {
		zeroPaddedLength += blockSize - zeroPaddedLength%blockSize
	}
	out := make([]byte, zeroPaddedLength+blockSize)
	copy(out, text)
	for i := zeroPaddedLength; i < len(out); i++ {
		out[i] = byte(blockSize)
	}
	return out
}

func pkcs5Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("invalid padding length")
	}
	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > blockSize {
		return nil, errors.New("invalid padding length")
	}
	for i := len(data) - padLen; i < len(data); i++ {
		if int(data[i]) != padLen {
			return nil, errors.New("invalid padding bytes")
		}
	}
	return data[:len(data)-padLen], nil
}

func trimZeroSuffix(data []byte) []byte {
	for i := len(data) - 1; i >= 0; i-- {
		if data[i] != 0 {
			return data[:i+1]
		}
	}
	return []byte{}
}

func decodeHexString(input string) ([]byte, error) {
	return hex.DecodeString(strings.TrimSpace(input))
}

func encodeHexUpper(data []byte) string {
	const digits = "0123456789ABCDEF"
	var out strings.Builder
	out.Grow(hex.EncodedLen(len(data)))
	for _, value := range data {
		out.WriteByte(digits[value>>4])
		out.WriteByte(digits[value&0x0f])
	}
	return out.String()
}
