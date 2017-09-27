package replica

import (
	"context"
	"sync"
	"time"

	ctypes "we.com/dolphin/controllers/types"
	"we.com/dolphin/registry/etcdkey"
	"we.com/dolphin/registry/generic"
	"we.com/dolphin/registry/instances"
	"we.com/dolphin/types"
)

type deployInfo struct {
	Key       types.DeployKey
	HostID    types.HostID
	Instances map[types.DeployVer]int
}

type controller struct {
	opt       option
	deployed  map[types.DeployKey]*types.DeployConfig
	deploying map[types.DeployKey]*types.DeployConfig

	// auctual deployment  relies on agent's report
	// so, if agent connot update is staus, these values cannot be correct
	actualDeploymentOfHost   map[types.HostID]*types.HostDeployment
	actualDeploymentOfConfig map[types.DeployKey][]deployInfo

	// maxtries time, when deploy a config  fails
	maxTries    int
	concurrency int
	Stage       types.Stage
	lock        sync.RWMutex

	store *instances.Registry

	schedualer ctypes.Schedualer
	updateTime time.Time
}

func (c *controller) getDeployConfig(key types.DeployKey) (*types.DeployConfig, error) {
	path := etcdkey.DepoyConfigOfKey(c.Stage, key)

	ret := types.DeployConfig{}
	err := c.store.Get(context.Background(), path, &ret, false)
	if err != nil {
		return nil, err
	}

	return &ret, nil
}

func (c *controller) updateActualDeployment() error {
	c.lock.Lock()
	c.lock.Unlock()

	path := etcdkey.DeployActualDir(c.Stage)

	ret := []*types.DeploySpec{}

	err := c.store.List(context.Background(), path, generic.Everything, &ret)
	if err != nil {
		return err
	}

	dh := map[types.HostID]*types.HostDeployment{}
	dmc := map[types.DeployKey][]deployInfo{}
	for _, v := range ret {
		dh[v.HostID] = v
		for k, spec := range v.DeploySpecs {
			diArr, ok := dmc[k]
			if !ok {
				diArr = []deployInfo{}
			}

			ins := map[types.DeployVer]int{}
			di := deployInfo{
				Key:    k,
				HostID: v.HostID,
			}
			for ver, inf := range spec.Info {
				ins[ver] = inf.Num
			}

			diArr = append(diArr, di)
			dmc[k] = diArr
		}
	}

	c.actualDeploymentOfConfig = dmc
	c.actualDeploymentOfHost = dh
	c.updateTime = time.Now()
	return nil
}

func (c *controller) updateActualDeploymentOfDeploykey(key types.DeployKey) error {

	return nil
}

func (c *controller) getRunningProcess(key types.DeployKey) {

}

// newDeployment deployment a new project
func (c *controller) newDeployment(config *types.DeployConfig, dryrun bool) error {
	if len(c.deploying) >= c.opt.cocurrency {
		return ErrCocurrencyFull
	}

	req := toRequire(config)

	hosts, err := c.schedualer.Schedual(req)
	if err != nil {
		return err
	}

	return nil
}

func (c *controller) update(key types.DeployKey, ver string, up types.UpdateOption) error {
	dc, err := c.getDeployConfig(key)
	if err != nil {
		return err
	}

	return nil
}

func toRequire(dc *types.DeployConfig) *ctypes.Require {
	s := dc.GetSelector()

	ret := &ctypes.Require{
		HostSelector: s,
		Resource:     dc.ResourceRequired,
	}

	return ret
}
