package replica

import (
	"container/list"
	"context"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/golang/glog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"we.com/dolphin/registry/etcdkey"
	"we.com/dolphin/registry/generic"
	"we.com/dolphin/registry/watch"
	"we.com/dolphin/types"
	"we.com/jiabiao/common/fields"
	"we.com/jiabiao/common/labels"
)

type deployInfo struct {
	Key       types.DeployKey
	HostID    types.HostID
	Instances map[types.DeployVer]int
}

type logEntity struct {
	Time    time.Time
	Phase   string
	Message string
	Success bool
}

var (
	phUpdateConfig      = "update deploy config"
	phStart             = "start deploy"
	phSchedulerHosts    = "scheduler host"
	phUpdateNodeConfig  = "updateNode config"
	phWaitingInstanceUp = "waiting instance up"
	phDeployUptodate    = "deploy config update to date"
)

type Controller interface {
	Update(ctx context.Context, dc *types.DeployConfig) error
	Stop(ctx context.Context, ins *types.Instance) error
	Destroy(ctx context.Context) error
	Log(n int) []logEntity
}

//	controller is reponsable for deploy a new deployment or update an exist deployment
//	a deployment has a controller, when the deployment is removed, the associated controller is alse destroyed
type controller struct {
	opt          option
	stage        types.Stage
	hostConfig   map[types.HostID]*types.DeploySpec
	key          types.DeployKey
	deployConfig *types.DeployConfig
	maxlogs      int
	logs         *list.List // list of log entities
}

func newController(dc *types.DeployConfig, opt option) (*controller, error) {
	if dc == nil {
		return nil, errors.New("DeployConfig cannot be nil")
	}

	hc, err := loadDeployConfig(dc.Stage, dc.Key())
	if err != nil {
		return nil, err
	}

	c := controller{
		opt:          opt,
		stage:        dc.Stage,
		hostConfig:   hc,
		key:          dc.Key(),
		deployConfig: dc,
		maxlogs:      100,
		logs:         list.New(),
	}

	return &c, nil
}

// newDeployment deploy a new project
func (c *controller) newDeployment(ctx context.Context, config *types.DeployConfig, dryrun bool) error {
	upo := config.UpdatePolicy
	if upo == nil {
		upo = types.GetDefaultUpdateOption(config.ServiceType)
		config.UpdatePolicy = upo
	}

	c.deployConfig = config

	switch upo.Policy {
	case types.RollingUpdate:
		return c.rollingUpdate(ctx, dryrun)
	case types.NewDeploy:
		return c.newDeploy(ctx, dryrun)
	case types.MixedUpdate:
		return c.mixUpdate(ctx, dryrun)
	default:
		glog.Fatalf("unknown deploy policy of " + string(config.Key()))
	}

	return nil
}

func (c *controller) appendLog(en *logEntity) {
	c.logs.PushBack(en)
	for c.logs.Len() > c.maxlogs {
		c.logs.Remove(c.logs.Front())
	}
}

func (c *controller) StopInstance(ctx context.Context, ins *types.Instance) error {
	en := logEntity{
		Time:    time.Now(),
		Phase:   phUpdateNodeConfig,
		Success: true,
	}
	c.appendLog(&en)
	spec, err := c.getHostDeployspec(ins.HostID)
	if err != nil {
		en.Message = err.Error()
		en.Success = false
		return err
	}

	rv := types.DeployVer(ins.Version)
	v, ok := spec.Info[rv]
	if !ok {
		return nil
	}

	v--
	if v <= 0 {
		delete(spec.Info, rv)
	}

	if err := c.setHostDeployspec(ins.HostID, spec); err != nil {
		en.Message = err.Error()
		en.Success = false
		return err
	}

	if len(spec.Info) == 0 {
		delete(c.hostConfig, ins.HostID)
	} else {
		c.hostConfig[ins.HostID] = spec
	}

	en.Message = "udpate host deploy config success"
	en.Success = true

	return nil
}

