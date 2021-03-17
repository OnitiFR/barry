package server

import (
	"errors"
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
	Custom        bool // was customized? (versus "cloned from config")
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

// ExpirationResult is the output of the whole Expiration thing :) (see GetNext)
type ExpirationResult struct {
	Original string
	Keep     time.Duration
}

// ParseExpiration will parse an array of strings and return an Expiration
func ParseExpiration(linesIn []string) (Expiration, error) {
	linesOut := make([]ExpirationLine, 0)
	defaultFound := false

	if len(linesIn) == 0 {
		return Expiration{}, errors.New("expiration can't be empty")
	}

	for _, lineIn := range linesIn {
		lineIn = strings.TrimSpace(lineIn)
		lineOut := ExpirationLine{
			Original: lineIn,
		}
		words := strings.Split(lineIn, " ")

		if len(words) != 3 && len(words) != 6 {
			return Expiration{}, fmt.Errorf("line '%s': invalid length", lineIn)
		}

		if words[0] != "keep" {
			return Expiration{}, fmt.Errorf("line '%s': missing 'keep' keyword", lineIn)
		}

		keepNum, err := strconv.Atoi(words[1])
		if err != nil {
			return Expiration{}, fmt.Errorf("line '%s': %s", lineIn, err)
		}
		if keepNum < 1 {
			return Expiration{}, fmt.Errorf("line '%s': invalid value %d", lineIn, keepNum)
		}

		switch words[2] {
		case "minute", "minutes":
			lineOut.Keep = time.Duration(keepNum) * time.Minute
		case "hour", "hours":
			lineOut.Keep = time.Duration(keepNum) * time.Hour
		case "day", "days":
			lineOut.Keep = time.Duration(keepNum) * time.Hour * 24
		case "year", "years":
			lineOut.Keep = time.Duration(keepNum) * time.Hour * 24 * 365
		default:
			return Expiration{}, fmt.Errorf("line '%s': invalid 'keep' unit: %s", lineIn, words[2])
		}

		lineOut.EveryUnit = ExpirationUnitDefault
		if len(words) == 6 {
			if words[3] != "every" {
				return Expiration{}, fmt.Errorf("line '%s': missing 'every' keyword", lineIn)
			}
			lineOut.Every, err = strconv.Atoi(words[4])
			if err != nil {
				return Expiration{}, fmt.Errorf("line '%s': %s", lineIn, err)
			}
			if lineOut.Every < 1 {
				return Expiration{}, fmt.Errorf("line '%s': invalid value %d", lineIn, lineOut.Every)
			}
			switch words[5] {
			case "file", "files":
				lineOut.EveryUnit = ExpirationUnitFile
			case "day", "days":
				lineOut.EveryUnit = ExpirationUnitDay
			default:
				return Expiration{}, fmt.Errorf("line '%s': invalid 'every' unit: %s", lineIn, words[5])
			}

		}

		if len(words) == 3 {
			defaultFound = true
		}

		linesOut = append(linesOut, lineOut)
	}

	if !defaultFound {
		return Expiration{}, errors.New("a default expiration must be given (without any 'every')")
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

// GetNext return the next expiration duration
func (exp *Expiration) GetNext(modTime time.Time) ExpirationResult {
	exp.FileCount++
	var maxExpiration ExpirationResult

	for _, line := range exp.Lines {
		switch line.EveryUnit {
		case ExpirationUnitDefault:
			expiration := line.Keep
			if expiration > maxExpiration.Keep {
				maxExpiration.Keep = expiration
				maxExpiration.Original = line.Original
			}

		case ExpirationUnitFile:
			if exp.FileCount%line.Every == 0 {
				expiration := line.Keep
				if expiration > maxExpiration.Keep {
					maxExpiration.Keep = expiration
					maxExpiration.Original = line.Original
				}
			}

		case ExpirationUnitDay:
			// num of days between refdate and now
			diff := modTime.Sub(exp.ReferenceDate)
			days := int(diff.Hours() / 24)
			if days%line.Every == 0 {
				expiration := line.Keep
				if expiration > maxExpiration.Keep {
					maxExpiration.Keep = expiration
					maxExpiration.Original = line.Original
				}
			}
		}
	}

	return maxExpiration
}

func (exp *Expiration) String() string {
	lines := make([]string, len(exp.Lines))
	for i, line := range exp.Lines {
		lines[i] = line.Original
	}
	return strings.Join(lines, ", ")
}
