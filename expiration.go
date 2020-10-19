package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

//  "units" used in "every xx" expressions
const (
	ExpirationUnitDefault = 0
	ExpirationUnitFile    = 1
	ExpirationUnitDay     = 2
)

type tomlExpiration struct {
	Local  []string `toml:"local"`
	Remote []string `toml:"remote"`
}

// ExpirationConfig is the expiration configuration at application level
type ExpirationConfig struct {
	Local  Expiration
	Remote Expiration
}

// Expiration host an expiration cycle, each project has two (local, remote)
type Expiration struct {
	FileCount     int
	ReferenceDate time.Time
	Lines         []ExpirationLine
}

// ExpirationLine is a line of expiration inside an Expiration
type ExpirationLine struct {
	Original  string
	Keep      time.Duration
	Every     int
	EveryUnit int
}

// ParseExpiration will parse an array of strings and return an Expiration
func ParseExpiration(linesIn []string) (Expiration, error) {
	linesOut := make([]ExpirationLine, 0)

	for _, lineIn := range linesIn {
		lineOut := ExpirationLine{
			Original: lineIn,
		}
		lineIn = strings.TrimSpace(lineIn)
		words := strings.Split(lineIn, " ")

		if len(words) != 3 && len(words) != 6 {
			return Expiration{}, fmt.Errorf("syntax error on [expiration] line '%s': invalid length", lineIn)
		}

		if words[0] != "keep" {
			return Expiration{}, fmt.Errorf("syntax error on [expiration] line '%s': missing 'keep' keyword", lineIn)
		}

		keepNum, err := strconv.Atoi(words[1])
		if err != nil {
			return Expiration{}, fmt.Errorf("syntax error on [expiration] line '%s': %s", lineIn, err)
		}

		switch words[2] {
		case "day", "days":
			lineOut.Keep = time.Duration(keepNum) * time.Hour * 24
		case "year", "years":
			lineOut.Keep = time.Duration(keepNum) * time.Hour * 24 * 365
		default:
			return Expiration{}, fmt.Errorf("syntax error on [expiration] line '%s': invalid 'keep' unit: %s", lineIn, words[2])
		}

		lineOut.EveryUnit = ExpirationUnitDefault
		if len(words) == 6 {
			if words[3] != "every" {
				return Expiration{}, fmt.Errorf("syntax error on [expiration] line '%s': missing 'every' keyword", lineIn)
			}
			lineOut.Every, err = strconv.Atoi(words[4])
			if err != nil {
				return Expiration{}, fmt.Errorf("syntax error on [expiration] line '%s': %s", lineIn, err)
			}
			switch words[5] {
			case "file", "files":
				lineOut.EveryUnit = ExpirationUnitFile
			case "day", "days":
				lineOut.EveryUnit = ExpirationUnitDay
			default:
				return Expiration{}, fmt.Errorf("syntax error on [expiration] line '%s': invalid 'every' unit: %s", lineIn, words[5])
			}

		}

		linesOut = append(linesOut, lineOut)
	}

	return Expiration{
		Lines: linesOut,
	}, nil
}

// NewExpirationConfigFromToml will check tomlExpiration and create an ExpirationConfig
func NewExpirationConfigFromToml(tConfig *tomlExpiration) (*ExpirationConfig, error) {
	expirationLocal, err := ParseExpiration(tConfig.Local)
	if err != nil {
		return nil, fmt.Errorf("expiration, local: %s", err)
	}

	expirationRemote, err := ParseExpiration(tConfig.Remote)
	if err != nil {
		return nil, fmt.Errorf("expiration, remote: %s", err)
	}

	return &ExpirationConfig{
		Local:  expirationLocal,
		Remote: expirationRemote,
	}, nil
}
