package scheduler

import (
	"context"

	"github.com/pkg/errors"
	ctypes "we.com/dolphin/controllers/types"
	"we.com/dolphin/registry/etcdkey"
	"we.com/dolphin/registry/generic"
	"we.com/dolphin/types"
	"we.com/dolphin/types/ins/registry"
)

func getStore() (generic.Interface, error) {
	return generic.GetStoreInstance(etcdkey.BaseDir(), false)
}

func toRequire(dc *types.DeployConfig) *ctypes.Require {
	s := dc.GetSelector()

	rr := dc.ResourceRequired
	if rr == nil {
		t := registry.GetDefaultDeployResource(registry.StageType{Stage: dc.Stage, Type: dc.Type})
		if t != nil {
			rr = &t.Medium
		}
	}
	if rr == nil {
		rr = &types.DeployResource{}
	}

	ret := &ctypes.Require{
		HostSelector: s,
		Resource:     *rr,
	}

	return ret
}

func getHostStatus(stage types.Stage, hostID types.HostID) (*types.HostStatus, error) {
	path := etcdkey.HostStatPath(stage, hostID)
	ret := types.HostStatus{}

	if err := getObject(path, &ret); err != nil {
		return nil, err
	}

	return &ret, nil
}

func getHostInfo(stage types.Stage, hostID types.HostID) (*types.HostInfo, error) {
	path := etcdkey.HostStatPath(stage, hostID)
	ret := types.HostInfo{}

	if err := getObject(path, &ret); err != nil {
		return nil, err
	}

	return &ret, nil
}

func getObject(path string, obj interface{}) error {
	store, err := getStore()
	if err != nil {
		return err
	}

	if err := store.Get(context.Background(), path, obj, false); err != nil {
		return err
	}
	return nil
}

var (
	errStop = errors.New("stop watch")
)
