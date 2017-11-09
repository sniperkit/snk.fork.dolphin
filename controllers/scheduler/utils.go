package scheduler

import (
	"context"
	"fmt"
	"reflect"

	"github.com/golang/glog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	ctypes "we.com/dolphin/controllers/types"
	"we.com/dolphin/registry/etcdkey"
	"we.com/dolphin/registry/generic"
	"we.com/dolphin/registry/watch"
	"we.com/dolphin/types"
	"we.com/dolphin/types/ins/registry"
)

func loadDeployConfig(stage types.Stage, key types.DeployKey) (map[types.HostID]*types.DeploySpec, error) {
	dir := etcdkey.DeployHostExpectDir(stage)
	ret := make(map[types.HostID]*types.DeploySpec)
	store, err := getStore()
	if err != nil {
		return nil, err
	}

	hosts, err := store.ListKeys(context.Background(), dir)
	if err != nil {
		return nil, err
	}

	var merr *multierror.Error
	for _, h := range hosts {
		spec := types.DeploySpec{}
		hid := types.HostID(h)
		key := etcdkey.DeployHostExpectPathOf(stage, hid, key)
		if err := store.Get(context.Background(), key, &spec, true); err != nil {
			merr = multierror.Append(merr, err)
		} else {
			ret[hid] = &spec
		}
	}

	return ret, merr.ErrorOrNil()
}

func getRunningInstances(stage types.Stage, key types.DeployKey) ([]*types.Instance, error) {
	path := etcdkey.DeployInstanceDirOfKey(stage, key)
	ret := []*types.Instance{}

	store, err := getStore()
	if err != nil {
		return nil, err
	}

	if err := store.List(context.Background(), path, generic.Everything, &ret); err != nil {
		return nil, err
	}

	// skip stopped instances
	for _, v := range ret {
		if v.LifeCycle != types.LCStopped {
			ret = append(ret, v)
		}
	}
	return ret, nil
}

// getNewHosts return hosts which has no deployment of key
func getNewHosts(c []types.HostID, ins []*types.Instance) []types.HostID {
	ret := []types.HostID{}
outer:
	for _, v := range c {
		for _, i := range ins {
			if i.HostID == v {
				continue outer
			}
		}
		ret = append(ret, v)
	}

	return ret
}

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

func watchEvent(ctx context.Context, path string, pred generic.SelectionPredicate, expectType reflect.Type, h watch.EventHandler) error {
	store, err := getStore()
	if err != nil {
		glog.Errorf("getStoreInstance error: %s", err)
		err = errors.Wrap(err, "getStoreInstance")
		return err
	}

	watcher, err := store.Watch(ctx, path, generic.Everything, true, expectType)
	if err != nil {
		return err
	}
	defer watcher.Stop()
	for {
		select {
		case event := <-watcher.ResultChan():
			glog.V(10).Infof("receive an instance event: %v", event)
			switch event.Type {
			case watch.Error:
				err, ok := event.Object.(error)
				if !ok {
					glog.Warningf("event type if error, but event.Object is not an error")
					err = fmt.Errorf("watch got error :%v", event.Object)
				}
				glog.Warningf("watch err: %v", err)
			default:
				if err := h(event); err != nil {
					return err
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
