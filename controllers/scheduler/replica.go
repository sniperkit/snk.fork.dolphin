package scheduler

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/golang/glog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	ctypes "we.com/dolphin/controllers/types"
	"we.com/dolphin/types"
)

type replicaCtrl struct {
	opt           option
	stage         types.Stage
	key           types.DeployKey
	dc            types.DeployConfig
	legacyTimer   *time.Timer
	expectVersion types.DeployVer

	info      ctypes.InstanceInfor
	hcManager ctypes.HostConfigManager
}

func newReplicaCtrl(dc *types.DeployConfig, info ctypes.InstanceInfor,
	hcManager ctypes.HostConfigManager, opt option) (*replicaCtrl, error) {
	if dc == nil || info == nil || hcManager == nil {
		return nil, errors.New("args cannot be nil")
	}

	if err := dc.Validate(); err != nil {
		return nil, err
	}

	var version string
	if dc.Image != nil && dc.Image.Version != nil {
		version = dc.Image.Version.String()
	}
	key := dc.Key()

	ctrl := replicaCtrl{
		opt:           opt,
		stage:         dc.Stage,
		dc:            *dc,
		key:           key,
		expectVersion: types.DeployVer(version),
		info:          info,
		hcManager:     hcManager,
	}

	return &ctrl, nil
}

func (c *replicaCtrl) checkStatus() error {
	hc := c.hcManager.ListHostConfigs(c.key)
	numExp, numUnexp := c.hcStat(hc)
	if numExp != c.dc.NumOfInstance {
		return errors.Errorf("sched: host config spec is not consist with deploy config")
	}

	if numUnexp > 0 && c.legacyTimer == nil {
		msg := fmt.Sprintf("sched: %v legacy version instance is running but has not legacy task", numUnexp)
		err := c.removeLegacyInstanceConfigs()

		if err != nil {
			glog.Errorf("sched: remove legacy instance: %v", err)
			err = errors.WithMessage(err, msg)
		} else {
			err = errors.New(msg)
		}
		return err
	}

	insMap := c.info.RunningInstance(c.key)

	expVer := string(c.expectVersion)
	for _, ins := range insMap {
		if ins.Version == expVer {
			numExp--
		} else {
			numUnexp--
		}
	}

	var merr *multierror.Error

	if numExp > 0 {
		err := errors.Errorf("sched: there are %d instances with expected versino %v not running", numExp, expVer)
		merr = multierror.Append(merr, err)
	} else if numExp < 0 {
		err := errors.Errorf("sched: there are %d instances with expected versino %v not running", numExp, expVer)
		merr = multierror.Append(merr, err)
	}

	if numUnexp != 0 {
		err := errors.New("sched: actual running instances differ with  expectation")
		merr = multierror.Append(merr, err)
	}

	return merr.ErrorOrNil()
}

// start deploy a new project
func (c *replicaCtrl) Deploy(ctx context.Context, config *types.DeployConfig) error {
	c.dc = *config
	upo := config.UpdatePolicy
	if upo == nil {
		upo = types.GetDefaultUpdateOption(config.ServiceType)
		config.UpdatePolicy = upo
	}

	switch upo.Policy {
	case types.RollingUpdate:
		return c.rollingUpdate(ctx)
	case types.NewDeploy:
		return c.newDeploy(ctx)
	case types.MixedUpdate:
		return c.mixUpdate(ctx)
	default:
		glog.Fatalf("unknown deploy policy of " + string(config.Key()))
	}
	return nil
}

func (c *replicaCtrl) renewLease() {
	if c.legacyTimer != nil {
		c.legacyTimer.Reset(c.opt.legacyVerionTimeout)
	}
}

func (c *replicaCtrl) revokeLease() {
	if c.legacyTimer != nil {
		c.legacyTimer.Reset(time.Millisecond)
	}
}

func (c *replicaCtrl) Destroy() error {
	hc := c.hcManager.ListHostConfigs(c.key)

	hs := make([]types.HostID, 0, len(hc))
	return c.hcManager.DeleteHostConfigs(c.key, hs...)
}

