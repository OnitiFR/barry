package common

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
)

// This feature is common to both the server and the client, for emergency decryption

const EncryptionIvSize = 16
const BarrySignature = "BARRY1"

// DecryptFile will decrypt a file, where you must provide a callback to return the key
func DecryptFile(infile *os.File, outfile *os.File, keyCallback func(string) ([]byte, error)) error {
	sig := make([]byte, len(BarrySignature))
	_, err := infile.Read(sig)
	if err != nil {
		return err
	}

	if string(sig) != BarrySignature {
		return fmt.Errorf("invalid signature")
	}

	// read comment string
	_, err = ReadString(infile, 128)
	if err != nil {
		return err
	}

	// read key name string
	keyName, err := ReadString(infile, 64)
	if err != nil {
		return err
	}

	key, err := keyCallback(keyName)
	if err != nil {
		return err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		log.Panic(err)
	}

	// read sha256 hash
	expectedHash := make([]byte, 32)
	_, err = infile.Read(expectedHash)
	if err != nil {
		return err
	}

	// read the IV
	iv := make([]byte, block.BlockSize())
	n, err := infile.Read(iv)
	if err != nil {
		return err
	}

	if n != block.BlockSize() {
		return fmt.Errorf("invalid IV size")
	}

	// read buffer size
	var bufferSize uint32
	err = binary.Read(infile, binary.LittleEndian, &bufferSize)
	if err != nil {
		return err
	}

	if bufferSize < 16 || bufferSize > 100*1024*1024 {
		return fmt.Errorf("invalid buffer size (out of range)")
	}

	if bufferSize%16 != 0 {
		return fmt.Errorf("invalid buffer size (must be multiple of 16)")
	}

	hash := sha256.New()

	buf := make([]byte, bufferSize)
	stream := cipher.NewCTR(block, iv)
	for {
		n, err := infile.Read(buf)
		if n > 0 {
			stream.XORKeyStream(buf, buf[:n])
			outfile.Write(buf[:n])
			hash.Write(buf[:n])
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}
	}

	if !bytes.Equal(hash.Sum(nil), expectedHash) {
		return fmt.Errorf("invalid checksum, is the key correct?")
	}

	return nil
}
