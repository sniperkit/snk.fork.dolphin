/*
Sniperkit-Bot
- Status: analyzed
*/

package java

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/golang/glog"
	"we.com/dolphin/registry/etcdkey"
	"we.com/dolphin/registry/generic"
	"we.com/dolphin/registry/watch"
	"we.com/dolphin/types"
	"we.com/dolphin/types/ins/java"
)

type diProvider struct {
	baseDir    string
	interfaces map[types.DeployName]java.ProbeInterfaces
	lock       sync.RWMutex
}

// NewDIProvider create java.ProbeInterfaceProvider
func NewDIProvider(stage types.Stage) (java.ProbeInterfaceProvider, error) {
	di := &diProvider{
		baseDir: etcdkey.JavaProbeDir(stage),
	}

	err := di.loadInterfaces()
	if err != nil {
		return nil, err
	}

	return di, nil
}

func (dp *diProvider) GetProbeInterfaces(key types.DeployName) java.ProbeInterfaces {
	dp.lock.RLock()
	tmp := dp.interfaces[key]
	dp.lock.RUnlock()

	ret := java.ProbeInterfaces{}

	for k, v := range tmp {
		ret[k] = v
	}

	return ret
}

func (dp *diProvider) loadInterfaces() error {
	ret := map[string]java.ProbeInterfaces{}
	store, err := generic.GetStoreInstance(dp.baseDir, false)
	if err != nil {
		return err
	}

	if err := store.List(context.Background(), "", generic.Everything, ret); err != nil {
		return err
	}

	ifs := make(map[types.DeployName]java.ProbeInterfaces, len(ret))
	for k, v := range ret {
		if len(v) > 0 {
			ifs[types.DeployName(k)] = v
		}
	}

	dp.lock.Lock()
	defer dp.lock.Unlock()

	dp.interfaces = ifs
	return nil
}

func (dp *diProvider) watch(ctx context.Context) error {
	store, err := generic.GetStoreInstance(dp.baseDir, false)
	if err != nil {
		return err
	}

	h := func(e watch.Event) error {
		dfs, ok := e.Object.(*java.ProbeInterfaces)
		if !ok {
			glog.Fatalf("event object must be an instance of *java.ProbeInterfaces, got %T", e.Object)
		}
		if dfs == nil {
			return nil
		}

		name := types.DeployName(e.Key)

		switch e.Type {
		case watch.Added, watch.Modified:
			dp.lock.Lock()
			dp.interfaces[name] = *dfs
			dp.lock.Unlock()

		case watch.Deleted:
			dp.lock.Lock()
			delete(dp.interfaces, name)
			dp.lock.Unlock()

		}

		return nil
	}

	typ := reflect.TypeOf(java.ProbeInterfaces{})
	w, err := store.Watch(ctx, "", generic.Everything, true, typ)
	if err != nil {
		return err
	}
	defer w.Stop()
	c := w.ResultChan()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-c:
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
		}
	}
}
