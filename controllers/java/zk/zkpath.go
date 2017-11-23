package zk

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"we.com/dolphin/controllers/java/project"
	"we.com/dolphin/registry/etcdkey"
	"we.com/dolphin/registry/generic"
	"we.com/dolphin/registry/watch"
	"we.com/dolphin/types"
)

type serviceZKPathInfor struct {
	lock    sync.RWMutex
	stopC   chan struct{}
	infos   map[types.DeployName]*project.Info
	pathMap map[string]types.DeployName
}

// path should be an abslute zk path
func cleanPath(path string) (string, error) {
	parts := strings.Split(path, "/")

	if parts[1] == "biz" {
		if len(parts) >= 4 {
			return strings.Join(parts[2:3], "."), nil
		}
		return "", errors.Errorf("%v parse error", path)
	}

	return parts[2], nil
}

func (s *serviceZKPathInfor) GetRoutePath(name types.DeployName) string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	i := s.infos[name]
	if i == nil {
		return ""
	}
	return i.ZKRoute
}

func (s *serviceZKPathInfor) GetInstancePath(name types.DeployName) string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	i := s.infos[name]
	if i == nil {
		return ""
	}
	return i.ZKInstance
}

// GetDeployName given a zk path, return its deployname
// TODO: 可以参考url请求的路由实现， 我们只需要依赖于项目配置，不再预知zk上服务的目录结构
func (s *serviceZKPathInfor) GetDeployName(path string) types.DeployName {
	var name types.DeployName
	path, err := cleanPath(path)
	if err != nil {
		return name
	}
	s.lock.RLock()
	defer s.lock.RUnlock()

	name = s.pathMap[path]
	return name
}

func (s *serviceZKPathInfor) GetAPIVersion(name types.DeployName) string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	i := s.infos[name]
	if i == nil {
		return ""
	}
	return i.APIVersion
}

func (s *serviceZKPathInfor) load() error {
	dir := etcdkey.JavaProjectDir()
	store, err := generic.GetStoreInstance(dir, false)
	if err != nil {
		return err
	}

	typ := reflect.TypeOf(project.Info{})
	w, err := store.Watch(context.Background(), "", generic.Everything, true, typ)
	if err != nil {
		return err
	}

	h := func(e watch.Event) error {
		dat, ok := e.Object.(*project.Info)
		if !ok {
			glog.Fatalf("event object must be an instance of *project.Info, got %T", e.Object)
		}

		switch e.Type {
		case watch.Added, watch.Modified:
			s.lock.Lock()
			s.infos[dat.Name] = dat
			p, err := cleanPath(dat.ZKInstance)
			if err != nil {
				s.pathMap[p] = dat.Name
			} else {
				glog.Errorf("zk sync: %v parse error: %v", dat.ZKInstance, err)
			}
			s.lock.Unlock()

		case watch.Deleted:
			s.lock.Lock()
			delete(s.infos, dat.Name)
			p, err := cleanPath(dat.ZKInstance)
			if err != nil {
				delete(s.pathMap, p)
			} else {
				glog.Errorf("zk sync: %v parse error: %v", dat.ZKInstance, err)
			}
			s.lock.Unlock()
		}

		return nil
	}

	go func() {
		for {
			select {
			case <-s.stopC:
				w.Stop()
				return
			case event := <-w.ResultChan():
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
						glog.Errorf("java: zk path info %v", err)
					}
				}
			}
		}
	}()

	return nil
}

// PathInfor get java zk path info
type PathInfor interface {
	GetRoutePath(name types.DeployName) string
	GetInstancePath(name types.DeployName) string
	GetDeployName(path string) types.DeployName
	GetAPIVersion(name types.DeployName) string
}

// NewZKPathInfor return a new PathInfor
func NewZKPathInfor() (PathInfor, error) {
	s := &serviceZKPathInfor{
		infos:   map[types.DeployName]*project.Info{},
		pathMap: map[string]types.DeployName{},
	}
	err := s.load()
	if err != nil {
		return nil, err
	}
	return s, nil
}
