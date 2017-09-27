package instances

import (
	"context"
	"path/filepath"

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
	path := etcdkey.DeployActualDir(r.stage)
	hostids, err := r.store.ListKeys(context.Background(), path)
	if err != nil {
		return nil, err
	}

	ret := []*types.Instance{}
	for _, v := range hostids {
		hostPath := etcdkey.DeployActualDirOfHost(r.stage, types.HostID(v))
		keyPath := filepath.Join(string(hostPath), string(key))
		ins := []*types.Instance{}
		err := r.store.List(context.Background(), keyPath, generic.Everything, &ins)
		if err != nil {
			return nil, err
		}
		ret = append(ret, ins...)
	}

	return ret, nil
}
