package cipher

type Interface interface {
	Encrypt(text string) (string, error)
	Decrypt(hex string) (string, error)
}
