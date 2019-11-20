package crypto

import (
	"crypto/cipher"
	"crypto/des"
	"errors"
)

var (
	ErrorNotDesPKCS7Padding = errors.New("Not DesPKCS7 Padding")
)

func Encrypt(plaintext, key []byte) (ciphertext []byte, err error) {
	if len(key) < 2*des.BlockSize {
		err = errors.New("des key is too short")
		return
	}

	var block cipher.Block
	block, err = des.NewCipher(key[des.BlockSize:])
	if err != nil {
		return
	}

	// do padding
	DesPKCS7Padding(&plaintext)
	ciphertext = make([]byte, len(plaintext))

	iv := key[:des.BlockSize]
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, plaintext)
	return
}

// DesDecrypt decrypts ciphertext using key.
func Decrypt(ciphertext, key []byte) (plaintext []byte, err error) {
	if len(key) < 2*des.BlockSize {
		err = errors.New("des key too short")
		return
	}

	if len(ciphertext) < des.BlockSize {
		err = errors.New("ciphertext too short")
		return
	}

	// CBC mode always works in whole blocks.
	if len(ciphertext)%des.BlockSize != 0 {
		err = errors.New("ciphertext is not a multiple of the block size")
		return
	}

	var block cipher.Block
	block, err = des.NewCipher(key[des.BlockSize:])
	if err != nil {
		return
	}

	iv := key[:des.BlockSize]
	mode := cipher.NewCBCDecrypter(block, iv)

	plaintext = make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// do unpadding
	err = DesUnPKCS7Padding(&plaintext)
	return
}

func DesPKCS7Padding(data *[]byte) {
	padSize := des.BlockSize - len(*data)%des.BlockSize
	if padSize == 0 {
		padSize = des.BlockSize
	}

	padding := make([]byte, padSize)
	for i := range padding {
		padding[i] = byte(padSize)
	}

	*data = append(*data, padding...)
}

func DesUnPKCS7Padding(data *[]byte) error {
	dataSize := len(*data)
	if dataSize == 0 {
		return ErrorNotDesPKCS7Padding
	}

	if dataSize%des.BlockSize != 0 {
		return ErrorNotDesPKCS7Padding
	}

	padByte := (*data)[dataSize-1]
	padSize := int(padByte)
	if padSize > des.BlockSize {
		return ErrorNotDesPKCS7Padding
	}

	padding := (*data)[dataSize-padSize:]
	for i := range padding {
		if padding[i] != padByte {
			return ErrorNotDesPKCS7Padding
		}
	}

	*data = (*data)[:dataSize-padSize]
	return nil
}
