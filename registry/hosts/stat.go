package hosts

import (
	"context"

	"we.com/dolphin/registry/etcdkey"
	"we.com/dolphin/registry/generic"
	"we.com/dolphin/types"
)

// NewRegistry  returns a registry, should call Stop when not used
func NewRegistry(stage types.Stage) (*Registry, error) {
	prefix := etcdkey.HostBaseDir(stage)
	s, err := generic.GetStoreInstance(prefix, false)
	if err != nil {
		return nil, err
	}
	return &Registry{
		stage: stage,
		store: s,
	}, nil
}

// Registry client
type Registry struct {
	stage types.Stage
	store generic.Interface
}

// GetResource return resource usage of a host
func (r *Registry) GetResource(hostID types.HostID) (*types.HostStatus, error) {
	key := etcdkey.HostStatPath(r.stage, hostID)
	out := &types.HostStatus{}
	err := r.store.Get(context.TODO(), key, out, false)
	return out, err
}

// UpdateResource return resource info of a host
func (r *Registry) UpdateResource(hostID types.HostID, hr *types.HostStatus) (*types.HostStatus, error) {
	key := etcdkey.HostStatPath(r.stage, hostID)

	// since  update is more frequent than create, so first try to update
	// if Node not exist, then create
	// hostResource  have a ttl of 2 mins
	err := r.store.Update(context.TODO(), key, hr, nil, 2*60)
	return hr, err
}

// GetHostInfoOfHostID return hostinfo of hostID,  or error
func (r *Registry) GetHostInfoOfHostID(hostID types.HostID) (*types.HostInfo, error) {
	key := etcdkey.HostInfoPath(r.stage, hostID)
	ret := types.HostInfo{}
	if err := r.store.Get(context.TODO(), key, &ret, false); err != nil {
		return nil, err
	}
	return &ret, nil
}

// SaveHostInfo  save or update   hostinfo of hostInfo.HostID, or err
func (r *Registry) SaveHostInfo(hi *types.HostInfo) error {
	key := etcdkey.HostInfoPath(r.stage, hi.HostID)
	return r.store.Update(context.TODO(), key, hi, nil, 0)
}

// DelHostInfo delete hostInfo from store
func (r *Registry) DelHostInfo(hostID types.HostID) error {
	key := etcdkey.HostInfoPath(r.stage, hostID)
	return r.store.Delete(context.TODO(), key, nil)
}