func (c *replicaCtrl) rollingUpdate(ctx context.Context) error {
	hc := c.hcManager.ListHostConfigs(c.key)
	numExp, numUnexp := c.hcStat(hc)

	numUpdate := c.dc.NumOfInstance - numExp

	// 期望版本的实例数已经满足，
	if numUpdate == 0 {
		return c.removeLegacyInstanceConfigs()
	}

	if numUpdate < 0 {
		_, err := c.removeExpectInstances(ctx, -numUpdate)
		if err != nil {
			return err
		}
		return c.removeLegacyInstanceConfigs()
	}

	if numUpdate > numUnexp {
		if err := c.addInstances(ctx, numUpdate-numUnexp); err != nil {
			return err
		}
		numUpdate = numUnexp
	}

	n, err := c.updateInstances(ctx, numUpdate)
	if err != nil {
		return err
	}
	if n != numUpdate {
		glog.Errorf("sched: bug: rollingUpdate, updated num is not expected")
	}

	return c.removeLegacyInstanceConfigs()
}

func (c *replicaCtrl) newDeploy(ctx context.Context) error {
	num := c.dc.NumOfInstance
	if err := c.addInstances(ctx, num); err != nil {
		return err
	}

	if c.legacyTimer == nil {
		tm := time.AfterFunc(c.opt.legacyVerionTimeout, func() {
			c.removeLegacyInstanceConfigs()
			c.legacyTimer = nil
		})
		c.legacyTimer = tm
	}

	return nil
}

func (c *replicaCtrl) mixUpdate(ctx context.Context) error {
	dc := c.dc
	upo := dc.UpdatePolicy
	hc := c.hcManager.ListHostConfigs(c.key)

	numExp, numUnexp := c.hcStat(hc)
	var err error
	defer func() {
		if c.legacyTimer != nil && err == nil {
			tm := time.AfterFunc(c.opt.legacyVerionTimeout, func() {
				c.removeLegacyInstanceConfigs()
				c.legacyTimer = nil
			})
			c.legacyTimer = tm
		}
	}()

	if numExp > dc.NumOfInstance {
		_, err = c.removeExpectInstances(ctx, numExp-dc.NumOfInstance)
		if err != nil {
			return err
		}
	}

	if numExp >= dc.NumOfInstance {
		return nil
	}

	// we try our best to ensure num of legacy instance close to theory at last
	numNew := int(math.Ceil(float64(dc.NumOfInstance) * upo.NewPercent))
	numLegacy := numNew

	numUpdate := numUnexp - numLegacy
	if numUpdate <= 0 {
		numNew = dc.NumOfInstance - numExp
		err = c.addInstances(ctx, numNew)
		return err
	}

	numNew = dc.NumOfInstance - numUpdate
	if numNew > 0 {
		if err = c.addInstances(ctx, numNew); err != nil {
			return err
		}
	}

	if numUpdate > dc.NumOfInstance {
		if _, err = c.updateInstances(ctx, dc.NumOfInstance); err != nil {
			return err
		}
	}

	return nil
}

func (c *replicaCtrl) removeLegacyInstanceConfigs() error {
	hc := c.hcManager.ListHostConfigs(c.key)

	var merr *multierror.Error
	for h, cfg := range hc {
		needUpdate := false
		for v := range cfg.Info {
			if v != c.expectVersion {
				delete(cfg.Info, v)
				needUpdate = true
			}
		}
		var err error
		if len(cfg.Info) <= 0 {
			err = c.hcManager.DeleteHostConfigs(c.key, h)
		} else if needUpdate {
			err = c.hcManager.SetHostConfig(c.key, h, cfg)
		}
		if err != nil {
			merr = multierror.Append(merr, err)
		}
	}

	return merr.ErrorOrNil()
}

// updateInstances update n oldversion instances to expectVersion
// return num of instance updated, or err
func (c *replicaCtrl) updateInstances(ctx context.Context, num int) (int, error) {
	if num <= 0 {
		return 0, nil
	}

	hc := c.hcManager.ListHostConfigs(c.key)
	_, numUnexp := c.hcStat(hc)
	if num > numUnexp {
		num = numUnexp
	}

	count := 0
	var merr *multierror.Error
	step := 30 * time.Second
	if c.dc.UpdatePolicy != nil && c.dc.UpdatePolicy.Step > 0 {
		step = c.dc.UpdatePolicy.Step
	}
outer:
	// restart  old version instances
	for h, cfg := range hc {
		_, ok := cfg.Info[c.expectVersion]
		if len(cfg.Info) == 1 && ok || len(cfg.Info) == 0 {
			delete(hc, h)
			continue
		}

		for ver, n := range cfg.Info {
			if ver == c.expectVersion {
				continue
			}

			if count >= num {
				return num, nil
			}

			count++
			v := cfg.Info[c.expectVersion] + 1
			cfg.Info[c.expectVersion] = v
			n--
			if n == 0 {
				delete(cfg.Info, ver)
			} else {
				cfg.Info[ver] = n
			}

			if err := c.hcManager.SetHostConfig(c.key, h, cfg); err != nil {
				merr = multierror.Append(merr, err)
			}

			select {
			case <-ctx.Done():
				merr = multierror.Append(merr, ctx.Err())
				return count, merr.ErrorOrNil()
			case <-time.After(step):
			}

			continue outer
		}
	}
	err := merr.ErrorOrNil()
	return count, err
}

