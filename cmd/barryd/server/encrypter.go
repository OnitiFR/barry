package server

import (
	"fmt"
	"math/rand"
)

type Encrypt struct {
	// input parameters
	EncryptionConfig *EncryptionConfig
	Filename         string

	// output chan
	Result chan error
}

type Encrypter struct {
	NumWorkers int
	Channel    chan *Encrypt
	Log        *Log
	Rand       *rand.Rand
	Status     []string
}

// NewEncrypter initialize a new instance
func NewEncrypter(numWorkers int, log *Log, rand *rand.Rand) *Encrypter {
	return &Encrypter{
		NumWorkers: numWorkers,
		Channel:    make(chan *Encrypt),
		Log:        log,
		Rand:       rand,
		Status:     make([]string, numWorkers),
	}
}

// NewEncrypt initialize a new instance
func NewEncrypt(encryptionConfig *EncryptionConfig, filename string) *Encrypt {
	return &Encrypt{
		EncryptionConfig: encryptionConfig,
		Filename:         filename,
		Result:           make(chan error, 1),
	}
}

// Start the Encrypter (run workers)
func (enc *Encrypter) Start() {
	for i := 0; i < enc.NumWorkers; i++ {
		go func(id int) {
			for {
				enc.worker(id + 1)
			}
		}(i)
	}
}

func (enc *Encrypter) worker(id int) {
	var err error

	enc.Status[id-1] = "idle"
	enc.Log.Tracef(MsgGlob, "encryption worker %d: waiting", id)
	encrypt := <-enc.Channel

	// make sure we always fill result chan
	defer func() {
		encrypt.Result <- err
	}()

	enc.Status[id-1] = fmt.Sprintf("encrypting %s", encrypt.Filename)
	enc.Log.Infof(MsgGlob, "worker %d: encrypting %s", id, encrypt.Filename)
	err = encrypt.EncryptionConfig.EncryptFileInPlace(encrypt.Filename, enc.Rand, enc.Log)
	if err != nil {
		enc.Log.Errorf(MsgGlob, "worker %d: error encrypting %s: %s", id, encrypt.Filename, err)
	} else {
		enc.Log.Infof(MsgGlob, "worker %d: encrypted %s", id, encrypt.Filename)
	}
}
