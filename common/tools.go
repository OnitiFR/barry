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

	switch iv.(type) {
	case int:
		return fmt.Sprintf("%d", iv.(int))
	case int32:
		return fmt.Sprintf("%d", iv.(int32))
	case int64:
		if format == "size" {
			return (datasize.ByteSize(iv.(int64)) * datasize.B).HR()
		}
		return strconv.FormatInt(iv.(int64), 10)
	case uint64:
		if format == "size" {
			return (datasize.ByteSize(iv.(int64)) * datasize.B).HR()
		}
		return strconv.FormatUint(iv.(uint64), 10)
	case float32:
		if format == "money" {
			return fmt.Sprintf("%.2f", iv.(float32))
		}
		return fmt.Sprintf("%f", iv.(float32))
	case float64:
		if format == "money" {
			return strconv.FormatFloat(iv.(float64), 'f', 2, 64)
		}
		return strconv.FormatFloat(iv.(float64), 'f', -1, 64)
	case string:
		return iv.(string)
	case []byte:
		return string(iv.([]byte))
	case bool:
		return strconv.FormatBool(iv.(bool))
	case time.Time:
		return iv.(time.Time).String()
	case time.Duration:
		return iv.(time.Duration).String()
	case []string:
		return strings.Join(iv.([]string), ", ")
	}
	return "INVALID_TYPE"
}
