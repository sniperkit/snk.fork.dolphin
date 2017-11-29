package zk

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/samuel/go-zookeeper/zk"

	"we.com/dolphin/controllers/java/router"
	zkcfg "we.com/dolphin/controllers/java/zk/types"
	"we.com/dolphin/registry/generic"
	"we.com/dolphin/types"
)

type manager struct {
	stage       types.Stage
	config      *zkcfg.EnvConfig
	zkClient    *Client
	zkPathInfor PathInfor
	cf          context.CancelFunc
	lock        sync.RWMutex
	zkIns       map[types.DeployName][]router.ServiceNode
	routeCfg    map[types.DeployName]*router.RouteCfg
}

func (m *manager) Destory() error {
	if m.cf != nil {
		m.cf()
	}

	if m.zkClient != nil {
		m.zkClient.Close()
	}
	m.cf = nil
	m.zkClient = nil

	return nil
}

func newManager(cfg *zkcfg.EnvConfig, pi PathInfor) (Manager, error) {
	if cfg == nil {
		return nil, errors.Errorf("config cannot be nil")
	}

	m := &manager{
		config:      cfg,
		stage:       cfg.ENV,
		zkIns:       map[types.DeployName][]router.ServiceNode{},
		routeCfg:    map[types.DeployName]*router.RouteCfg{},
		zkPathInfor: pi,
	}

	cli, err := NewClient(cfg.ZKServers)
	if err != nil {
		return nil, err
	}

	m.zkClient = cli

	if err := m.start(); err != nil {
		m.Destory()
		return nil, err
	}

	return m, nil
}

func (m *manager) start() error {
	if m == nil {
		return errors.Errorf("manaager is nil")
	}

	if m.config == nil {
		return errors.Errorf("config is nil")
	}
	
	go func() {
		timer := time.NewTicker(4 * time.Hour)
		defer timer.Stop()
		for {
			select {
			case <-  timer.C:
				if err := m.ReloadData(); err != nil {
					glog.Errorf("zk: sync data from zk to etcd: %v", err)
				}
			}
		}
	}


	ctx, cf := context.WithCancel(context.Background())
	m.cf = cf

	

	go func() {
		// if more than 5 times err happend with 5 mins
		// log.Fatal
		startTime := time.Now()
		errCount := 0
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if err := m.Watch(ctx); err != nil {
					glog.Error(err)
					errCount++
					now := time.Now()
					if startTime.Add(5 * time.Minute).After(now) {
						if errCount > 5 {
							glog.Fatalf("zk: %v sync failed %v times within %v seconds", m.stage.String(), errCount, (now.Unix() - startTime.Unix()))
						}
					} else {
						startTime = now
						errCount = 1
					}
				}

				m.Destory()
			}
		}
	}()

	return nil
}

func (m *manager) ReloadData() error {
	env := m.stage
	cfg := m.config
	store, err := generic.GetStoreInstance("", false)
	if err != nil {
		return err
	}

	var merr *multierror.Error
	for _, v := range cfg.ZKPaths {
		dat, err := m.zkClient.GetValues([]string{v.Base}, v.Regexp, false)
		if err != nil {
			glog.Errorf("zk: get zk values: %v", err)
			return err
		}
		for k, d := range dat {
			if strings.Contains(k, "esb") {
				glog.Infof("zkpath: %v", k)
			}
			typ, etcdPath, err := m.zkPathInfor.GetEtcdPath(env, k)
			if typ == zkRoute && strings.HasPrefix(k, "/serv") {
				glog.Infof("zk: route: %v, %v, %v", k, etcdPath, err)
			}
			if err != nil {
				glog.Warningf("zk: %v zkpath %v ingored for %v", env, k, err)
				continue
			}

			if err := m.parseZKData(typ, k, d); err != nil {
				merr = multierror.Append(merr, err)
			}

			err = store.Update(context.Background(), etcdPath, d, nil, 0)
			if err != nil {
				glog.Errorf("zk: sync data from zk to etcd: %v", err)
			}
		}
	}

	return nil
}

func (m *manager) GetRouteConfig(name types.DeployName) (*router.RouteCfg, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	rc := m.routeCfg[name]
	return rc, nil
}

