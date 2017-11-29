package scheduler

import (
	"context"
	"strings"
	"sync"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"we.com/dolphin/controllers/alert"
	ctypes "we.com/dolphin/controllers/types"
	"we.com/dolphin/types"
)

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
}

// NewSchedular  create a new schedual manager
func NewSchedular(stage types.Stage, lease time.Duration, info ctypes.InstanceInfor, hcManager ctypes.HostConfigManager) (Manager, error) {
	if info == nil || hcManager == nil {
		return nil, errors.New("info and hcmanager cannot be nil")
	}

	m := manager{
		stage:     stage,
		lease:     lease,
		info:      info,
		hcManager: hcManager,
	}

	return &m, nil
}

func (m *manager) Stop() {
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

func (m *manager) RevokeLegacyLease(key types.DeployKey) error {
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

func (m *manager) CheckStatus() error {
	keys := m.info.ListDeploykeys()
	keyMap := make(map[types.DeployKey]struct{}, len(keys))

	for _, v := range keys {
		keyMap[v] = struct{}{}
	}

	var unManagedKeys []string
	m.lock.RLock()
	for k := range keyMap {
		if _, ok := m.controllers[k]; !ok {
			unManagedKeys = append(unManagedKeys, string(k))
			delete(keyMap, k)
		}
	}
	m.lock.RUnlock()

	var alerts []alert.Message
	var merr *multierror.Error
	for k := range keyMap {
		c, _ := m.getController(k)
		if c != nil {
			if err := c.checkStatus(); err != nil {
				pt, dn, _ := types.ParseDeployKey(k)
				merr = multierror.Append(merr, err)
				alerts = append(alerts, alert.Message{
					Labels: map[string]string{
						"env":       m.stage.String(),
						"from":      "dolphin scheduler",
						"deployKey": string(k),
						"ptype":     string(pt),
						"proj":      dn,
					},
					Annotations: map[string]string{
						"msg": err.Error(),
					},
				})
			}
		}
	}

	if len(unManagedKeys) > 0 {
		err := errors.Errorf("sched: there %v unmanaged deploy keys, have instances running: %v", len(unManagedKeys), strings.Join(unManagedKeys, ", "))

		merr = multierror.Append(merr, err)
		alerts = append(alerts, alert.Message{
			Labels: map[string]string{
				"env":  m.stage.String(),
				"from": "dolphin scheduler",
				"key":  "monitor",
			},
			Annotations: map[string]string{
				"msg": err.Error(),
			},
		})
	}

	if len(alerts) > 0 {
		go alert.SendAlerts(alerts...)
	}
	return merr.ErrorOrNil()
}
