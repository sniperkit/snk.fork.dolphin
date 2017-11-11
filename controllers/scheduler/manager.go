package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"we.com/dolphin/types"
)

type runner struct {
	ctx context.Context
	c   chan struct{}
}

func newRunner(ctx context.Context, worker int) *runner {
	if worker <= 0 {
		return nil
	}

	return &runner{c: make(chan struct{}, worker), ctx: ctx}
}

func (r *runner) run(f func() error) <-chan error {
	r.c <- struct{}{}
	c := make(chan error, 1)
	go func() {
		c <- f()
		<-r.c
	}()
	return c
}

// Manager managers controllers
type Manager interface {
	Deploy(ctx context.Context, dc *types.DeployConfig) error
	Update(ctx context.Context, dc *types.DeployConfig) error
	Stop(ctx context.Context, key types.DeployKey, insID types.InstanceID) error
	StopLegacy(ctx context.Context, key types.DeployKey) error
	Destroy(ctx context.Context, key types.DeployKey) error
	Log(ctx context.Context, key types.DeployKey, n int) chan<- logEntity
}

type manager struct {
	stage       types.Stage
	wg          sync.WaitGroup
	lock        sync.RWMutex
	controllers map[types.DeployKey]*controller
	runner      *runner
	stopC       chan struct{}
}

func (m *manager) Stop() {
	close(m.stopC)
	m.wg.Wait()
}

func (m *manager) Deploy(ctx context.Context, dc *types.DeployConfig) error {
	if dc == nil {
		return nil
	}

	if err := dc.Validate(); err != nil {
		return err
	}

	key := dc.Key()

	m.lock.Lock()
	_, ok := m.controllers[key]
	if !ok {
		// 先占个坑
		m.controllers[key] = nil
	}
	m.lock.Unlock()
	if ok {
		return errors.Errorf("scheduler: %v, doployment %v already exists", m.stage.String(), key)
	}

	c, err := newController(dc, option{
		maxTries:            3,
		legacyVerionTimeout: 2 * time.Hour,
		dryMode:             false,
	})

	m.lock.Lock()
	if err != nil {
		delete(m.controllers, key)
	} else {
		m.controllers[key] = c
	}
	m.lock.Unlock()

	if err != nil {
		return err
	}

	return m.Update(ctx, dc)
}

func (m *manager) Update(ctx context.Context, dc *types.DeployConfig) error {
	if dc == nil {
		return nil
	}

	if err := dc.Validate(); err != nil {
		return err
	}

	if dc.Stage != m.stage {
		return errors.Errorf("scheduler: wrong stage %v for manager %v", dc.Stage.String(), m.stage.String())
	}

	key := dc.Key()
	m.lock.RLock()
	c, ok := m.controllers[key]
	m.lock.RUnlock()
	if !ok {
		return errors.Errorf("sched: unknown %v for env %v", key, m.stage.String())
	}

	return c.Deploy(ctx, dc)
}

func (m *manager) StopInstance(ctx context.Context, key types.DeployKey, insID types.InstanceID) error {
	return nil
}

func (m *manager) getController(key types.DeployKey) (*controller, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	c, ok := m.controllers[key]
	return c, ok
}

func (m *manager) updateController(key types.DeployKey, c *controller) (old *controller) {
	m.lock.Lock()
	defer m.lock.Unlock()
	old = m.controllers[key]
	m.controllers[key] = c
	return
}

func (m *manager) deleteController(key types.DeployKey) (old *controller) {
	m.lock.Lock()
	defer m.lock.Unlock()
	old = m.controllers[key]
	delete(m.controllers, key)
	return
}

func (m *manager) move(key types.DeployKey, from types.HostID, to types.HostID) error {
	return nil
}
