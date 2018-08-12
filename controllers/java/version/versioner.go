/*
Sniperkit-Bot
- Status: analyzed
*/

package version

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/golang/glog"
	"we.com/dolphin/registry/generic"
	"we.com/dolphin/registry/watch"
	"we.com/dolphin/types"
	"we.com/jiabiao/common/runtime"
)

const (
	FieldSperator        = ":"
	projectVersionPrefix = "version"
	VersionNone          = ""
)

// versionManger manager versioninfo
type versionManger struct {
	lock sync.RWMutex
	env  string
	// keys is fmt.Sprintf("type:cluster:")
	// cluster not include version
	versionInfos map[string]*VersionInfo
}

func (vm *versionManger) delConfig(typ types.ProjectType, cluster types.UUID) {
	if vm == nil {
		return
	}
	key := fmt.Sprintf("%v%v%v%v", typ, FieldSperator, cluster, FieldSperator)
	vm.lock.RLock()
	defer vm.lock.RUnlock()
	delete(vm.versionInfos, key)
}

func (vm *versionManger) getVersionInfo(typ types.ProjectType, cluster types.UUID) (*VersionInfo, bool) {
	key := fmt.Sprintf("%v%v%v%v", typ, FieldSperator, cluster, FieldSperator)
	vm.lock.RLock()
	defer vm.lock.RUnlock()
	vi, ok := vm.versionInfos[key]
	if !ok {
		return NewVersionInfo(typ, cluster, VersionNone, VersionNone), false
	}
	return vi, true
}

func (vm *versionManger) getBackup(typ types.ProjectType, cluster types.UUID) string {
	vi, _ := vm.getVersionInfo(typ, cluster)
	return vi.GetBackup()
}

func (vm *versionManger) getExpect(typ types.ProjectType, cluster types.UUID) string {
	vi, _ := vm.getVersionInfo(typ, cluster)

	return vi.GetExpected()
}

func (vm *versionManger) addVersion(typ types.ProjectType, cluster types.UUID, version ...string) {
	vi, ok := vm.getVersionInfo(typ, cluster)
	vm.lock.Lock()
	defer vm.lock.Unlock()

	if !ok {
		key := fmt.Sprintf("%v%v%v%v", typ, FieldSperator, cluster, FieldSperator)
		vm.versionInfos[key] = vi
	}

	for i := len(version) - 1; i > 0; i-- {
		vi.AddVersion(version[i])
	}
}

func (vm *versionManger) setExpected(typ types.ProjectType, cluster types.UUID, version string) {
	vi, ok := vm.getVersionInfo(typ, cluster)
	vm.lock.Lock()
	defer vm.lock.Unlock()
	if !ok {
		key := fmt.Sprintf("%v%v%v%v", typ, FieldSperator, cluster, FieldSperator)
		vm.versionInfos[key] = vi
	}
	vi.SetExpected(version)
}

func (vm *versionManger) String() string {
	ret := fmt.Sprintf("%v:\n", vm.env)
	vm.lock.RLock()
	vm.lock.RUnlock()
	for _, v := range vm.versionInfos {
		ret += fmt.Sprintf("\t%v: %v, %v\n", v.Cluster, v.GetExpected(), v.GetBackup())
	}
	return ret
}

var (
	lock            sync.RWMutex
	versionManagers = map[string]*versionManger{}
	cancelFunc      context.CancelFunc
)

func getvm(env string) *versionManger {
	lock.Lock()
	defer lock.Unlock()
	vm, ok := versionManagers[env]
	if !ok {
		vm = &versionManger{
			env: env,
			// keys if fmt.Sprintf("type:cluster:")
			// cluster not include version
			versionInfos: map[string]*VersionInfo{},
		}
		versionManagers[env] = vm
	}
	return vm
}

// AddVersion add a new version to vm
func AddVersion(env string, typ types.ProjectType, cluster types.UUID, version ...string) {
	vm := getvm(env)

	vm.addVersion(typ, cluster, version...)
}

// SetExpected version
func SetExpected(env string, typ types.ProjectType, cluster types.UUID, version string) {
	vm := getvm(env)
	vm.setExpected(typ, cluster, version)
}

// GetExpected verion
func GetExpected(env string, typ types.ProjectType, cluster types.UUID) string {
	vm := getvm(env)
	return vm.getExpect(typ, cluster)
}

// GetBackup verion
func GetBackup(env string, typ types.ProjectType, cluster types.UUID) string {
	vm := getvm(env)
	return vm.getBackup(typ, cluster)
}

func watchVersionInfo(ctx context.Context) error {
	defer runtime.HandleCrash()

	//  here env is nil, we watch version info changes of all env
	return watchVersionInfo(ctx)
}

// Start starts to watch etcd from version info change
func Start() {
	lock.Lock()
	defer lock.Unlock()
	if cancelFunc != nil {
		return
	}
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	cancelFunc = cancel

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if err := watchVersionInfo(ctx); err != nil {
					glog.Errorf("watch version info: %v", err)
				}
			}
		}
	}()
}

// Stop stop watch
func Stop() {
	lock.Lock()
	defer lock.Unlock()
	if cancelFunc != nil {
		cancelFunc()
	}

	cancelFunc = nil
}

func eventHandler(event watch.Event) error {
	dat, ok := event.Object.(*VersionInfo)
	if !ok {
		glog.Fatalf("event object must be an instance of VersionInfo, got %T", event.Object)
	}

	parts := strings.SplitN(event.Key, "/", 2)

	if len(parts) != 2 {
		glog.Warningf("version info changes: %v, %v, %v", event.Key, event.Type, dat)
		return nil
	}

	env := string(parts[0])
	glog.V(10).Infof("version info is %v: %v, %v", event.Type, env, dat)

	switch event.Type {
	case watch.Deleted:
		lock.Lock()
		vm, _ := versionManagers[env]
		lock.Unlock()

		vm.delConfig(dat.Type, dat.Cluster)
	case watch.Added, watch.Modified:
		back := dat.GetBackup()
		expect := dat.GetExpected()

		AddVersion(env, dat.Type, dat.Cluster, back, expect)
	default:
		glog.Errorf("unkonw eventy type: %v", event.Type)
	}
	return nil
}

// WatchVersion watch version info changes
func WatchVersion(ctx context.Context, handler watch.EventHandler) error {
	if handler == nil {
		return errors.New("handler is nil")
	}

	if ctx == nil {
		return errors.New("context cannot be nil")
	}

	store, err := generic.GetStoreInstance("", false)
	if err != nil {
		glog.Errorf("getStoreInstance error: %s", err)
		return err
	}

	key := projectVersionPrefix

	watcher, err := store.Watch(ctx, key, generic.Everything, true, reflect.TypeOf(VersionInfo{}))
	if err != nil {
		return err
	}

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
				glog.Warningf("watch version info: %v", err)
			default:
				if err := handler(event); err != nil {
					glog.Fatalf("handle version info: %v", err)
				}
			}
		case <-ctx.Done():
			return nil
		}
	}
}
