package instances

import (
	"context"
	"os"

	"we.com/dolphin/registry/etcdkey"
	"we.com/dolphin/registry/generic"
	"we.com/dolphin/types"
)

// Registry  instance client
type Registry struct {
	stage   types.Stage
	baseDir string
	store   generic.Interface
}

// NewRegistry returns a Registry
func NewRegistry(stage types.Stage) (*Registry, error) {
	baseDir := etcdkey.DeployDir(stage)
	store, err := generic.GetStoreInstance(etcdkey.DeployDir(stage), false)
	if err != nil {
		return nil, err
	}
	return &Registry{
		stage:   stage,
		baseDir: baseDir,
		store:   store,
	}, nil
}

func (r *Registry) getStore() (generic.Interface, error) {
	return generic.GetStoreInstance(r.baseDir, false)
}

// GetInstances return a list of Instances(may be stopped), collected from store
func (r *Registry) GetInstances(key types.DeployKey) ([]*types.Instance, error) {
	path := etcdkey.DeployInstanceDirOfKey(r.stage, key)
	ret := []*types.Instance{}

	err := r.store.List(context.Background(), path, generic.Everything, &ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

// SetHostDeploySpecOfKey set host deploy spec of key to spec
func (r *Registry) SetHostDeploySpecOfKey(hostID types.HostID, key types.DeployKey, spec types.DeploySpec) error {
	path := etcdkey.DeployHostExpectPathOf(r.stage, hostID, key)

	return r.store.Update(context.Background(), path, spec, nil, 0)
}

// GetHostDeploySpecOfKey get current host deploy spec of key
func (r *Registry) GetHostDeploySpecOfKey(hostID types.HostID, key types.DeployKey) (*types.DeploySpec, error) {
	path := etcdkey.DeployHostExpectPathOf(r.stage, hostID, key)

	ret := types.DeploySpec{}

	err := r.store.Get(context.Background(), path, &ret, false)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &ret, nil
}
