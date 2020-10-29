package topics

import (
	"fmt"
	"os"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/OnitiFR/barry/cmd/barry/client"
)

type tomlRootConfig struct {
	URL   string
	Key   string
	Trace bool
	Time  bool
}

// NewRootConfig reads configuration from filename and
// environment.
// Priority : CLI flag, config file, environment
func NewRootConfig(filename string) (*client.RootConfig, error) {
	rootConfig := &client.RootConfig{}

	envTrace, _ := strconv.ParseBool(os.Getenv("TRACE"))
	envTime, _ := strconv.ParseBool(os.Getenv("TIME"))

	tConfig := &tomlRootConfig{
		Trace: envTrace,
		Time:  envTime,
	}

	if stat, err := os.Stat(filename); err == nil {

		requiredMode, err := strconv.ParseInt("0600", 8, 32)
		if err != nil {
			return nil, err
		}

		if stat.Mode() != os.FileMode(requiredMode) {
			return nil, fmt.Errorf("%s: only the owner should be able to read/write this file (chmod 0600 %s)", filename, filename)
		}

		meta, err := toml.DecodeFile(filename, tConfig)

		if err != nil {
			return nil, err
		}

		undecoded := meta.Undecoded()
		for _, param := range undecoded {
			return nil, fmt.Errorf("unknown setting '%s'", param)
		}

		rootConfig.ConfigFile = filename
	} else {
		return nil, nil
	}

	flagTrace := rootCmd.PersistentFlags().Lookup("trace")
	flagTime := rootCmd.PersistentFlags().Lookup("time")

	if flagTrace.Changed {
		trace, _ := strconv.ParseBool(flagTrace.Value.String())
		tConfig.Trace = trace
	}
	if flagTime.Changed {
		time, _ := strconv.ParseBool(flagTime.Value.String())
		tConfig.Time = time
	}
	rootConfig.Trace = tConfig.Trace
	rootConfig.Time = tConfig.Time

	rootConfig.URL = tConfig.URL
	rootConfig.Key = tConfig.Key

	return rootConfig, nil
}