func (c *controller) updateInstance(ctx context.Context, ins *types.Instance) error {
	en := logEntity{
		Time:    time.Now(),
		Phase:   phUpdateNodeConfig,
		Success: true,
	}
	c.appendLog(&en)
	spec, err := c.getHostDeployspec(ins.HostID)
	if err != nil {
		en.Message = err.Error()
		en.Success = false
		return err
	}

	nv := types.DeployVer(c.deployConfig.Image.Version.String())
	rv := types.DeployVer(ins.Version)
	v, ok := spec.Info[rv]
	if !ok || v <= 0 {
		versions := make([]string, 0, len(spec.Info))
		for k := range spec.Info {
			versions = append(versions, string(k))
		}
		vers := strings.Join(versions, ",")
		en.Message = fmt.Sprintf("instance %v has version %v, but not in config: %v, of num of configed instance is 0", ins.Pid, rv, vers)
		en.Success = false
		return errors.New("")
	}

	v--
	if v <= 0 {
		delete(spec.Info, rv)
	}

	ov := spec.Info[nv]
	spec.Info[nv] = ov + 1
	if err := c.setHostDeployspec(ins.HostID, spec); err != nil {
		en.Message = err.Error()
		en.Success = false
		return err
	}

	c.hostConfig[ins.HostID] = spec

	en.Message = "udpate success"
	en.Success = true

	// start to waiting for instance up
	c.waitInstanceUp(ctx, ins.HostID)
	return nil
}

func (c *controller) newInstance(ctx context.Context, hostID types.HostID) error {
	en := logEntity{
		Time:    time.Now(),
		Phase:   phUpdateNodeConfig,
		Success: true,
	}
	c.appendLog(&en)
	spec, err := c.getHostDeployspec(hostID)
	if err != nil {
		en.Message = err.Error()
		en.Success = false
		return err
	}

	nv := types.DeployVer(c.deployConfig.Image.Version.String())
	ov := spec.Info[nv]
	spec.Info[nv] = ov + 1
	if err := c.setHostDeployspec(hostID, spec); err != nil {
		en.Message = err.Error()
		en.Success = false
		return err
	}

	c.hostConfig[hostID] = spec
	en.Message = "udpate success"
	en.Success = true

	// start to waiting for instance up
	c.waitInstanceUp(ctx, hostID)
	return nil
}

// waitInstanceUp block until instance is up,  and  probe  success
// for onetime instance, it may exist shortly after start, in this case, we think it is up
// todo: better way to  recognize the new started instance, before start the instance, we can
// generate a instanceID and set it to instance's envmap, so we can parse envmap for the new
// started instances
// for now we just compare instances version  and  startTime
func (c *controller) waitInstanceUp(ctx context.Context, hostID types.HostID) error {
	path := etcdkey.DeployInstanceDirOfKey(c.stage, c.key)

	req := labels.Set{"hostID": string(hostID)}
	pred := generic.SelectionPredicate{
		Label: req.AsSelector(),
		Field: fields.Everything(),
		GetAttrs: func(obj interface{}) (labels.Set, fields.Set, error) {
			ins := obj.(*types.HostInfo)
			return labels.Set{"hostID": string(ins.HostID)}, nil, nil
		},
	}

	dc := c.deployConfig
	nv := types.DeployVer(dc.Image.Version.String())

	h := func(e watch.Event) error {
		dat, ok := e.Object.(*types.Instance)
		if !ok {
			glog.Fatalf("event object must be an instance of *types.Instance, got %T", e.Object)
		}

		switch e.Type {
		case watch.Added, watch.Modified:
			rv := types.DeployVer(dat.Version)
			if rv != nv {
				return nil
			}

			if dc.RestartPolicy.Type == types.OneTime {
				if dat.StartTime.Add(10 * time.Second).After(time.Now()) {

					return errStop
				}
			} else {
				// todo: so we should not start two instance withing 2mins
				// or if host date is not sync,
				if dat.StartTime.Add(2 * time.Minute).After(time.Now()) {
					if dat.LifeCycle == types.LCRunning {
						return errStop
					}
					return nil
				}
			}
		}

		return nil
	}

	return watchEvent(ctx, path, pred, reflect.TypeOf(types.Instance{}), h)
}

