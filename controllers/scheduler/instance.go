package scheduler

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	ctypes "we.com/dolphin/controllers/types"
	"we.com/dolphin/registry/etcdkey"
	"we.com/dolphin/registry/generic"
	"we.com/dolphin/registry/watch"
	"we.com/dolphin/types"
	"we.com/dolphin/types/ins/registry"
)

type insManager struct {
	stage     types.Stage
	lock      sync.RWMutex
	instances map[types.DeployKey]map[types.InstanceID]*types.Instance
}

// NewInfor create a new instance Infor
func NewInfor(stage types.Stage) ctypes.InstanceInfor {
	return &insManager{
		stage:     stage,
		instances: map[types.DeployKey]map[types.InstanceID]*types.Instance{},
	}
}

func (m *insManager) GetInstance(key types.DeployKey, insID types.InstanceID) *types.Instance {
	m.lock.RLock()
	defer m.lock.RUnlock()
	s := m.instances[key]
	if len(s) == 0 {
		return nil
	}
	return s[insID]
}
func (m *insManager) Start(ctx context.Context) error {
	go m.watch(ctx)
	return nil
}

func (m *insManager) NewStartedInstance(key types.DeployKey, d time.Duration) []*types.Instance {
	tmp := m.RunningInstance(key)
	now := time.Now()
	var ret []*types.Instance
	for _, v := range tmp {
		if v.StartTime.Add(d).After(now) {
			ret = append(ret, v)
		}
	}
	return ret
}

func (m *insManager) NewStoppedInstance(key types.DeployKey, d time.Duration) []*types.Instance {
	tmp := m.RecetStoppedInstance(key)
	now := time.Now()
	var ret []*types.Instance
	for _, v := range tmp {
		if v.StopTime.Add(d).After(now) {
			ret = append(ret, v)
		}
	}
	return ret
}

func (m *insManager) RunningInstance(key types.DeployKey) map[types.InstanceID]*types.Instance {
	m.lock.RLock()
	defer m.lock.RUnlock()
	s, ok := m.instances[key]
	if !ok {
		return nil
	}
	ret := make(map[types.InstanceID]*types.Instance)
	for k, v := range s {
		if v.LifeCycle != types.LCStopped {
			ret[k] = v
		}
	}
	return ret
}

func (m *insManager) RecetStoppedInstance(key types.DeployKey) map[types.InstanceID]*types.Instance {
	m.lock.RLock()
	defer m.lock.RUnlock()
	s, ok := m.instances[key]
	if !ok {
		return nil
	}
	ret := make(map[types.InstanceID]*types.Instance)
	for k, v := range s {
		ret[k] = v
	}
	return ret
}

func (m *insManager) watch(ctx context.Context) {
	path := etcdkey.DeployInstanceDir(m.stage)
	watchEvent(ctx, path, generic.Everything, reflect.TypeOf(&registry.Instance{}), m.handleInstanceEvent)
}

func (m *insManager) handleInstanceEvent(e watch.Event) error {
	tmp, ok := e.Object.(*registry.Instance)
	if !ok {
		glog.Fatalf("event object must be an instance of *types.Instance, got %T", e.Object)
	}
	dat := (*types.Instance)(tmp)

	m.lock.Lock()
	defer m.lock.Unlock()

	insID := dat.ID
	key := dat.DeployKey()
	switch e.Type {
	case watch.Added, watch.Modified:
		insMap := m.instances[key]
		if insMap == nil {
			insMap = map[types.InstanceID]*types.Instance{}
			m.instances[key] = insMap
		}
		insMap[insID] = dat

	case watch.Deleted:
		if dat.LifeCycle != types.LCStopped {
			glog.Errorf("instance: node %v:%v:%v delete, but it has not stopped", dat.Host, dat.DeployKey(), dat.Pid)
		}
		insMap := m.instances[key]
		if insMap == nil {
			return nil
		}
		delete(insMap, insID)
	}

	return nil
}

func watchEvent(ctx context.Context, path string, pred generic.SelectionPredicate,
	expectType reflect.Type, h watch.EventHandler) error {
	store, err := generic.GetStoreInstance(etcdkey.BaseDir(), false)
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
