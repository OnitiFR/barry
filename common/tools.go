package common

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/c2h5oh/datasize"
)

// TrueStr is the true truth.
const TrueStr = "true"

// PathExist returns true if a file or directory exists
func PathExist(path string) bool {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}

	// I know Stat() may fail for a lot of reasons, but
	// os.IsNotExist is not enough, see ENOTDIR for
	// things like /etc/passwd/test
	if err != nil {
		return false
	}

	return true
}

// InterfaceValueToString converts most interface types to string
func InterfaceValueToString(iv interface{}, format string) string {

	// civ is the casted iv
	switch civ := iv.(type) {
	case int:
		return fmt.Sprintf("%d", civ)
	case int32:
		return fmt.Sprintf("%d", civ)
	case int64:
		if format == "size" {
			return (datasize.ByteSize(civ) * datasize.B).HR()
		}
		return strconv.FormatInt(civ, 10)
	case uint64:
		if format == "size" {
			return (datasize.ByteSize(civ) * datasize.B).HR()
		}
		return strconv.FormatUint(civ, 10)
	case float32:
		if format == "money" {
			return fmt.Sprintf("%.2f", civ)
		}
		return fmt.Sprintf("%f", civ)
	case float64:
		if format == "money" {
			return strconv.FormatFloat(civ, 'f', 2, 64)
		}
		return strconv.FormatFloat(civ, 'f', -1, 64)
	case string:
		return iv.(string)
	case []byte:
		return string(iv.([]byte))
	case bool:
		return strconv.FormatBool(iv.(bool))
	case time.Time:
		return civ.String()
	case time.Duration:
		return civ.String()
	case []string:
		return strings.Join(civ, ", ")
	}
	return "INVALID_TYPE"
}

// CleanURL by parsing it
func CleanURL(urlIn string) (string, error) {
	urlObj, err := url.Parse(urlIn)
	if err != nil {
		return urlIn, err
	}
	urlObj.Path = path.Clean(urlObj.Path)
	return urlObj.String(), nil
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
