package ps

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/process"
	"we.com/dolphin/types"
	"we.com/dolphin/types/hostinfo"
	"we.com/dolphin/types/ins/registry"
	"we.com/jiabiao/common/probe"
)

const (
	// UnknownDeployKey unknown deploy key
	UnknownDeployKey types.DeployKey = "unknown"

	envDeployKey  = "_depolyKey"
	envVersion    = "_version"
	envNode       = "_nodeName"
	envInstanceID = "_instanceID"
)

// Scanner scan instances running on the host, and probe them eveny 5s
type Scanner interface {
	Watch(ctx context.Context) (<-chan InstanceEvent, <-chan Metric)
}

type EventType string

const (
	ETStarting  EventType = "starting"
	ETStarted   EventType = "started"
	ETProbeErr  EventType = "probeErr"
	ETProbeWarn EventType = "prbeWarn"
	ETStopping  EventType = "stopping"
	ETStopped   EventType = "stopped"
)

type InstanceEvent struct {
	Type EventType
	Ins  *types.Instance
}

// NewScanner returns a new scanner
func NewScanner(rp registry.ResourceInfor) Scanner {
	return &scanner{
		rp:         rp,
		procs:      map[int]*process.Process{},
		instances:  map[types.DeployKey]*types.Instance{},
		stopped:    map[types.DeployKey]*types.Instance{},
		eventChan:  make(chan InstanceEvent, 5),
		metricChan: make(chan Metric, 200),
	}
}

/*
	对每个已知类型的实例，启动一个goroutine 监控其状态，当实例退出后， 将实例放到
	stopped map中， 同时从instances map删除。实例退出5分钟的，从stopped map删除。
*/
type scanner struct {
	lock       sync.RWMutex
	wg         sync.WaitGroup
	rp         registry.ResourceInfor
	procs      map[int]*process.Process
	instances  map[types.DeployKey]*types.Instance
	stopped    map[types.DeployKey]*types.Instance
	eventChan  chan InstanceEvent
	metricChan chan Metric
}

// watch watches all instance status running on this host
func (s *scanner) Watch(ctx context.Context) (<-chan InstanceEvent, <-chan Metric) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := s.update(); err != nil {
					glog.Errorf("ps: update process list %v", err)
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	return s.eventChan, s.metricChan
}

func (s *scanner) update() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	for pid := range s.procs {
		if isProcessStopped(pid) {
			delete(s.procs, pid)
		}
	}

	pids := GetAllPids()

	for _, pid := range pids {
		if _, ok := s.procs[pid]; !ok {
			p, err := process.NewProcess(int32(pid))
			if err != nil {
				continue
			}
			s.procs[pid] = p
			p.Percent(0)
			ins, err := s.parse(p)
			if err != nil {
				glog.Infof("ps: parse instance info %v", err)
			}
			if ins != nil {
				glog.Infof("ps: new instance (%v,%v)", ins.DeployKey(), pid)
				s.instances[ins.DeployKey()] = ins
				s.eventChan <- InstanceEvent{
					Type: ETStarting,
					Ins:  ins,
				}
				go s.watchInstance(ins, registry.GetTypeInfo(ins.ProjecType))
			}
		}
	}

	return nil
}

func (s *scanner) watchInstance(ins *types.Instance, typeInfo *registry.TypeInfo) error {
	if ins == nil {
		return nil
	}

	r := rand.Intn(5)
	time.Sleep(time.Duration(r) * time.Second)

	d := 5 * time.Second
	timer := time.NewTimer(d)
	go listenPorts(ins)

	prevSt := types.InstanceUnknown

	key := ins.DeployKey()
	proc := s.procs[ins.Pid]

	for {
		timer.Reset(d)
		select {
		case n := <-timer.C:
			if isProcessStopped(ins.Pid) {
				s.eventChan <- InstanceEvent{
					Type: ETStopped,
					Ins:  ins,
				}
				return nil
			}

			st := getProcessState(proc)

			rr := s.rp.GetDeployResouce(key)
			if err := Probe(ins, st, rr); err != nil {
				if s.metricChan != nil {
					s.metricChan <- getMetrics(ins, st)
				}
				glog.Errorf("ps:  probe instance %v err: %v", ins.Pid, err)
				continue
			}

			ins.UpdateTime = n
			if s.metricChan != nil {
				s.metricChan <- getMetrics(ins, st)
			}

			// probe status changed
			if ins.Status != prevSt && s.eventChan != nil {
				glog.V(10).Infof("ps: %v prev status :%v, new status: %v", ins.DeployKey(), prevSt, ins.Status)
				prevSt = ins.Status
				ev := InstanceEvent{
					Ins: ins,
				}
				switch ins.Status {
				case types.InstanceError:
					ev.Type = ETProbeErr
				case types.InstanceWarning:
					ev.Type = ETProbeWarn
				case types.InstanceSuccess:
					ev.Type = ETStarted
				default:
					ev.Type = ETProbeWarn
				}

				s.eventChan <- ev
			}
		}
	}
}

