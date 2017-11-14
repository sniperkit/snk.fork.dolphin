package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	ctypes "we.com/dolphin/controllers/types"
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
	// Deploy deploy a new key
	Deploy(ctx context.Context, dc *types.DeployConfig) error
	// Update update a deployconfig and trigger a new deployment
	Update(ctx context.Context, dc *types.DeployConfig) error
	// RevokeLegacyLease  trigger an instance stop of legacy instaces
	RevokeLegacyLease(key types.DeployKey) error
	// RenewLegacyLease  reset lease timeout
	RenewLegacyLease(key types.DeployKey) error
	// Destroy delete replicaCtrl and  stop all running instaces
	Destroy(ctx context.Context, key types.DeployKey) error
}

type manager struct {
	stage       types.Stage
	lease       time.Duration
	lock        sync.RWMutex
	info        ctypes.InstanceInfor
	hcManager   ctypes.HostConfigManager
	controllers map[types.DeployKey]*replicaCtrl
	runner      *runner
	stopC       chan struct{}
}

func (m *manager) Stop() {
	close(m.stopC)
}

func (m *manager) Deploy(ctx context.Context, dc *types.DeployConfig) error {
	if dc == nil {
		return nil
	}

	if err := dc.Validate(); err != nil {
		return err
	}

	key := dc.Key()

	_, err := m.controlerReady(key)
	if err != nil {
		return err
	}

	c, err := newReplicaCtrl(dc, m.info, m.hcManager, option{
		maxTries:            3,
		legacyVerionTimeout: m.lease,
		dryMode:             false,
	})

	if err != nil {
		m.deleteController(key)
		return err
	}
	c.hcManager = m.hcManager

	err = m.Update(ctx, dc)
	if err != nil {
		m.updateController(key, c)
		return err
	}

	m.updateController(key, c)
	return nil
}

func (m *manager) Update(ctx context.Context, dc *types.DeployConfig) error {
	if dc == nil {
		return nil
	}

	if err := dc.Validate(); err != nil {
		return err
	}

	if dc.Stage != m.stage {
		return errors.Errorf("sched: wrong stage %v for manager %v", dc.Stage.String(), m.stage.String())
	}

	key := dc.Key()
	c, ok := m.getController(key)
	if !ok {
		return errors.Errorf("sched: unknown %v for env %v", key, m.stage.String())
	}

	return c.Deploy(ctx, dc)
}

func (m *manager) RevokeLegacy(key types.DeployKey) error {
	c, err := m.controlerReady(key)
	if err != nil {
		return err
	}
	c.revokeLease()
	return nil
}

func (m *manager) RenewLegacyLease(key types.DeployKey) error {
	c, err := m.controlerReady(key)
	if err != nil {
		return err
	}
	c.renewLease()
	return nil
}

func (m *manager) Destroy(ctx context.Context, key types.DeployKey) error {
	c, err := m.controlerReady(key)
	if err != nil {
		return err
	}

	return c.Destroy()
}

func (m *manager) controlerReady(key types.DeployKey) (*replicaCtrl, error) {
	c, ok := m.getController(key)
	if c != nil {
		return c, nil
	}

	if !ok {
		return nil, errors.Errorf("sched: env=%v, unknown deployment %v", m.stage, key)
	}

	return nil, errors.Errorf("sched: env=%v, %v deployment in process,  please try again later", m.stage, key)
}

func (m *manager) getController(key types.DeployKey) (*replicaCtrl, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	c, ok := m.controllers[key]
	return c, ok
}

func (m *manager) updateController(key types.DeployKey, c *replicaCtrl) (old *replicaCtrl) {
	m.lock.Lock()
	defer m.lock.Unlock()
	old = m.controllers[key]
	m.controllers[key] = c
	return
}

func (m *manager) deleteController(key types.DeployKey) (old *replicaCtrl) {
	m.lock.Lock()
	defer m.lock.Unlock()
	old = m.controllers[key]
	delete(m.controllers, key)
	return
}
