package server

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"strconv"
)

type tomlEncryption struct {
	Name    string
	File    string
	Default bool
}

type EncryptionConfig struct {
	Name     string
	Filename string
	Key      []byte
	Default  bool
}

// NewEncryptionsConfigFromToml will "parse" TOML encryptions
func NewEncryptionsConfigFromToml(tEncryptions []*tomlEncryption, autogenerate bool, rand *rand.Rand, configPath string) (map[string]*EncryptionConfig, error) {
	res := make(map[string]*EncryptionConfig)
	defaultFound := false

	for _, tEncryption := range tEncryptions {
		if tEncryption.Name == "" {
			return nil, errors.New("encryption must have a 'name' setting")
		}

		_, exists := res[tEncryption.Name]
		if exists {
			return nil, fmt.Errorf("duplicate encryption '%s'", tEncryption.Name)
		}

		conf := EncryptionConfig{
			Name: tEncryption.Name,
		}

		if tEncryption.File == "" {
			return nil, fmt.Errorf("encryption %s: 'file' is needed", tEncryption.Name)
		}

		keyPath := path.Clean(configPath + "/" + tEncryption.File)
		conf.Filename = keyPath

		if autogenerate {
			key, err := loadOrGenerateKeyFile(keyPath, rand)
			if err != nil {
				return nil, fmt.Errorf("encryption %s: %w", tEncryption.Name, err)
			}
			conf.Key = key
		} else {
			key, err := loadKeyFile(keyPath)
			if err != nil {
				return nil, fmt.Errorf("encryption %s: %w", tEncryption.Name, err)
			}
			conf.Key = key
		}

		if tEncryption.Default {
			if defaultFound {
				return nil, fmt.Errorf("encryption %s: already have a default encryption", tEncryption.Name)
			}
			defaultFound = true
			conf.Default = true
		}

		res[tEncryption.Name] = &conf
	}

	return res, nil
}

func loadOrGenerateKeyFile(filename string, rand *rand.Rand) ([]byte, error) {
	// if the file exists, load it
	if _, err := os.Stat(filename); err == nil {
		return loadKeyFile(filename)
	}

	return generateKeyFile(filename, rand)
}

// load a key file (base64 encoded)
func loadKeyFile(filename string) ([]byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("%w (see -genkey arg?)", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	requiredMode, err := strconv.ParseInt("0600", 8, 32)
	if err != nil {
		return nil, err
	}

	if stat.Mode() != os.FileMode(requiredMode) {
		return nil, fmt.Errorf("%s: only the owner should be able to read/write this file (mode 0600)", filename)
	}

	// read file content as string
	b64, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	// decode base64
	passphrase, err := base64.StdEncoding.DecodeString(string(b64))
	if err != nil {
		return nil, err
	}

	return passphrase, nil
}

// generate a randome key file (base64 encoded)
func generateKeyFile(filename string, rand *rand.Rand) ([]byte, error) {
	passphrase := make([]byte, 32)

	_, err := rand.Read(passphrase)
	if err != nil {
		return nil, err
	}

	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	str := base64.StdEncoding.EncodeToString(passphrase)

	_, err = f.Write([]byte(str))
	if err != nil {
		return nil, err
	}

	fmt.Printf("generated new encryption key file '%s'\n", filename)

	return passphrase, nil
}

// GetDefaultEncryption return the default encryption, or nil if none
func (conf *AppConfig) GetDefaultEncryption() *EncryptionConfig {
	for _, encryption := range conf.Encryptions {
		if encryption.Default {
			return encryption
		}
	}

	return nil
}
