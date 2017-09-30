package template

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/golang/glog"
	"github.com/influxdata/telegraf/deploy/conf/backends"
	"github.com/kelseyhightower/memkv"
	"we.com/jiabiao/common/yaml"
)

// Config  globle config of the tmeplate engine
type Config struct {
	CheckCmd      string
	ReloadCmd     string
	ConfDir       string
	ConfigDir     string
	KeepStageFile bool
	Noop          bool
	Prefix        string
	StoreClient   backends.StoreClient
	SyncOnly      bool
	TemplateDir   string
}

// Resource is the representation of a parsed template resource.
type Resource struct {
	CheckCmd      string      `json:"checkCmd,omitempty"`
	Dest          string      `json:"dest,omitempty"`
	FileMode      os.FileMode `json:"fileMode,omitempty"`
	Gid           int         `json:"gid,omitempty"`
	Keys          []string    `json:"keys,omitempty"`
	Mode          string      `json:"mode,omitempty"`
	Prefix        string      `json:"prefix,omitempty"`
	ReloadCmd     string      `json:"reloadCmd,omitempty"`
	Src           string      `json:"src,omitempty"`
	StageFile     *os.File    `json:"stageFile,omitempty"`
	UID           int         `json:"uid,omitempty"`
	funcMap       map[string]interface{}
	lastIndex     uint64
	keepStageFile bool
	noop          bool
	store         memkv.Store
	storeClient   backends.StoreClient
	syncOnly      bool
}

// UnmarshalJSON  implements json.Unmarshaler interface
func (t *Resource) UnmarshalJSON(data []byte) error {
	type plain Resource

	// Set the default uid and gid so we can determine if it was
	// unset from configuration.
	t.Gid = -1
	t.UID = -1

	if err := json.Unmarshal(data, (*plain)(t)); err != nil {
		return err
	}

	return nil
}

// ErrEmptySrc  empty src template error
var ErrEmptySrc = errors.New("empty src template")

// NewResource creates a Resource.
func NewResource(path string, config Config) (*Resource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return NewResourceFromReader(f, config)
}

// NewResourceFromReader  parse a new Resource from the given io.reader
func NewResourceFromReader(reader io.Reader, config Config) (*Resource, error) {
	if config.StoreClient == nil {
		return nil, errors.New("a valid StoreClient is required")
	}

	var tr Resource
	decoder := yaml.NewYAMLOrJSONDecoder(reader, 4)
	err := decoder.Decode(&tr)
	if err != nil {
		return nil, fmt.Errorf("Cannot process template resource - %s", err.Error())
	}

	tr.keepStageFile = config.KeepStageFile
	tr.noop = config.Noop
	tr.storeClient = config.StoreClient
	tr.funcMap = newFuncMap()
	tr.store = memkv.New()
	tr.syncOnly = config.SyncOnly
	addFuncs(tr.funcMap, tr.store.FuncMap)

	if config.Prefix != "" {
		tr.Prefix = config.Prefix
	}

	if !strings.HasPrefix(tr.Prefix, "/") {
		tr.Prefix = "/" + tr.Prefix
	}

	if tr.Src == "" {
		return nil, ErrEmptySrc
	}

	if tr.UID == -1 {
		tr.UID = os.Geteuid()
	}

	if tr.Gid == -1 {
		tr.Gid = os.Getegid()
	}

	tr.Src = filepath.Join(config.TemplateDir, tr.Src)
	return &tr, nil
}

// setVars sets the Vars for template resource.
func (t *Resource) setVars() error {
	var err error
	glog.V(10).Info("Retrieving keys from store")
	glog.V(10).Infof("Key prefix set to %v", t.Prefix)

	result, err := t.storeClient.GetValues(appendPrefix(t.Prefix, t.Keys))
	if err != nil {
		return err
	}
	glog.V(10).Infof("Got the following map from store: %v", result)

	t.store.Purge()

	for k, v := range result {
		t.store.Set(path.Join("/", strings.TrimPrefix(k, t.Prefix)), v)
	}
	return nil
}

// createStageFile stages the src configuration file by processing the src
// template and setting the desired owner, group, and mode. It also sets the
// StageFile for the template resource.
// It returns an error if any.
func (t *Resource) createStageFile() error {
	glog.V(10).Infof("Using source template %v", t.Src)

	if !isFileExist(t.Src) {
		return errors.New("Missing template: " + t.Src)
	}

	glog.V(10).Info("Compiling source template " + t.Src)

	tmpl, err := template.New(filepath.Base(t.Src)).Funcs(t.funcMap).ParseFiles(t.Src)
	if err != nil {
		return fmt.Errorf("Unable to process template %s, %s", t.Src, err)
	}

	// create TempFile in Dest directory to avoid cross-filesystem issues
	temp, err := ioutil.TempFile(filepath.Dir(t.Dest), "."+filepath.Base(t.Dest))
	if err != nil {
		return err
	}

	if err = tmpl.Execute(temp, nil); err != nil {
		temp.Close()
		os.Remove(temp.Name())
		return err
	}
	defer temp.Close()

	// Set the owner, group, and mode on the stage file now to make it easier to
	// compare against the destination configuration file later.
	os.Chmod(temp.Name(), t.FileMode)
	os.Chown(temp.Name(), t.UID, t.Gid)
	t.StageFile = temp
	return nil
}