func (m *manager) SetRouteConfig(name types.DeployName, cfg *router.RouteCfg) error {
	path, err := m.zkPathInfor.GetRoutePath(name)
	if err != nil {
		return err
	}
	if path == "" {
		return errors.Errorf("dont known zk path form %v", name)
	}

	var val string
	if cfg != nil {
		val = cfg.String()
	}

	if err := m.zkClient.SetNodeValue(path, val); err != nil {
		return err
	}

	m.lock.Lock()
	defer m.lock.Unlock()
	m.routeCfg[name] = cfg

	return nil
}

func (m *manager) GetInstanceList(name types.DeployName) ([]*router.ServiceNode, error) {
	ret := []*router.ServiceNode{}
	m.lock.RLock()
	defer m.lock.RUnlock()

	ins, ok := m.zkIns[name]
	if !ok {
		return nil, nil
	}
	for _, v := range ins {
		ret = append(ret, &v)
	}

	return ret, nil
}

// Watch  config configed zkpath of env for changes
// note: this will not load data from zk, if no events happened
func (m *manager) Watch(ctx context.Context) error {
	handler := m.handlerFunc(ctx, m.stage)

	ech := make(chan zk.Event, 10)
	defer func() {
		close(ech)
		for range ech {
		}
	}()

	if err := m.watch(ctx, ech); err != nil {
		return err
	}

	for {
		select {
		case event := <-ech:
			if err := handler(event); err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (m *manager) watch(ctx context.Context, ech chan<- zk.Event) error {
	var re *regexp.Regexp
	for _, v := range m.config.ZKPaths {
		if v.Regexp != nil {
			re = v.Regexp
		}

		m.zkClient.WatchPrefix(ctx, v.Base, re, ech)
	}

	return nil
}

func (m *manager) parseZKData(typ zkTyp, path string, dat []byte) error {
	name, err := m.zkPathInfor.GetDeployName(path)
	if err != nil {
		return err
	}
	if name == "" {
		return errors.Errorf("zk sync: cannot recognize zk path: %v", path)
	}

	nodeName := filepath.Base(path)

	s := router.ServiceNode{}
	var rc *router.RouteCfg
	if typ == zkInstance {
		if err := json.Unmarshal(dat, &s); err != nil {
			return err
		}
		s.NodeName = nodeName
	} else if typ == zkRoute {
		ver := m.zkPathInfor.GetAPIVersion(name)
		rc, err = router.Parse(string(dat), ver)
		if err != nil {
			glog.Errorf("zk: parse %v router config, err: %v", name, err)
			return err
		}
	}

	m.lock.Lock()
	defer m.lock.Unlock()
	if typ == zkInstance {
		ss := m.zkIns[name]
		ss = append(ss, s)
		m.zkIns[name] = ss
	} else if typ == zkRoute {
		m.routeCfg[name] = rc
	}

	return nil
}

func (m *manager) handlerFunc(ctx context.Context, env types.Stage) func(zk.Event) error {
	return func(event zk.Event) error {
		cli := m.zkClient
		//glog.V(10).Infof("zk: %v receiver event: %v", env, event)
		fmt.Printf("zk: %v receiver event: %v\n", env, event)
		if event.Err != nil {
			glog.Errorf("zk: %v: %v", env, event.Err)
		}
		path := event.Path
		switch event.Type {
		case zk.EventNodeChildrenChanged:
			// get children
			l, _, err := cli.client.Children(path)
			if err != nil {
				return err
			}

			store, err := generic.GetStoreInstance("", false)
			if err != nil {
				return err
			}
			keys, err := store.ListKeys(context.Background(), path)
			if err != nil {
				return err
			}

			zkKeysMap := map[string]struct{}{}
			for _, k := range l {
				zkKeysMap[k] = struct{}{}
			}

			etcdKeysMap := map[string]struct{}{}
			for _, k := range keys {
				etcdKeysMap[k] = struct{}{}
			}

			// deleted nodes
			for _, k := range keys {
				if _, ok := zkKeysMap[k]; !ok {
					if err = store.Delete(context.Background(), path+"/"+k, nil); err != nil {
						glog.Errorf("zk: %v delete etcdpath %v: %v", env, path+"/"+k, err)
					}
				}
			}

			name, err := m.zkPathInfor.GetDeployName(path)
			if err != nil {
				glog.Errorf("zk: %v delete etcdpath %v: %v", env, path, err)
				return err
			}

			new := make([]router.ServiceNode, 0, len(l))
			m.lock.Lock()
			ss := m.zkIns[name]
			m.zkIns[name] = new
			m.lock.Unlock()

			// new added nodes
			for _, k := range l {
				sn := filepath.Base(k)
				found := false
				for _, v := range ss {
					if v.NodeName == sn {
						found = true
						new = append(new, v)
					}
				}

				if _, ok := etcdKeysMap[k]; !ok {
					s := path + "/" + k
					data, _, err := cli.client.Get(s)
					if err != nil {
						glog.Errorf("zk: %v get zk data of %v: %v", env, s, err)
						continue
					}

					if !found {
						m.parseZKData(zkInstance, s, data)
					}

					_, etcdPath, err := m.zkPathInfor.GetEtcdPath(env, s)
					if err != nil {
						glog.Infof("zk: %v zkpath to etcdpath: %v, skip", env, s)
						continue
					}
					if err = store.Update(context.Background(), etcdPath, data, nil, 0); err != nil {
						glog.Errorf("zk: %v store data to etcd %v: %v", env, etcdPath, err)
					}
				}
			}

		case zk.EventNodeDataChanged:
			data, _, err := cli.client.Get(path)
			if err != nil {
				glog.Errorf("zk: %v get zk data of %v: %v", env, path, err)
				break
			}

			typ, etcdPath, err := m.zkPathInfor.GetEtcdPath(env, path)
			if err != nil {
				glog.Infof("zk: %v zkpath to etcdpath: %v, skip", env, path)
				break
			}

			if typ == zkRoute {
				m.parseZKData(typ, path, data)
			}

			store, err := generic.GetStoreInstance("", false)
			if err != nil {
				return err
			}

			if err = store.Update(context.Background(), etcdPath, data, nil, 0); err != nil {
				glog.Errorf("zk: %v store data to etcd %v: %v", env, etcdPath, err)
			}
		case zk.EventNodeDeleted:
			typ, etcdPath, err := m.zkPathInfor.GetEtcdPath(env, path)
			if err != nil {
				glog.Infof("zk: %v zkpath to etcdpath: %v, skip", env, path)
				break
			}

			name, err := m.zkPathInfor.GetDeployName(path)
			if err != nil {
				glog.Infof("zk: %v zkpath to etcdpath: %v, skip", env, path)
				break
			}

			m.lock.Lock()
			if typ == zkRoute {
				delete(m.routeCfg, name)
			} else if typ == zkInstance {
				nodeName := filepath.Base(path)
				ss := m.zkIns[name]
				for idx, v := range ss {
					if v.NodeName == nodeName {
						ss[idx] = ss[len(ss)-1]
						m.zkIns[name] = ss[:len(ss)-1]
					}
				}
			}
			m.lock.Unlock()

			store, err := generic.GetStoreInstance("", false)
			if err != nil {
				return err
			}
			if err = store.Delete(context.Background(), etcdPath, nil); err != nil {
				glog.Errorf("zk: %v delete etcd data %v: %v", env, etcdPath, err)
			}

		default:
			// log
			glog.Infof("zk: %v %v ignored", env, event)
		}

		return nil
	}
}

func (m *manager) ListDeployment() []types.DeployName {
	ret := make([]types.DeployName, 0, len(m.zkIns))

	m.lock.RLock()
	defer m.lock.RUnlock()
	for k := range m.zkIns {
		ret = append(ret, k)
	}
	return ret
}

// Manager get or set java instance infos
type Manager interface {
	ListDeployment() []types.DeployName
	GetRouteConfig(name types.DeployName) (*router.RouteCfg, error)
	SetRouteConfig(name types.DeployName, rc *router.RouteCfg) error
	GetInstanceList(name types.DeployName) ([]*router.ServiceNode, error)
	Destory() error
}
