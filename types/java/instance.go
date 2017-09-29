package java

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/shirou/gopsutil/process"
	ps "we.com/dolphin/process"
	"we.com/dolphin/types"
	"we.com/dolphin/types/hostinfo"
	"we.com/jiabiao/common/probe"
)

const (
	// Type java project type
	Type          = types.ProjectType("java")
	fieldSperator = ":"
)

// InstanceInfo addition instance info
type InstanceInfo struct {
	NodeName    string       `json:"nodeName,omitempty"`
	ProbeStatus probe.Result `json:"probeStatus,omitempty"`
	UpdateTime  time.Time    `json:"updateTime,omitempty"`

	Listening []types.Addr `json:"listening,omitempty"`
	RouteInfo string       `json:"routeInfo,omitempty"`

	ServiceType types.ServiceType `json:"serviceType,omitempty"`

	// A pointer to outer common instance info
	*types.Instance
}

// NewInstanceInfo parse processInfo from process
func NewInstanceInfo(proc *process.Process) (*types.Instance, error) {
	ctime, _ := proc.CreateTime()
	// start to calculate cpu usage
	proc.Percent(0)

	user, err := proc.Username()
	if err != nil {
		glog.Errorf("error get project user: %v", err)
	}

	hostID := hostinfo.GetHostID()

	gi := &types.Instance{
		ProjecType: Type,
		ID:         types.InstanceID(fmt.Sprintf("%s-%v", hostID[:7], proc.Pid)),
		Pid:        int(proc.Pid),

		//DeployName: "",
		// Instance info
		//	Version    string    `json:"version,omitempty"`

		// Host info
		User:   user,
		HostID: hostID,
		Host:   hostinfo.GetHostName(),
		IP:     hostinfo.GetInternalIP(),
		Stage:  hostinfo.GetStage(),

		StartTime:  time.Unix(ctime/1000, (ctime%1000)*1e6),
		UpdateTime: time.Now(),
		LifeCycle:  types.LCStarting,

		//	Listening []Addr `json:"listening,omitempyt,omitempty"`

		// status info
		//	Status     InstanceStatus    `json:"status,omitempty"`
		//	Conditions []*Condition      `json:"conditions,omitempty"` // error
		//	Events     []*Condition      `json:"events,omitempty"`     // warning
		//	ResUsage   *InstanceResUsage `json:"resUsage,omitempty"`

		// addional info  of specific type
		//	Private interface{} `json:"provite,omitempyt,omitempty"`
	}

	ii := &InstanceInfo{}

	// update instance info
	if err = ii.parseInfo(proc); err != nil {
		return nil, err
	}

	ii.Instance = gi
	gi.Private = ii

	return gi, nil
}

func (ii *InstanceInfo) parseInfo(p *process.Process) error {
	args, err := p.CmdlineSlice()
	if err != nil {
		return err
	}

	// -Djava.apps.version=51 -Djava.apps.prog=financial-account-server
	for _, v := range args {
		parts := strings.Split(v, "=")
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "-Djava.apps.version":
			ii.Version = parts[1]
		case "-Djava.apps.prog":
			v := parts[1]
			idx := strings.LastIndex(parts[1], "-")
			if idx == -1 {
				continue
			}

			dn := fmt.Sprintf("%v%v%v", v[:idx], fieldSperator, v[idx+1:])
			ii.DeployName = types.DeployName(dn)
		case "-Dinstance.sequence":
			v := parts[1]
			ii.NodeName = v
		}
	}

	if needGetListenPort(ii.DeployName) {
		ii.ServiceType = types.ServiceService
		go func(jin *InstanceInfo) {
			ctx := context.Background()
			ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			tick := time.NewTimer(2 * time.Second)
			defer tick.Stop()
			for {
				select {
				case <-tick.C:
					addrs, err := ps.ListenPortsOfPid(jin.Pid)
					if err != nil {
						glog.Warningf("get listening port of %v: %v", jin.Pid, err)
					}
					if len(addrs) > 0 {
						glog.V(10).Infof("process %v listening: %v", jin.Pid, addrs)
						jin.Listening = addrs
						jin.ServiceType = types.ServiceService
						return
					}
				case <-ctx.Done():
					glog.V(11).Infof("not find listen port for process %v", jin.Pid)
					return
				}
			}
		}(ii)
	}

	return nil
}

// SetGeneralInstance update embeded general instance
func (ii *InstanceInfo) SetGeneralInstance(i *types.Instance) {
	ii.Instance = i
	i.Private = ii
}

func needGetListenPort(dn types.DeployName) bool {
	return false
}

func init() {
	err := ps.Register(Type, ps.PidType{
		Typ:  ps.TypPattern,
		Args: "D",
	})
	if err != nil {
		glog.Fatal(err)
	}
}
