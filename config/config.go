package config

import (
	"fmt"

	"github.com/golang/glog"
	"we.com/dolphin/types"
)

// ConfigManager  super config manager interface
type ConfigManager interface {
	// which folder content change  this manager is interest to
	MonitorDir() string

	// LoadConfig load  deploy config from the file
	LoadConfig(filename string) error

	// DeleteConfig delete config loaded from this file
	DeleteConfig(filename string)
}

// DeployConfigManger provider deploy config of a type type
type DeployConfigManger interface {
	ENVs() []types.Stage
	GetProjecType() types.ProjectType

	//  return all configed deploy of env
	GetDeploy(env types.Stage) map[types.UUID]types.DeployConfig
}

// HostConfigManager  managers host configs
type HostConfigManager interface {
	Stags() []types.Stage

	// GetHostConfigs returns a map of configed hosts
	GetHostConfigs(env types.Stage) map[types.UUID]types.HostConfig
}

var (
	configManagers      = map[types.UUID]ConfigManager{}
	deployConfigManager = map[types.ProjectType]DeployConfigManger{}
	hostConfigManager   HostConfigManager
)

// GetDeployConfigsOfType return deploy config of this env and type
func GetDeployConfigsOfType(env types.Stage, typ types.ProjectType) map[types.UUID]types.DeployConfig {
	m, ok := deployConfigManager[typ]
	if !ok {
		glog.V(10).Infof("knowns project type for deploy managers: %v", typ)
		return nil
	}

	return m.GetDeploy(env)
}

// GetDeployConfigsOfCluster return deploy config of this env,  type and cluster
func GetDeployConfigsOfCluster(env types.Stage, typ types.ProjectType, cluster types.UUID) *types.DeployConfig {
	m := GetDeployConfigsOfType(env, typ)
	if m == nil {
		return nil
	}

	// make a copy
	dc, ok := m[cluster]
	if !ok {
		return nil
	}

	return &dc
}

// GetDeployConfigs return deploy config of this env and type
func GetDeployConfigs(env types.Stage) map[types.ProjectType]map[types.UUID]types.DeployConfig {
	ret := map[types.ProjectType]map[types.UUID]types.DeployConfig{}
	for t, m := range deployConfigManager {
		ret[t] = m.GetDeploy(env)
	}

	return ret
}

// GetHostConfigs return hostconfig of env
func GetHostConfigs(env types.Stage) map[types.UUID]types.HostConfig {
	return hostConfigManager.GetHostConfigs(env)
}

// GetDeployConfigManager meant to be used by file monitor only
func GetDeployConfigManager() map[types.ProjectType]DeployConfigManger {
	return deployConfigManager
}

// AddConfigManager register a config manager
func AddConfigManager(cm ConfigManager) {
	key := types.EmptyUUID
	switch manager := cm.(type) {
	case DeployConfigManger:
		key = types.UUID(fmt.Sprintf("deploy:%v", manager.GetProjecType()))
		deployConfigManager[manager.GetProjecType()] = manager
	case HostConfigManager:
		key = types.UUID("hostconfigmanager")
		hostConfigManager = manager
	default:
		glog.Fatalf("unknown config manager type: %v, %v", manager.MonitorDir(), manager)
	}
	configManagers[key] = cm
}

// GetConfigManagers return a map of  config managers
func GetConfigManagers() map[types.UUID]ConfigManager {
	return configManagers
}
