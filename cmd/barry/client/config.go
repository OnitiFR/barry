package client

// RootConfig describes client application config parameters
type RootConfig struct {
	ConfigFile string

	URL string
	Key string

	Trace bool
	Time  bool
}

// GlobalHome is the user HOME
var GlobalHome string

// GlobalCfgFile is the config filename
var GlobalCfgFile string

// GlobalAPI is the global API instance
var GlobalAPI *API

// GlobalConfig is the global RootConfig instance
var GlobalConfig *RootConfig