// addInstances deploy num new instances
func (c *replicaCtrl) addInstances(ctx context.Context, num int) error {
	req := toRequire(&c.dc)
	scheduler := newScheduler(c.stage, c.key, req, c.info)
	var merr *multierror.Error
	step := 30 * time.Second
	if c.dc.UpdatePolicy != nil && c.dc.UpdatePolicy.Step > 0 {
		step = c.dc.UpdatePolicy.Step
	}
	tm := time.NewTimer(0)
	for num > 0 {
		select {
		case <-ctx.Done():
			merr = multierror.Append(merr, ctx.Err())
			return merr.ErrorOrNil()
		case <-tm.C:
			tm.Reset(step)
			h, err := scheduler.NextHost()
			if err != nil {
				merr = multierror.Append(merr, err)
				continue
			}

			if err = c.addOneInstance(ctx, h); err != nil {
				merr = multierror.Append(merr, err)
			}
			num--
		}
	}

	return merr.ErrorOrNil()
}

func (c *replicaCtrl) addOneInstance(ctx context.Context, hostID types.HostID) error {
	spec := c.hcManager.GetHostConfig(c.key, hostID)
	if spec == nil {
		spec = &types.DeploySpec{}
	}

	nv := types.DeployVer(c.expectVersion)
	num := spec.Info[nv]
	spec.Info[nv] = num + 1

	return c.hcManager.SetHostConfig(c.key, hostID, *spec)
}

func (c *replicaCtrl) removeOneInstance(ctx context.Context, ver types.DeployVer, hostID types.HostID) error {
	spec := c.hcManager.GetHostConfig(c.key, hostID)
	if spec == nil {
		return nil
	}

	v, ok := spec.Info[ver]
	if !ok {
		return nil
	}

	v--
	if v <= 0 {
		delete(spec.Info, ver)
	}

	if len(spec.Info) == 0 {
		return c.hcManager.DeleteHostConfigs(c.key, hostID)
	}

	return c.hcManager.SetHostConfig(c.key, hostID, *spec)
}

func (c *replicaCtrl) updateOneInstance(ctx context.Context, hostID types.HostID, oldVer types.DeployVer) error {
	newVer := c.expectVersion
	if newVer == oldVer {
		return nil
	}

	spec := c.hcManager.GetHostConfig(c.key, hostID)
	if spec == nil {
		return nil
	}

	v, ok := spec.Info[oldVer]
	if !ok {
		return nil
	}

	v--
	if v <= 0 {
		delete(spec.Info, oldVer)
	}

	newVal := spec.Info[newVer]
	spec.Info[newVer] = newVal + 1
	return c.hcManager.SetHostConfig(c.key, hostID, *spec)
}

func (c *replicaCtrl) hcStat(hc map[types.HostID]types.DeploySpec) (int, int) {
	numExp := 0
	numUnExp := 0

	for _, cfg := range hc {
		for ver, num := range cfg.Info {
			if ver == c.expectVersion {
				numExp += num
			} else {
				numUnExp += num
			}
		}
	}

	return numExp, numUnExp
}

func (c *replicaCtrl) removeExpectInstances(ctx context.Context, num int) (int, error) {
	count := num
	step := 30 * time.Second
	if c.dc.UpdatePolicy != nil && c.dc.UpdatePolicy.Step > 0 {
		step = c.dc.UpdatePolicy.Step
	}
	for num > 0 {
		hc := c.hcManager.ListHostConfigs(c.key)
		for h, cfg := range hc {
			for ver := range cfg.Info {
				if ver != c.expectVersion {
					continue
				}
				if err := c.removeOneInstance(ctx, c.expectVersion, h); err != nil {
					return count - num, err
				}
				num--
				if num < 0 {
					return count, nil
				}
				select {
				case <-ctx.Done():
					return count - num, ctx.Err()
				case <-time.After(step):
				}
			}
		}
	}

	return count, nil
}
