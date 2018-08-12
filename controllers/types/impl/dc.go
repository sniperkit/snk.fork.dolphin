/*
Sniperkit-Bot
- Status: analyzed
*/

package impl

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/golang/glog"
	ctypes "we.com/dolphin/controllers/types"
	"we.com/dolphin/registry/etcdkey"
	"we.com/dolphin/registry/generic"
	"we.com/dolphin/registry/watch"
	"we.com/dolphin/types"
	"we.com/dolphin/types/ins/registry"
)

type deployConfigManager struct {
	stage types.Stage
	dc    map[types.DeployKey]*types.DeployConfig
	lock  sync.RWMutex
	df    context.CancelFunc
}

func (m *deployConfigManager) getStore() (generic.Interface, error) {
	prefix := etcdkey.StageBaseDir(m.stage)
	return generic.GetStoreInstance(prefix, false)
}

func (m *deployConfigManager) Destroy() {
	m.df()
}

func (m *deployConfigManager) load() error {
	store, err := m.getStore()
	if err != nil {
		return err
	}
	dir := etcdkey.DeployConfigDir(m.stage)
	ret := []*types.DeployConfig{}
	if err := store.List(context.Background(), dir, generic.Everything, &ret); err != nil {
		return err
	}
	for _, v := range ret {
		m.dc[v.Key()] = v
	}
	return nil
}

func (m *deployConfigManager) watch() error {
	ctx := context.Background()
	ctx, df := context.WithCancel(ctx)
	m.df = df
	return m.watchDeployConfig(ctx, m.eventHandler)
}

func (m *deployConfigManager) eventHandler(e watch.Event) error {
	dat, ok := e.Object.(*types.DeployConfig)
	if !ok {
		glog.Fatalf("event object must be an instance of *types.DeploySpec, got %T", e.Object)
	}

	dkey := types.DeployKey(e.Key)

	m.lock.Lock()
	defer m.lock.Unlock()

	switch e.Type {
	case watch.Added, watch.Modified:
		glog.V(10).Infof("monitor: %v delete, stop controller", dkey)
		m.dc[dkey] = dat

	case watch.Deleted:
		glog.V(10).Infof("monitor: %v delete, stop controller", dkey)
		delete(m.dc, dkey)

	default:
		glog.Warning("monitor: unknown event type: %v", e.Type)
	}

	return nil
}

func (m *deployConfigManager) GetDeployResouce(key types.DeployKey) *types.DeployResource {
	m.lock.RLock()
	defer m.lock.RUnlock()
	r := m.dc[key]

	if r == nil || r.ResourceRequired == nil {
		t, _, err := types.ParseDeployKey(key)
		if err != nil {
			glog.Warningf("monitor: parse deploy key errror: %v", err)
			return nil
		}
		spec := registry.GetDefaultDeployResource(registry.StageType{
			Stage: m.stage,
			Type:  t,
		})
		if spec == nil {
			glog.V(10).Infof("monitor: no default deploy config for %v", t)
			return nil
		}
		return &spec.Medium
	}
	return r.ResourceRequired
}

func (m *deployConfigManager) GetDeployConfig(key types.DeployKey) *types.DeployConfig {
	m.lock.RLock()
	defer m.lock.RUnlock()
	r := m.dc[key]

	return r
}

func (m *deployConfigManager) watchDeployConfig(ctx context.Context, handler watch.EventHandler) error {
	store, err := m.getStore()
	if err != nil {
		return err
	}

	dir := etcdkey.DeployConfigDir(m.stage)
	typ := reflect.TypeOf(types.DeployConfig{})
	watcher, err := store.Watch(ctx, dir, generic.Everything, true, typ)
	if err != nil {
		return err
	}
	go func() {
		defer watcher.Stop()
		for {
			select {
			case event := <-watcher.ResultChan():
				glog.V(10).Infof("receive an deploy config event: %v", event)
				switch event.Type {
				case watch.Error:
					err, ok := event.Object.(error)
					if !ok {
						glog.Warningf("event type if error, but event.Object is not an error")
						err = fmt.Errorf("watch got error :%v", event.Object)
					}
					glog.Warningf("watch err: %v", err)
				default:
					event.Key = strings.TrimPrefix(event.Key, dir)
					if err := handler(event); err != nil {
						glog.Errorf("monitor: watch host deploy config err: %v", err)
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// NewDeployConfigManager create a new deploy config manager
func NewDeployConfigManager(stage types.Stage) (ctypes.DeployConfigManager, error) {
	dcm := deployConfigManager{
		stage: stage,
		dc:    map[types.DeployKey]*types.DeployConfig{},
	}

	if err := dcm.load(); err != nil {
		return nil, err
	}
	return &dcm, nil
}
