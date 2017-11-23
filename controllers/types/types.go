package types

import (
	"context"
	"time"

	"we.com/dolphin/types"
	"we.com/jiabiao/common/labels"
)

// Selector select a list of hosts, meeting the given condidtion
type Selector interface {
	Select(s labels.Selector) (hosts []types.HostID, err error)
}

// HostEvaluator  given a host, get its score
type HostEvaluator interface {
	// Evaluat: evaluat a host for current state
	// a negtive score indicator hosts is under condition, and we should not schedual any
	// not instance to it
	Evaluat(hostID types.HostID) (score float64)
}

// Require requirement host should meet
type Require struct {
	HostSelector labels.Selector
	Resource     types.DeployResource
}

// Scheduler schedual a depoly to list of hosts
type Scheduler interface {
	NextHost() (hostid types.HostID, err error)
}

// InstanceInfor query instance  infos
type InstanceInfor interface {
	Start(ctx context.Context) error
	ListDeploykeys() []types.DeployKey
	NewStartedInstance(key types.DeployKey, d time.Duration) []*types.Instance
	NewStoppedInstance(key types.DeployKey, d time.Duration) []*types.Instance
	RunningInstance(key types.DeployKey) map[types.InstanceID]*types.Instance
	RecentStoppedInstance(key types.DeployKey) map[types.InstanceID]*types.Instance
	GetInstance(key types.DeployKey, insID types.InstanceID) *types.Instance
}

// HostConfigManager host  deploy config manger
type HostConfigManager interface {
	ListDeployKeys() []types.DeployKey
	GetHostConfig(key types.DeployKey, hostID types.HostID) *types.DeploySpec
	ListHostConfigs(key types.DeployKey) map[types.HostID]types.DeploySpec
	DeleteHostConfigs(key types.DeployKey, hostIDs ...types.HostID) error
	SetHostConfig(key types.DeployKey, hostID types.HostID, cfg types.DeploySpec) error
	Destroy()
}

// DeployConfigManager deploy config manager
type DeployConfigManager interface {
	GetDeployConfig(key types.DeployKey) *types.DeployConfig
	GetDeployResouce(key types.DeployKey) *types.DeployResource
	Destroy()
}