func (s *scanner) parse(proc *process.Process) (*types.Instance, error) {
	pid := int(proc.Pid)
	if isProcessStopped(pid) {
		return nil, errors.New("process existed")
	}

	info := &insInfo{
		proc: proc,
	}

	dkey := info.getDeployKey()

	var typ types.ProjectType
	var dname types.DeployName
	if string(dkey) == "" {
		typ = registry.GetInstanceType(info)
		if typ == registry.PTUnknown {
			return nil, nil
		}
	} else {

		tp, dnameStr, err := types.ParseDeployKey(dkey)
		if err != nil {
			return nil, err
		}
		typ = types.ProjectType(tp)
		dname = types.DeployName(dnameStr)
	}

	typeInfo := registry.GetTypeInfo(typ)
	if typeInfo == nil {
		return nil, nil
	}

	ctime, _ := proc.CreateTime()
	hostID := hostinfo.GetHostID()
	user, err := proc.Username()
	if err != nil {
		glog.Warningf("ps: parse instance info: %v", err)
	}

	instanceID := info.GetInstanceID()
	if string(instanceID) == "" {
		instanceID = types.InstanceID(fmt.Sprintf("%s-%v", hostID[:7], proc.Pid))
	}

	ins := &types.Instance{
		ProjecType: typ,
		ID:         instanceID,
		Pid:        pid,
		DeployName: dname,

		// Host info
		User:   user,
		HostID: hostID,
		Host:   hostinfo.GetHostName(),
		IP:     hostinfo.GetInternalIP(),
		Stage:  hostinfo.GetStage(),

		StartTime:  time.Unix(ctime/1000, (ctime%1000)*1e6),
		UpdateTime: time.Now(),
		LifeCycle:  types.LCStarting,
	}

	if err := typeInfo.Parse(ins, info); err != nil {
		return nil, err
	}

	return ins, nil
}

func getEnvMap(pid int) (map[string]string, error) {
	path := fmt.Sprintf("/proc/%v/environ", pid)

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	content, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	ret := map[string]string{}

	parts := bytes.Split(content, []byte{0})

	for _, v := range parts {
		e := string(v)
		env := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}
		ret[env[0]] = env[1]
	}
	return ret, nil
}

func listenPorts(ins *types.Instance) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 125*time.Second)
	defer cancel()

	count := uint(0)

	tick := time.NewTimer(4 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if isProcessStopped(ins.Pid) {
				glog.Infof("ps: get process listen port, process existed: %v", ins.DeployKey())
				return
			}
			addrs, err := ListenPortsOfPid(ins.Pid)
			if err != nil {
				glog.Warningf("get listening port of %v: %v", ins.Pid, err)
			}
			if len(addrs) > 0 {
				glog.V(10).Infof("process %v listening: %v", ins.Pid, addrs)
				ins.Listening = addrs
				ins.ServiceType = types.ServiceService
				ins.LifeCycle = types.LCRunning
				return
			}

			count++
			tick.Reset((4 << count) * time.Second)
		case <-ctx.Done():
			glog.V(11).Infof("not find listen port for process %v", ins.Pid)
			ins.LifeCycle = types.LCRunning
			return
		}
	}
}

type insInfo struct {
	envMap map[string]string
	exe    string
	args   string
	proc   *process.Process
}

