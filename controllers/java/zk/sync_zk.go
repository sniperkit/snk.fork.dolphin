package zk

import (
	"context"
	"regexp"
	"sync"
	"time"

	"github.com/coreos/fleet/log"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/samuel/go-zookeeper/zk"

	zkcfg "we.com/dolphin/controllers/java/zk/types"
	"we.com/dolphin/registry/generic"
)

type manager struct {
	lock    sync.RWMutex
	config  *zkcfg.Config
	clients map[string]*Client
	cancels map[string]context.CancelFunc
	wg      sync.WaitGroup
}

func (m *manager) StopAll() {
	m.lock.Lock()
	defer m.lock.Unlock()
	for e, cf := range m.cancels {
		log.Infof("stop zk sync of %v", e)
		cf()
		delete(m.cancels, e)
	}

	m.wg.Wait()

	for _, c := range m.clients {
		c.Close()
	}
	m.cancels = nil
	m.clients = nil
}

func (m *manager) stop(env string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if cf, ok := m.cancels[env]; ok {
		cf()
		delete(m.cancels, env)
	}

	if c, ok := m.clients[env]; ok {
		c.Close()
		delete(m.clients, env)
	}

	return nil
}

func newManager(cfg *zkcfg.Config) (*manager, error) {
	if cfg == nil {
		return nil, errors.Errorf("config cannot be nil")
	}

	m := &manager{
		config:  cfg,
		clients: map[string]*Client{},
		cancels: map[string]context.CancelFunc{},
	}

	return m, nil
}

func (m *manager) startAll() error {
	if m == nil {
		return errors.Errorf("manaager is nil")
	}

	if m.config == nil {
		return errors.Errorf("config is nil")
	}

	for k := range m.config.Envs {
		if err := m.ReloadData(k); err != nil {
			return err
		}

		m.lock.RLock()
		if _, ok := m.cancels[k]; ok {
			m.lock.RUnlock()
			return errors.Errorf("zk: watch for %v already starting, please stop first", k)
		}
		ctx, cf := context.WithCancel(context.Background())
		m.cancels[k] = cf
		m.lock.RUnlock()

		go func(env string) {
			// if more than 5 times err happend with 5 mins
			// log.Fatal
			startTime := time.Now()
			errCount := 0
			for {
				select {
				case <-ctx.Done():
					return
				default:
					if err := m.WatchEnv(ctx, env); err != nil {
						glog.Error(err)
						errCount++
						now := time.Now()
						if startTime.Add(5 * time.Minute).After(now) {
							if errCount > 5 {
								glog.Fatalf("zk: %v sync failed %v times within %v seconds", env, errCount, (now.Unix() - startTime.Unix()))
							}
						} else {
							startTime = now
							errCount = 1
						}
					}

					if err := m.stop(env); err != nil {
						glog.Errorf("zk: stop env %v: %v", env, err)
					}
				}
			}
		}(k)
	}

	return nil
}

func (m *manager) ReloadData(env string) error {
	cli, err := m.getzkClient(env)
	if err != nil {
		return errors.Errorf("zk: %v get zkclient, %v", env, err)
	}

	m.lock.RLock()
	cfg, ok := m.config.Envs[env]
	if !ok {
		m.lock.RUnlock()
		return errors.Errorf("zk: unknown env: %v", env)
	}
	m.lock.RUnlock()

	store, err := generic.GetStoreInstance("", false)
	if err != nil {
		return err
	}

	for _, v := range cfg.ZKPaths {
		dat, err := cli.GetValues([]string{v.Base}, v.Regexp, false)
		if err != nil {
			return err
		}

		for k, d := range dat {
			etcdPath, err := getEtcdPath(env, k)
			if err != nil {
				glog.Warningf("zk: %v zkpath %v ingored for %v", env, k, err)
				continue
			}

			err = store.Update(context.Background(), etcdPath, d, nil, 0)
			if err != nil {
				glog.Errorf("zk: sync data from zk to etcd: %v", err)
			}
		}
	}

	return nil
}

// WatchEnv  config configed zkpath of env for changes
// note: this will not load data from zk, if no events happened
func (m *manager) WatchEnv(ctx context.Context, env string) error {
	handler := m.handlerFunc(ctx, env)

	ech := make(chan zk.Event, 2)
	defer close(ech)

	if err := m.watchEnv(ctx, env, ech); err != nil {
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

func (m *manager) getzkClient(env string) (*Client, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	cli, ok := m.clients[env]
	if ok {
		return cli, nil
	}

	if m.config == nil {
		return nil, errors.Errorf("zk: manager has no config")
	}

	cfg, ok := m.config.Envs[env]
	if !ok {
		return nil, errors.Errorf("unknown env: %v", env)
	}

	cli, err := NewClient(cfg.ZKServers)
	if err != nil {
		return nil, err
	}

	m.clients[env] = cli
	return cli, err
}

func (m *manager) watchEnv(ctx context.Context, env string, ech chan<- zk.Event) error {

	cli, err := m.getzkClient(env)
	if err != nil {
		return err
	}

	m.lock.RLock()
	cfg, ok := m.config.Envs[env]
	m.lock.RUnlock()
	if !ok {
		return errors.Errorf("zk: manager there not config for %v", env)
	}

	var re *regexp.Regexp
	for _, v := range cfg.ZKPaths {
		if v.Regexp != nil {
			re = v.Regexp
		}

		cli.WatchPrefix(ctx, v.Base, re, ech)
	}

	return nil
}

func (m *manager) handlerFunc(ctx context.Context, env string) func(zk.Event) error {
	return func(event zk.Event) error {
		cli, ok := m.clients[env]
		if !ok {
			return errors.Errorf("zk: %v handlerFunc: cannot get zk client", env)
		}

		glog.V(10).Infof("zk: %v receiver event: %v", env, event)
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

			// new added nodes
			for _, k := range l {
				if _, ok := etcdKeysMap[k]; !ok {
					s := path + "/" + k
					data, _, err := cli.client.Get(s)
					if err != nil {
						glog.Errorf("zk: %v get zk data of %v: %v", env, s, err)
						continue
					}

					etcdPath, err := getEtcdPath(env, s)
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

			etcdPath, err := getEtcdPath(env, path)
			if err != nil {
				glog.Infof("zk: %v zkpath to etcdpath: %v, skip", env, path)
				break
			}

			store, err := generic.GetStoreInstance("", false)
			if err != nil {
				return err
			}

			if err = store.Update(context.Background(), etcdPath, data, nil, 0); err != nil {
				glog.Errorf("zk: %v store data to etcd %v: %v", env, etcdPath, err)
			}
		case zk.EventNodeDeleted:
			etcdPath, err := getEtcdPath(env, path)
			if err != nil {
				glog.Infof("zk: %v zkpath to etcdpath: %v, skip", env, path)
				break
			}

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
