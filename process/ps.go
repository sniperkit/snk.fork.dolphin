package ps

import (
	"context"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/process"
	"we.com/dolphin/types"
	"we.com/jiabiao/common/probe"
)

type ps struct {
	typeInfo map[types.ProjectType]*PidType

	instances map[types.DeployKey]map[int]*types.Instance
	processes map[int]*process.Process

	updateTime time.Time
	lock       sync.RWMutex
	cacheTime  time.Duration
}

func (p *ps) update(force bool) {
	if p.updateTime.Add(p.cacheTime).After(time.Now()) && !force {
		return
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.updateTime.Add(p.cacheTime).After(time.Now()) && !force {
		return
	}
	p.forceUpdate()
}

// forceUpdate all running process
func (p *ps) forceUpdate() {
	pids := GetAllPids()

	pidMap := make(map[int]struct{}, len(pids))
	for _, v := range pids {
		pidMap[v] = struct{}{}
	}

	p.classifyProcs(pidMap)
}

func processStopped(pid int) bool {
	p, _ := os.FindProcess(pid)
	err := p.Signal(syscall.Signal(0))
	return err != nil
}

// caller should hold the lock
func (p *ps) classifyProcs(pidMap map[int]struct{}) {
	// clean stopped processes
	// https://stackoverflow.com/questions/11323410/linux-pid-recycling
	// todo: process pid stopped, and then a new process with pid start
	for k, dmap := range p.instances {
		for pid := range dmap {
			if processStopped(pid) {
				delete(p.processes, pid)
				delete(dmap, pid)
			} else {
				delete(pidMap, pid)
			}
		}
		if len(dmap) == 0 {
			delete(p.instances, k)
		}
	}

	unknownMap, ok := p.instances[unknown]
	if !ok {
		unknownMap = map[int]*types.Instance{}
		p.instances[unknown] = unknownMap
	}

	instances := p.instances

	// new started process
out:
	for pid := range pidMap {
		proc, err := process.NewProcess(int32(pid))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			glog.Warningf("ps: get system running processes: %v", err)
			continue
		}

		cmdline, err := proc.Cmdline()
		if err != nil {
			glog.Warningf("ps: get process cmdline: %v", err)
			continue
		}

		for _, v := range p.typeInfo {
			if !matchCmdline([]byte(cmdline), v) {
				continue // continue inner loop
			}

			ins, err := v.Parse.Parse(pid)
			if err != nil {
				glog.Errorf("ps: parse process instance: %v", err)
				continue out
			}

			key := ins.DeployKey()

			insMap, ok := instances[key]
			if !ok {
				insMap = map[int]*types.Instance{}
				instances[key] = insMap
			}

			insMap[pid] = ins
			p.processes[pid] = proc
			continue out
		}
		unknownMap[pid] = nil
		p.processes[pid] = proc
	}

	p.instances = instances
}

// GetInstanceOfDeploy returns process of key
func (p *ps) GetInstanceOfDeploy(key types.DeployKey, forceUpdate bool) (map[int]*types.Instance, error) {
	// update if  needed
	p.update(forceUpdate)

	p.lock.RLock()
	defer p.lock.RUnlock()

	tmp, ok := p.instances[key]
	if !ok {
		typ, _, err := types.ParseDeployKey(key)
		if err != nil {
			return nil, err
		}
		if _, ok := p.typeInfo[typ]; !ok {
			return nil, errors.New("unknown typ")
		}

		return nil, nil
	}

	ret := make(map[int]*types.Instance, len(tmp))
	for k, v := range tmp {
		ret[k] = v
	}

	return ret, nil
}

func (p *ps) GetDeployKeys() map[types.DeployKey]struct{} {
	// update if  needed
	p.update(false)

	p.lock.RLock()
	defer p.lock.RUnlock()
	ret := make(map[types.DeployKey]struct{}, len(p.instances))

	for k := range p.instances {
		ret[k] = struct{}{}
	}

	return ret
}

func (p *ps) GetInstanceOfType(typ types.ProjectType) (map[int]*types.Instance, error) {
	// update if  needed
	p.update(false)

	p.lock.RLock()
	defer p.lock.RUnlock()

	ret := make(map[int]*types.Instance)

	for key, v := range p.instances {
		if key == unknown {
			continue
		}
		t, _, err := types.ParseDeployKey(key)
		if err != nil {
			return nil, err
		}
		if typ != t {
			continue
		}
		for pid, i := range v {
			ret[pid] = i
		}
	}

	return ret, nil
}

