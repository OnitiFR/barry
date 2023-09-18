package server

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"path"
	"strconv"
	"time"
)

const EncryptionIvSize = 16
const BarrySignature = "BARRY1"

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

		if path.Ext(tEncryption.File) != ".key" {
			return nil, fmt.Errorf("encryption %s: 'file' must have a .key extension", tEncryption.Name)
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

// GetEncryption return an encryption by name, or an error if not found
func (conf *AppConfig) GetEncryption(name string) (*EncryptionConfig, error) {
	encryption, exists := conf.Encryptions[name]
	if !exists {
		return nil, fmt.Errorf("encryption '%s' not found", name)
	}

	return encryption, nil
}

// EncryptFile encrypt a file
func (enc *EncryptionConfig) EncryptFile(srcFilename string, dstFilename string, rand *rand.Rand) error {
	infile, err := os.Open(srcFilename)
	if err != nil {
		return err
	}
	defer infile.Close()

	block, err := aes.NewCipher(enc.Key)
	if err != nil {
		return err
	}

	iv := make([]byte, block.BlockSize())
	if _, err := io.ReadFull(rand, iv); err != nil {
		return err
	}

	outfile, err := os.OpenFile(dstFilename, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer outfile.Close()

	_, err = outfile.WriteString(BarrySignature)
	if err != nil {
		return err
	}

	// write comment
	_, err = outfile.WriteString("Barry Encryption v1")
	if err != nil {
		return err
	}

	_, err = outfile.Write([]byte{0})
	if err != nil {
		return err
	}

	// write key name
	_, err = outfile.WriteString(enc.Name)
	if err != nil {
		return err
	}

	_, err = outfile.Write([]byte{0})
	if err != nil {
		return err
	}

	// write the IV
	_, err = outfile.Write(iv)
	if err != nil {
		return err
	}

	// The buffer size must be multiple of 16 bytes
	bufferSize := 4096

	// write buffer size
	err = binary.Write(outfile, binary.LittleEndian, uint32(bufferSize))
	if err != nil {
		return err
	}

	buf := make([]byte, bufferSize)
	stream := cipher.NewCTR(block, iv)
	for {
		n, err := infile.Read(buf)
		if n > 0 {
			stream.XORKeyStream(buf, buf[:n])
			outfile.Write(buf[:n])
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}
	}

	return nil
}

// EncryptFileInPlace encrypt a file in place (using a temp file)
func (enc *EncryptionConfig) EncryptFileInPlace(filename string, rand *rand.Rand, log *Log) error {
	// get original file info
	stat, err := os.Stat(filename)
	if err != nil {
		return err
	}

	// create a temp file
	tmp, err := os.CreateTemp("", path.Base(filename)+"-encrypt")
	if err != nil {
		return err
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	defer os.Remove(tmp.Name())

	log.Tracef(MsgGlob, "encrypting %s (using %s)", filename, tmp.Name())
	start := time.Now()

	// encrypt the file
	err = enc.EncryptFile(filename, tmp.Name(), rand)
	if err != nil {
		return err
	}

	log.Tracef(MsgGlob, "encryption of %s done in %s, finalizing", filename, time.Since(start))
	start = time.Now()

	// move the temp file to the original file
	err = os.Rename(tmp.Name(), filename)
	if err != nil {
		return err
	}

	// restore original file info (mode, date)
	err = os.Chmod(filename, stat.Mode())
	if err != nil {
		return err
	}

	err = os.Chtimes(filename, stat.ModTime(), stat.ModTime())
	if err != nil {
		return err
	}

	log.Tracef(MsgGlob, "finalization of %s done in %s", filename, time.Since(start))

	return nil
}

// DecryptFile decrypt a file
// TODO: add security checks to header reading (limit string length, buffer size, etc)
func (app *App) DecryptFile(srcFilename string, dstFilename string) error {
	infile, err := os.Open(srcFilename)
	if err != nil {
		return err
	}
	defer infile.Close()

	// read signature
	sig, err := ReadString(infile, len(BarrySignature))
	if err != nil {
		return err
	}

	if sig != BarrySignature {
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

	cryptConf, err := app.Config.GetEncryption(keyName)
	if err != nil {
		return err
	}

	block, err := aes.NewCipher(cryptConf.Key)
	if err != nil {
		log.Panic(err)
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

	outfile, err := os.OpenFile(dstFilename, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer outfile.Close()

	buf := make([]byte, bufferSize)
	stream := cipher.NewCTR(block, iv)
	for {
		n, err := infile.Read(buf)
		if n > 0 {
			stream.XORKeyStream(buf, buf[:n])
			outfile.Write(buf[:n])
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}
	}

	return nil
}

// DecryptFileInPlace decrypt a file in place (using a temp file)
func (app *App) DecryptFileInPlace(filename string, log *Log) error {
	// get original file info
	stat, err := os.Stat(filename)
	if err != nil {
		return err
	}

	// create a temp file
	tmp, err := os.CreateTemp("", path.Base(filename)+"-decrypt")
	if err != nil {
		return err
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	defer os.Remove(tmp.Name())

	log.Tracef(MsgGlob, "decrypting %s (using %s)", filename, tmp.Name())
	start := time.Now()

	err = app.DecryptFile(filename, tmp.Name())
	if err != nil {
		return err
	}

	log.Tracef(MsgGlob, "decryption of %s done in %s, finalizing", filename, time.Since(start))
	start = time.Now()

	// move the temp file to the original file
	err = os.Rename(tmp.Name(), filename)
	if err != nil {
		return err
	}

	// restore original file info (mode, date)
	err = os.Chmod(filename, stat.Mode())
	if err != nil {
		return err
	}

	err = os.Chtimes(filename, stat.ModTime(), stat.ModTime())
	if err != nil {
		return err
	}

	log.Tracef(MsgGlob, "finalization of %s done in %s", filename, time.Since(start))

	return nil
}
