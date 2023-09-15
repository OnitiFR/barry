package server

import (
	"errors"
	"math/rand"
	"os"
)

// RandString generate a random string of A-Za-z0-9 runes
func RandString(n int, rand *rand.Rand) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

// ReadString read a string from a file, byte by byte, until null (slow but convenient)
func ReadString(file *os.File, maxLen int) (string, error) {
	var err error
	var s []byte

	b := make([]byte, 1)

	for {
		_, err = file.Read(b)
		if err != nil {
			return "", err
		}

		if b[0] == 0 {
			break
		}

		s = append(s, b[0])
		if len(s) > maxLen {
			return "", errors.New("string too long")
		}
	}

	return string(s), nil
}