func (i *insInfo) GetExe() string {
	if i.exe != "" {
		return i.exe
	}
	exe, _ := i.proc.Exe()
	i.exe = exe
	return exe
}

func (i *insInfo) GetEnvMap() map[string]string {
	if i.envMap != nil {
		return i.envMap
	}
	envMap, _ := getEnvMap(int(i.proc.Pid))
	if envMap == nil {
		envMap = map[string]string{}
	}
	i.envMap = envMap
	return envMap
}

func (i *insInfo) GetArgs() string {
	if i.args != "" {
		return i.args
	}

	args, _ := i.proc.Cmdline()
	i.args = args
	return args
}

func (i *insInfo) GetVersion() string {
	envMap := i.GetEnvMap()
	val := envMap[envVersion]
	return val
}

func (i *insInfo) getDeployKey() types.DeployKey {
	envMap := i.GetEnvMap()
	val := envMap[envDeployKey]
	return types.DeployKey(val)
}

func (i *insInfo) GetInstanceID() types.InstanceID {
	envMap := i.GetEnvMap()

	val := envMap[envInstanceID]
	return types.InstanceID(val)
}

func isProcessStopped(pid int) bool {
	p, _ := os.FindProcess(pid)
	err := p.Signal(syscall.Signal(0))
	return err != nil
}

// Metric  process metric
type Metric struct {
	Name   string
	Fields map[string]interface{}
	Tags   map[string]string
	Time   time.Time
}

// Stop call stop  script to stop an instance
// it will make sure the process is stopped
func Stop(ctx context.Context, ins *types.Instance, force bool) {
	args := ins.StopCmdArgs()
	stop(ctx, args[:], nil)
	p, _ := os.FindProcess(ins.Pid)
	if force {
		p.Kill()
	}
}

// Start starts a new instance of key
func Start(ctx context.Context, key types.DeployKey) ([]byte, error) {
	args := []string{string(key)}

	//todo: get the pid of new started service
	// throught pid file ?
	return start(ctx, args, nil)
}

// Probe checks process resouce usage, and probe instance status through the given probe method
func Probe(ins *types.Instance, st *ProcessState, resReq *types.DeployResource) (err error) {
	if ins == nil {
		return
	}

	var conditions []*types.Condition
	var events []*types.Condition

	typ := ins.ProjecType
	pt := registry.GetTypeInfo(typ)
	if pt == nil {
		err = errors.Errorf("unknown procees type %v", typ)
		return
	}

	if resReq == nil {
		spec := registry.GetDefaultDeployResource(registry.StageType{
			Stage: ins.Stage,
			Type:  ins.ProjecType,
		})
		if spec != nil {
			resReq = &spec.Medium
		}
	}

	if resReq != nil {
		conditions, events, err = st.check(resReq)
		if err != nil {
			return
		}
	}

	if pt.Prober != nil {
		result, err := pt.Prober.Probe(ins)
		if err != nil {
			return err
		}
		cond := &types.Condition{
			Type:    types.ProbeCondition,
			Message: "probe status  failuer",
		}

		switch result {
		case probe.Failure:
			conditions = append(conditions, cond)
		case probe.Warning:
			events = append(events, cond)
		}
	}

	ins.Conditions = conditions
	ins.Events = events
	if len(events) > 0 {
		ins.Status = types.InstanceWarning
	} else if len(conditions) > 0 {
		ins.Status = types.InstanceError
	} else {
		ins.Status = types.InstanceSuccess
	}

	return nil
}

func getMetrics(ins *types.Instance, state *ProcessState) Metric {
	tags := map[string]string{
		"dkey":        string(ins.DeployKey()),
		"ptype":       string(ins.ProjecType),
		"deploy":      string(ins.DeployName),
		"probeStatus": string(ins.Status),
		"env":         string(ins.Stage),
		"esuer":       ins.User,
		"node":        string(ins.ID),
	}

	fileds := state.GetMetric()
	if fileds != nil {
		fileds["pid"] = ins.Pid
	}

	return Metric{
		Name:   fmt.Sprintf("monitor_%v", ins.ProjecType),
		Fields: fileds,
		Tags:   tags,
		Time:   ins.UpdateTime,
	}
}
