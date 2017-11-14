package scheduler

import (
	"context"
	"os"
	"strings"
	"sync"

	multierror "github.com/hashicorp/go-multierror"
	ctypes "we.com/dolphin/controllers/types"
	"we.com/dolphin/registry/etcdkey"
	"we.com/dolphin/registry/generic"
	"we.com/dolphin/types"
)

// NewHCManager create a new host deploy config manger
func NewHCManager(stage types.Stage) (ctypes.HostConfigManager, error) {

	cfg, err := loadHostDeploySpec(stage)
	if err != nil {
		return nil, err
	}
	return &hcManager{
		stage: stage,
		cfgs:  cfg,
	}, nil
}

type hcManager struct {
	stage types.Stage
	cfgs  map[types.DeployKey]map[types.HostID]types.DeploySpec
	lock  sync.RWMutex
}

func (m *hcManager) ListDeployKeys() []types.DeployKey {
	m.lock.RLock()
	defer m.lock.RUnlock()
	ret := make([]types.DeployKey, 0, len(m.cfgs))
	for k := range m.cfgs {
		ret = append(ret, k)
	}
	return ret
}

func (m *hcManager) DeleteHostConfigs(key types.DeployKey, hostIDs ...types.HostID) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	keyMap, ok := m.cfgs[key]
	if !ok {
		return nil
	}

	var merr *multierror.Error
	for _, h := range hostIDs {
		if err := m.deleteHostDeployspec(h, key); err != nil {
			merr = multierror.Append(merr, err)
		}
		delete(keyMap, h)
	}
	if len(keyMap) == 0 {
		delete(m.cfgs, key)
	}
	return merr.ErrorOrNil()
}

func (m *hcManager) SetHostConfig(key types.DeployKey, hostID types.HostID, cfg types.DeploySpec) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	keyMap, ok := m.cfgs[key]
	if !ok {
		keyMap = map[types.HostID]types.DeploySpec{}
		m.cfgs[key] = keyMap
	}

	keyMap[hostID] = cfg

	return m.setHostDeployspec(hostID, key, &cfg)
}

func (m *hcManager) GetHostConfig(key types.DeployKey, hostID types.HostID) *types.DeploySpec {
	m.lock.RLock()
	defer m.lock.RUnlock()

	keyMap := m.cfgs[key]
	if len(keyMap) == 0 {
		return nil
	}

	ret := keyMap[hostID]

	return &ret
}

func (m *hcManager) ListHostConfigs(key types.DeployKey) map[types.HostID]types.DeploySpec {
	m.lock.RLock()
	defer m.lock.RUnlock()
	keyMap := m.cfgs[key]
	if len(keyMap) == 0 {
		return nil
	}
	ret := make(map[types.HostID]types.DeploySpec, len(keyMap))
	for k, v := range keyMap {
		ret[k] = v
	}

	return ret
}

func (m *hcManager) Destroy() {

}

func (m *hcManager) setHostDeployspec(hostID types.HostID, key types.DeployKey, spec *types.DeploySpec) error {
	path := etcdkey.DeployHostExpectPathOf(m.stage, hostID, key)
	store, err := generic.GetStoreInstance(path, false)
	if err != nil {
		return err
	}

	if spec != nil {
		return store.Update(context.Background(), path, spec, nil, 0)
	}

	return store.Delete(context.Background(), path, nil)
}

func (m *hcManager) getHostDeployspec(hostID types.HostID, key types.DeployKey) (*types.DeploySpec, error) {
	path := etcdkey.DeployHostExpectPathOf(m.stage, hostID, key)
	store, err := generic.GetStoreInstance(path, false)
	if err != nil {
		return nil, err
	}

	ret := types.DeploySpec{}

	err = store.Get(context.Background(), path, &ret, false)

	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &ret, nil
}

func (m *hcManager) deleteHostDeployspec(hostID types.HostID, key types.DeployKey) error {
	path := etcdkey.DeployHostExpectPathOf(m.stage, hostID, key)
	store, err := generic.GetStoreInstance(path, false)
	if err != nil {
		return err
	}

	return store.Delete(context.Background(), path, nil)
}

func loadHostDeploySpec(stage types.Stage) (map[types.DeployKey]map[types.HostID]types.DeploySpec, error) {
	dir := etcdkey.DeployHostExpectDir(stage)

	store, err := generic.GetStoreInstance(dir, false)
	if err != nil {
		return nil, err
	}

	out := make(map[string]types.DeploySpec)
	err = store.List(context.Background(), dir, generic.Everything, out)
	if err != nil {
		return nil, err
	}

	ret := map[types.DeployKey]map[types.HostID]types.DeploySpec{}

	for k, spec := range out {
		idx := strings.LastIndex(k, "/")
		if idx < 0 || idx+1 > len(k) {
			continue
		}
		key := types.DeployKey(k[:idx])
		hid := types.HostID(k[idx+1:])
		hmap, ok := ret[key]
		if !ok {
			hmap = map[types.HostID]types.DeploySpec{}
			ret[key] = hmap
		}
		hmap[hid] = spec
	}

	return ret, nil
}
