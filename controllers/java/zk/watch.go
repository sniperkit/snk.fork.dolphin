package zk

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	zkcfg "we.com/dolphin/controllers/java/zk/types"
)

var (
	m    *manager
	cf   = map[string]context.CancelFunc{}
	lock sync.Mutex
)

func Start(cfg *zkcfg.Config) error {
	lock.Lock()
	defer lock.Unlock()

	if m != nil {
		return errors.Errorf("zk: there is already a manager running")
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	tm, err := newManager(cfg)
	if err != nil {
		return err
	}

	if err := tm.startAll(); err != nil {
		return err
	}

	m = tm
	return nil
}

func Stop() {
	lock.Lock()
	defer lock.Unlock()
	if m == nil {
		return
	}

	m.StopAll()

	m = nil
}

func StartENV(env string) error {
	lock.Lock()
	defer lock.Unlock()

	if m == nil {
		return errors.Errorf("manager is nil")
	}

	m.lock.RLock()
	if _, ok := m.cancels[env]; ok {
		m.lock.RUnlock()
		return errors.Errorf("zk: watch for %v already starting, please stop first", env)
	}
	m.lock.RUnlock()

	if err := m.ReloadData(env); err != nil {
		return err
	}

	ctx, cf := context.WithCancel(context.Background())

	m.lock.Lock()
	m.cancels[env] = cf
	m.lock.Unlock()

	if err := m.WatchEnv(ctx, env); err != nil {
		return err
	}

	return nil
}

func StopENV(env string) error {
	lock.Lock()
	defer lock.Unlock()

	if m == nil {
		return errors.Errorf("manager is nil")
	}

	return m.stop(env)
}