func (p *ps) GetDeployedProjectTypes() (map[types.ProjectType]struct{}, error) {
	// update if  needed
	p.update(false)
	ret := map[types.ProjectType]struct{}{}
	p.lock.RLock()
	defer p.lock.RUnlock()
	for key := range p.instances {
		if key == unknown {
			continue
		}
		t, _, err := types.ParseDeployKey(key)
		if err != nil {
			return nil, err
		}

		ret[t] = struct{}{}
	}
	return ret, nil
}

var (
	psManager = &ps{
		typeInfo:  map[types.ProjectType]*PidType{},
		instances: map[types.DeployKey]map[int]*types.Instance{},
		processes: map[int]*process.Process{},
	}
)

// Metric  process metric
type Metric struct {
	Name   string
	Fields map[string]interface{}
	Tags   map[string]string
	Time   time.Time
}

// GetInstanceOfDeploy  most of the times, forceUpdate should set to force
// defaut processes will be cached for cacheTime
// typ is specified when setup
func GetInstanceOfDeploy(key types.DeployKey, forceUpdate bool) (map[int]*types.Instance, error) {
	ins, err := psManager.GetInstanceOfDeploy(key, forceUpdate)
	if err != nil {
		return nil, err
	}

	return ins, nil
}

// GetInstanceOfType return instances of the given type
func GetInstanceOfType(typ types.ProjectType) (map[int]*types.Instance, error) {
	return psManager.GetInstanceOfType(typ)
}

// GetDeployKeys return all  running deployKeys on this host
func GetDeployKeys() map[types.DeployKey]struct{} {
	return psManager.GetDeployKeys()
}

// GetDeployedProjectTypes return all projectType running on this host
func GetDeployedProjectTypes() (map[types.ProjectType]struct{}, error) {
	return psManager.GetDeployedProjectTypes()
}

// ProcessResource get process resource usage info
func ProcessResource(ins *types.Instance) (*ProcessState, error) {
	if ins == nil {
		return nil, nil
	}

	psManager.lock.RLock()
	proc, ok := psManager.processes[ins.Pid]
	psManager.lock.RUnlock()

	if !ok {
		return nil, errors.New("process does not exist")
	}

	return getProcessState(proc), nil
}

// Probe checks process resouce usage, and probe instance status through the given probe method
func Probe(ins *types.Instance, st *ProcessState, resReq *types.DeployResource) (conditions []*types.Condition, events []*types.Condition, err error) {
	if ins == nil {
		return nil, nil, nil
	}

	typ := ins.ProjecType
	pt, ok := psManager.typeInfo[typ]
	if !ok {
		return nil, nil, errors.Errorf("unknown procees type %v", typ)
	}

	conditions, events, err = st.check(resReq)
	if err != nil {
		return
	}

	if pt.Prober == nil {
		return
	}

	result, err := pt.Prober.Probe(ins)
	if result == probe.Success {
		return
	}

	cond := &types.Condition{
		Type:    types.ProbeCondition,
		Message: "probe status  failuer",
	}

	if result == probe.Failure {
		conditions = append(conditions, cond)
		return
	}

	if result == probe.Warning {
		events = append(events, cond)
		return
	}

	return nil, nil, nil
}

// Stop call stop  script to stop an instance
// it will make sure the process is stopped
func Stop(ctx context.Context, ins *types.Instance) {
	args := ins.StopCmdArgs()
	stop(ctx, args[:], nil)
	p, _ := os.FindProcess(ins.Pid)
	p.Kill()
}

// Start starts a new instance of key
func Start(ctx context.Context, key types.DeployKey) error {
	args := []string{string(key)}

	//todo: get the pid of new started service
	// throught pid file ?
	err := start(ctx, args, nil)

	return err
}

// Register  register a project type
func Register(typ types.ProjectType, pt PidType) error {
	psManager.lock.Lock()
	defer psManager.lock.Unlock()
	if _, ok := psManager.typeInfo[typ]; ok {
		return errors.New("project type already exists")
	}

	if _, err := pt.GetRegexp(); err != nil {
		return errors.Wrap(err, "check regexp failed")
	}

	if pt.Parse == nil {
		return errors.New("ps: Parse cannot be nil when register")
	}

	psManager.typeInfo[typ] = &pt

	return nil
}
