package host

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/golang/glog"
	"we.com/dolphin/config"
	"we.com/dolphin/registry/hosts"
	"we.com/dolphin/types"
	"we.com/jiabiao/common/yaml"
)

// ConfigManager manager java config
type ConfigManager struct {
	lock        sync.RWMutex
	hostConfigs map[types.Stage]map[types.UUID]*types.HostConfig
	// key  if filename, value fmt.Sprintf("%v:%v", env, uuid)
	fileMap map[string]string
}

var (
	manager = &ConfigManager{
		hostConfigs: map[types.Stage]map[types.UUID]*types.HostConfig{},
		fileMap:     map[string]string{},
	}
)

func init() {
	config.AddConfigManager(manager)
}

// LoadConfig  load config from file and alse implements  HostConfigManger interface
func (m *ConfigManager) LoadConfig(filename string) error {
	if !strings.HasSuffix(filename, ".yml") && !strings.HasSuffix(filename, ".json") {
		glog.Warningf("filename should has suffix .yml or .json: got %v", filename)
		return nil
	}

	dir := filepath.Dir(filename)
	envStr := filepath.Base(dir)

	reader, err := os.Open(filename)
	if err != nil {
		return err
	}

	decoder := yaml.NewYAMLOrJSONDecoder(reader, 4)
	ret := types.HostConfig{}

	err = decoder.Decode(&ret)
	if err != nil {
		return err
	}

	if ret.HostName == "" {
		file := filepath.Base(filename)
		idx := strings.LastIndex(file, ".")
		if idx < 0 {
			glog.Fatalf("bug")
		}
		ret.HostName = file[0:idx]
	}

	env := types.Stage(envStr)

	m.lock.Lock()
	defer m.lock.Unlock()

	envConfig, ok := m.hostConfigs[env]
	if !ok {
		envConfig = map[types.UUID]*types.HostConfig{}
		m.hostConfigs[env] = envConfig
	}

	old, ok := envConfig[types.UUID(ret.HostName)]
	if ok {
		glog.Infof("overwrite config for host (%v, %v)", old, ret)
	}

	if ret.Stage != "" && ret.HostName != "" {
		r, _ := hosts.NewRegistry(ret.Stage)
		r.SaveConfig(&ret)
	}

	envConfig[types.UUID(ret.HostName)] = &ret

	m.fileMap[filename] = fmt.Sprintf("%v:%v", env, ret.HostName)
	return nil
}

// DeleteConfig remote configuration loaded from file filename
func (m *ConfigManager) DeleteConfig(filename string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	id, ok := m.fileMap[filename]
	if !ok {
		return
	}

	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 {
		glog.Errorf("expect value contain at least one ':', got: %v", id)
		return
	}

	env := types.Stage(parts[0])
	host := types.UUID(parts[1])

	envConfig, ok := m.hostConfigs[env]
	if !ok {
		glog.Errorf("expect host config contains key: %v, but not found", env)
		return
	}
	delete(envConfig, host)
	delete(m.fileMap, filename)

	if len(envConfig) == 0 {
		delete(m.hostConfigs, env)
	}
}

// MonitorDir which folder content change  this manager is interest to
func (m *ConfigManager) MonitorDir() string {
	return "hosts"
}

// GetHostConfigs returns a map of configed hosts
func (m *ConfigManager) GetHostConfigs(env types.Stage) map[types.UUID]types.HostConfig {
	ret := map[types.UUID]types.HostConfig{}
	m.lock.RLock()
	m.lock.RUnlock()

	envConfig, ok := m.hostConfigs[env]
	if !ok {
		return nil
	}
	for host, cfg := range envConfig {
		ret[host] = *cfg
	}

	return ret
}

// ENVs returns list of known envs
func (m *ConfigManager) ENVs() []types.Stage {
	var ret []types.Stage
	for e := range m.hostConfigs {
		ret = append(ret, e)
	}
	return ret
}
