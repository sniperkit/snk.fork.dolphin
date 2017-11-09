package types

import (
	"regexp"
	"time"

	"github.com/golang/glog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	mytime "we.com/jiabiao/common/time"
)

type zkPathConfig struct {
	Base      string         `json:"base,omitempty"`
	RegexpStr string         `json:"regexp,omitempty"`
	Regexp    *regexp.Regexp `json:"-"`
}

// EnvConfig  zk config of an env
type EnvConfig struct {
	ENV         string          `json:"env,omitempty"`
	ZKServers   []string        `json:"zkServers,omitempty"`
	DialTimeout mytime.Duration `json:"dialTimeout,omitempty"`
	ZKPaths     []zkPathConfig  `json:"zkPaths,omitempty"`
}

// Config  zk config
type Config struct {
	Timeout mytime.Duration      `json:"timeout,omitempty"`
	Envs    map[string]EnvConfig `json:"env,omitempty"`
}

// Validate check if ec is valid
func (ec *EnvConfig) Validate() error {
	if ec == nil {
		return nil
	}

	e := ec.ENV
	var merr *multierror.Error
	if len(ec.ZKServers) == 0 {
		merr = multierror.Append(merr, errors.Errorf("at least one zk server should config for %v", e))
	}

	to := time.Duration(ec.DialTimeout)
	if int64(to) < 0 {
		merr = multierror.Append(merr, errors.Errorf("%v: dialtimeout less than 0", e))
	}

	if len(ec.ZKPaths) == 0 {
		glog.Warningf("%v has 0 zkpath configed, will be ignored", e)
	}

	for _, v := range ec.ZKPaths {
		if v.RegexpStr != "" {
			re, err := regexp.Compile(v.RegexpStr)
			if err != nil {
				merr = multierror.Append(merr, errors.Errorf("regexp of %v: %v: %v", e, v.RegexpStr, err))
			} else {
				v.Regexp = re
			}
		}
	}

	return merr.ErrorOrNil()
}

// Validate check if Config is valid
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config is nil")
	}

	to := time.Duration(c.Timeout)
	if int64(to) < 0 {
		glog.Warningf("timeout less than 0")
	}

	if to == 0 {
		c.Timeout = mytime.Duration(10 * time.Second)
	}

	var merr *multierror.Error
	for e, cfg := range c.Envs {
		cfg.ENV = e
		if err := cfg.Validate(); err != nil {
			merr = multierror.Append(merr, err)
		}
	}

	return merr.ErrorOrNil()
}
