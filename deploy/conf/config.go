package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"we.com/dolphin/deploy/conf/template"
	"we.com/jiabiao/common/yaml"
)

var (
	configFile        = ""
	defaultConfigFile = "/etc/confd/confd.toml"
	confdir           string
	config            Config // holds the global confd config.
	interval          int
	keepStageFile     bool
	noop              bool
	onetime           bool
	prefix            string
	printVersion      bool
	templateConfig    template.Config
)

// A Config structure is used to configure confd.
type Config struct {
	ConfDir  string `json:"conf,omitempty"`
	Interval int    `json:"interval,omitempty"`
	Noop     bool   `json:"noop,omitempty"`
	Prefix   string `json:"prefix,omitempty"`
	SyncOnly bool   `json:"syncOnly,omitempty"`
	Watch    bool   `json:"watch,omitempty"`
}

// initConfig initializes the confd configuration by first setting defaults,
// then overriding settings from the confd config file, then overriding
// settings from environment variables, and finally overriding
// settings from flags set on the command line.
// It returns an error if any.
func initConfig() error {
	if configFile == "" {
		if _, err := os.Stat(defaultConfigFile); !os.IsNotExist(err) {
			configFile = defaultConfigFile
		}
	}
	// Set defaults.
	config = Config{
		ConfDir:  "/etc/confd",
		Interval: 600,
		Prefix:   "",
	}
	// Update config from the TOML configuration file.
	if configFile == "" {
		glog.V(10).Info("Skipping confd config file.")
	} else {
		glog.V(10).Info("Loading " + configFile)
		configBytes, err := ioutil.ReadFile(configFile)
		if err != nil {
			return err
		}

		decode := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(configBytes), 4)
		if err := decode.Decode(&config); err != nil {
			return err
		}
	}

	// Template configuration.
	templateConfig = template.Config{
		ConfDir:       config.ConfDir,
		ConfigDir:     filepath.Join(config.ConfDir, "conf.d"),
		KeepStageFile: keepStageFile,
		Noop:          config.Noop,
		Prefix:        config.Prefix,
		SyncOnly:      config.SyncOnly,
		TemplateDir:   filepath.Join(config.ConfDir, "templates"),
	}
	return nil
}
