package hosts

import (
	"context"

	"we.com/dolphin/registry/etcdkey"
	"we.com/dolphin/types"
)

// SaveConfig save or overwrite host config
func (r *Registry) SaveConfig(hc *types.HostConfig) error {
	key := etcdkey.HostConfigPath(r.stage, hc.HostName)
	return r.store.Update(context.TODO(), key, hc, nil, 0)
}

// GetConfig get config of a hostID, if config not exist return not exist error
func (r *Registry) GetConfig(hostname string) (*types.HostConfig, error) {
	key := etcdkey.HostConfigPath(r.stage, hostname)
	ret := types.HostConfig{}

	if err := r.store.Get(context.TODO(), key, &ret, false); err != nil {
		return nil, err
	}
	return &ret, nil
}
