package common

import (
	"fmt"
	"os"
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
