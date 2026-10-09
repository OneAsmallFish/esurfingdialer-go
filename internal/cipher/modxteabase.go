package cipher

const (
	modXTEARounds = 32
	modXTEADelta  = uint32(0x9E3779B9)
)

func modXTEAEncryptBlock(v0In, v1In uint32, key []uint32) (uint32, uint32) {
	v0 := v0In
	v1 := v1In
	var sum uint32
	for i := 0; i < modXTEARounds; i++ {
		v0 += (v1 ^ sum) + key[sum&3] + ((v1 << 4) ^ (v1 >> 5))
		sum += modXTEADelta
		v1 += key[(sum>>11)&3] + (v0 ^ sum) + ((v0 << 4) ^ (v0 >> 5))
	}
	return v0, v1
}

func modXTEADecryptBlock(v0In, v1In uint32, key []uint32) (uint32, uint32) {
	v0 := v0In
	v1 := v1In
	sum := uint32(modXTEARounds)
	sum *= modXTEADelta
	for i := 0; i < modXTEARounds; i++ {
		v1 -= key[(sum>>11)&3] + (v0 ^ sum) + ((v0 << 4) ^ (v0 >> 5))
		sum -= modXTEADelta
		v0 -= (v1 ^ sum) + key[sum&3] + ((v1 << 4) ^ (v1 >> 5))
	}
	return v0, v1
}

func modXTEAGetInt(data []byte, offset int) uint32 {
	return uint32(data[offset])<<24 |
		uint32(data[offset+1])<<16 |
		uint32(data[offset+2])<<8 |
		uint32(data[offset+3])
}

func modXTEASetInt(data []byte, offset int, value uint32) {
	data[offset] = byte(value >> 24)
	data[offset+1] = byte(value >> 16)
	data[offset+2] = byte(value >> 8)
	data[offset+3] = byte(value)
}