// sync compares the staged and dest config files and attempts to sync them
// if they differ. sync will run a config check command if set before
// overwriting the target config file. Finally, sync will run a reload command
// if set to have the application or service pick up the changes.
// It returns an error if any.
func (t *Resource) sync() error {
	staged := t.StageFile.Name()
	if t.keepStageFile {
		glog.Info("Keeping staged file: " + staged)
	} else {
		defer os.Remove(staged)
	}

	glog.V(10).Info("Comparing candidate config to " + t.Dest)
	ok, err := sameConfig(staged, t.Dest)
	if err != nil {
		glog.Error(err.Error())
	}
	if t.noop {
		glog.Warning("Noop mode enabled. " + t.Dest + " will not be modified")
		return nil
	}
	if !ok {
		glog.Info("Target config " + t.Dest + " out of sync")
		if !t.syncOnly && t.CheckCmd != "" {
			if err := t.check(); err != nil {
				return errors.New("Config check failed: " + err.Error())
			}
		}
		glog.V(10).Infof("Overwriting target config %v", t.Dest)
		err := os.Rename(staged, t.Dest)
		if err != nil {
			if strings.Contains(err.Error(), "device or resource busy") {
				glog.V(10).Info("Rename failed - target is likely a mount. Trying to write instead")
				// try to open the file and write to it
				var contents []byte
				var rerr error
				contents, rerr = ioutil.ReadFile(staged)
				if rerr != nil {
					return rerr
				}
				err := ioutil.WriteFile(t.Dest, contents, t.FileMode)
				// make sure owner and group match the temp file, in case the file was created with WriteFile
				os.Chown(t.Dest, t.UID, t.Gid)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}
		if !t.syncOnly && t.ReloadCmd != "" {
			if err := t.reload(); err != nil {
				return err
			}
		}
		glog.Info("Target config " + t.Dest + " has been updated")
	} else {
		glog.V(10).Info("Target config " + t.Dest + " in sync")
	}
	return nil
}

// check executes the check command to validate the staged config file. The
// command is modified so that any references to src template are substituted
// with a string representing the full path of the staged file. This allows the
// check to be run on the staged file before overwriting the destination config
// file.
// It returns nil if the check command returns 0 and there are no other errors.
func (t *Resource) check() error {
	var cmdBuffer bytes.Buffer
	data := make(map[string]string)
	data["src"] = t.StageFile.Name()
	tmpl, err := template.New("checkcmd").Parse(t.CheckCmd)
	if err != nil {
		return err
	}
	if err := tmpl.Execute(&cmdBuffer, data); err != nil {
		return err
	}
	glog.V(10).Info("Running " + cmdBuffer.String())
	c := exec.Command("/bin/sh", "-c", cmdBuffer.String())
	output, err := c.CombinedOutput()
	if err != nil {
		glog.Error(fmt.Sprintf("%q", string(output)))
		return err
	}
	glog.V(10).Info(fmt.Sprintf("%q", string(output)))
	return nil
}

// reload executes the reload command.
// It returns nil if the reload command returns 0.
func (t *Resource) reload() error {
	glog.V(10).Info("Running " + t.ReloadCmd)
	c := exec.Command("/bin/sh", "-c", t.ReloadCmd)
	output, err := c.CombinedOutput()
	if err != nil {
		glog.Error(fmt.Sprintf("%q", string(output)))
		return err
	}
	glog.V(10).Info(fmt.Sprintf("%q", string(output)))
	return nil
}

// process is a convenience function that wraps calls to the three main tasks
// required to keep local configuration files in sync. First we gather vars
// from the store, then we stage a candidate configuration file, and finally sync
// things up.
// It returns an error if any.
func (t *Resource) process() error {
	if err := t.setFileMode(); err != nil {
		return err
	}
	if err := t.setVars(); err != nil {
		return err
	}
	if err := t.createStageFile(); err != nil {
		return err
	}
	if err := t.sync(); err != nil {
		return err
	}
	return nil
}

// setFileMode sets the FileMode.
func (t *Resource) setFileMode() error {
	if t.Mode == "" {
		if !isFileExist(t.Dest) {
			t.FileMode = 0644
		} else {
			fi, err := os.Stat(t.Dest)
			if err != nil {
				return err
			}
			t.FileMode = fi.Mode()
		}
	} else {
		mode, err := strconv.ParseUint(t.Mode, 0, 32)
		if err != nil {
			return err
		}
		t.FileMode = os.FileMode(mode)
	}
	return nil
}
