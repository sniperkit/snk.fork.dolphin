package replica

import (
	"container/list"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	ctypes "we.com/dolphin/controllers/types"
	ps "we.com/dolphin/process"
	"we.com/dolphin/registry/etcdkey"
	"we.com/dolphin/registry/generic"
	"we.com/dolphin/types"
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

//	controller is reponsable for deploy a new deployment or update an exist deployment
//	a deployment has a controller, when the deployment is removed, the associated controller is alse destroyed
type controller struct {
	opt          option
	stage        types.Stage
	key          types.DeployKey
	deployConfig *types.DeployConfig
	maxlogs      int
	logs         list.List // list of log entities
}

type runner struct {
	ctx context.Context
	c   chan struct{}
}

func newRunner(ctx context.Context, worker int) *runner {
	if worker <= 0 {
		return nil
	}

	worker++
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

func (c *controller) appendLog(en *logEntity) {
	c.logs.PushBack(en)
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

	c := ctx.Done()

	ctx.Err()

	en.Message = "udpate success"
	en.Success = true

	// start to waiting for instance up
	en = logEntity{
		Time:    time.Now(),
		Phase:   phWaitingInstanceUp,
		Success: true,
	}
	c.appendLog(&en)

	return nil
}

// waitInstanceUp block until instance is up,  and  probe  success
// for onetime instance, it may exist shortly after start, in this case, we think it is up
func (c *controller) waitInstanceUp(ctx context.Context, hostID types.HostID) error {
	dc := c.deployConfig
	if dc.RestartPolicy.Type == types.OneTime {

	}
	return nil
}

func (c *controller) rollingUpdate(ctx context.Context, dc *types.DeployConfig, dryrun bool) error {
	oldIns, err := c.getRunningProcess()
	if err != nil {
		return err
	}

	for _, v := range oldIns {
		select {
		case err := <-Go(updateInstance, ctx, v):
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

func (c *controller) newDeploy(ctx context.Context, dc *types.DeployConfig, dryrun bool) error {
	//	req := toRequire(dc)
	scheduler := newScheduler()

	h, err := scheduler.NextHost()
	if err != nil {
		return err
	}
	return nil
}

func (c *controller) mixUpdate(ctx context.Context, dc *types.DeployConfig, dryrun bool) error {
	ins, err := c.getRunningProcess()
	if err != nil {
		return err
	}
	return nil
}

// newDeployment deployment a new project
func (c *controller) newDeployment(ctx context.Context, config *types.DeployConfig, dryrun bool) error {
	upo := config.UpdatePolicy
	if upo == nil {
		upo = types.GetDefaultUpdateOption(config.ServiceType)
		config.UpdatePolicy = upo
	}

	switch upo.Policy {
	case types.RollingUpdate:
		return c.rollingUpdate(ctx, config, dryrun)
	case types.NewDeploy:
		return c.newDeploy(ctx, config, dryrun)
	case types.MixedUpdate:
		return c.mixUpdate(ctx, config, dryrun)
	default:
		glog.Fatalf("unknown deploy policy of " + string(config.Key()))
	}

	return nil
}

func (c *controller) setHostDeployspec(hostID types.HostID, spec *types.DeploySpec) error {
	store, err := getStore()
	if err != nil {
		return err
	}

	path := etcdkey.DeployHostExpectPathOf(c.stage, hostID, c.key)
	return store.Update(context.Background(), path, spec, nil, 0)
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

func getRunningInstances(stage types.Stage, key types.DeployKey) ([]*types.Instance, error) {
	path := etcdkey.DeployInstanceDirOfKey(stage, key)
	ret := []*types.Instance{}

	store, err := getStore()
	if err != nil {
		return nil, err
	}

	if err := store.List(context.Background(), path, generic.Everything, &ret); err != nil {
		return nil, err
	}

	// skip stopped instances
	for _, v := range ret {
		if v.LifeCycle != types.LCStopped {
			ret = append(ret, v)
		}
	}
	return ret, nil
}

// getNewHosts return hosts which has no deployment of key
func getNewHosts(c []types.HostID, ins []*types.Instance) []types.HostID {
	ret := []types.HostID{}
outer:
	for _, v := range c {
		for _, i := range ins {
			if i.HostID == v {
				continue outer
			}
		}
		ret = append(ret, v)
	}

	return ret
}

func getStore() (generic.Interface, error) {
	return nil, nil
}

func toRequire(dc *types.DeployConfig) *ctypes.Require {
	s := dc.GetSelector()

	rr := dc.ResourceRequired
	if rr == nil {
		t := ps.GetDefaultDeployResource(ps.StageType{Stage: dc.Stage, Type: dc.Type})
		if t != nil {
			rr = &t.Medium
		}
	}
	if rr == nil {
		rr = &types.DeployResource{}
	}

	ret := &ctypes.Require{
		HostSelector: s,
		Resource:     *rr,
	}

	return ret
}

func getHostStatus(stage types.Stage, hostID types.HostID) (*types.HostStatus, error) {
	path := etcdkey.HostStatPath(stage, hostID)
	ret := types.HostStatus{}

	if err := getObject(path, &ret); err != nil {
		return nil, err
	}

	return &ret, nil
}

func getHostInfo(stage types.Stage, hostID types.HostID) (*types.HostInfo, error) {
	path := etcdkey.HostStatPath(stage, hostID)
	ret := types.HostInfo{}

	if err := getObject(path, &ret); err != nil {
		return nil, err
	}

	return &ret, nil
}

func getObject(path string, obj interface{}) error {
	store, err := getStore()
	if err != nil {
		return err
	}

	if err := store.Get(context.Background(), path, obj, false); err != nil {
		return err
	}
	return nil
}