func (c *controller) updateInstances(ctx context.Context, ins []*types.Instance) error {
	f := func(ins *types.Instance) func() error {
		return func() error {
			return c.updateInstance(ctx, ins)
		}
	}

	insMap := make(map[types.InstanceID]*types.Instance, len(ins))
	for _, v := range ins {
		insMap[v.ID] = v
	}

	p1 := types.HostID("")
	p2 := types.HostID("")
	//
	s := func() *types.Instance {
		if len(insMap) == 0 {
			return nil
		}

		var ret *types.Instance
		defer func() {
			p1 = p2
			p2 = ret.HostID
			delete(insMap, ret.ID)
		}()

		var b1 *types.Instance
		var b2 *types.Instance
		for k, v := range insMap {
			if v.HostID != p2 && v.HostID != p1 {
				ret = v
				return ret
			}
			if v.HostID != p2 {
				b1 = v
				continue
			}
			b2 = v
		}
		if b1 != nil {
			ret = b1
			return ret
		}

		ret = b2
		return ret
	}

	r := newRunner(ctx, 1)
	for len(ins) > 0 {
		v := s()
		select {
		case err := <-r.run(f(v)):
			if err != nil {
				return err
			}
		case <-ctx.Done():
			err := ctx.Err()
			return err
		}
	}

	return nil
}

func (c *controller) newInstances(ctx context.Context, num int) error {
	//	req := toRequire(dc)
	scheduler := newScheduler()
	var merr *multierror.Error
	for num > 0 {

		en := logEntity{
			Time:  time.Now(),
			Phase: phSchedulerHosts,
		}
		c.appendLog(&en)
		h, err := scheduler.NextHost()
		if err != nil {
			merr = multierror.Append(merr, err)
			en.Success = false
			en.Message = err.Error()
			continue
		}

		if err = c.newInstance(ctx, h); err != nil {
			merr = multierror.Append(merr, err)
		}
		num--
	}

	return merr.ErrorOrNil()
}

func (c *controller) rollingUpdate(ctx context.Context, dryrun bool) error {
	oldIns, err := c.getRunningProcess()
	if err != nil {
		return err
	}

	return c.updateInstances(ctx, oldIns)
}

func (c *controller) newDeploy(ctx context.Context, dryrun bool) error {
	dc := c.deployConfig
	num := dc.NumOfInstance
	return c.newInstances(ctx, num)
}

func (c *controller) mixUpdate(ctx context.Context, dryrun bool) error {
	dc := c.deployConfig
	upo := dc.UpdatePolicy
	ins, err := c.getRunningProcess()
	if err != nil {
		return err
	}

	numNew := int(math.Ceil(float64(dc.NumOfInstance) * upo.NewPercent))
	if numNew+len(ins) < dc.NumOfInstance {
		numNew = dc.NumOfInstance - len(ins)
	}

	err = c.newInstances(ctx, numNew)
	if err != nil {
		return err
	}

	//

	return nil
}

func (c *controller) setHostDeployspec(hostID types.HostID, spec *types.DeploySpec) error {
	store, err := getStore()
	if err != nil {
		return err
	}

	path := etcdkey.DeployHostExpectPathOf(c.stage, hostID, c.key)
	if spec != nil && len(spec.Info) > 0 {
		return store.Update(context.Background(), path, spec, nil, 0)
	}

	return store.Delete(context.Background(), path, nil)
}

func (c *controller) getHostDeployspec(hostID types.HostID) (*types.DeploySpec, error) {
	path := etcdkey.DeployHostExpectPathOf(c.stage, hostID, c.key)
	ret := types.DeploySpec{}

	if err := getObject(path, &ret); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &ret, nil
}

func (c *controller) addNewInstances(host types.HostID, info types.DeploySpec) (*types.DeploySpec, error) {
	old, err := c.getHostDeployspec(host)
	if err != nil {
		return nil, err
	}
	if old == nil {
		old = &types.DeploySpec{}
	}

	for k, v := range info.Info {
		t := old.Info[k]
		old.Info[k] = t + v
	}

	err = c.setHostDeployspec(host, old)
	if err != nil {
		return nil, err
	}
	return old, nil
}

func (c *controller) getRunningProcess() ([]*types.Instance, error) {
	return getRunningInstances(c.stage, c.key)
}

func (c *controller) getDeployConfig() (*types.DeployConfig, error) {
	path := etcdkey.DepoyConfigOfKey(c.stage, c.key)

	ret := types.DeployConfig{}

	if err := getObject(path, &ret); err != nil {
		return nil, err
	}

	return &ret, nil
}

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
