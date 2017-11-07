package types

import (
	"fmt"
	"time"

	"github.com/pborman/uuid"
	"github.com/pkg/errors"
	"we.com/jiabiao/common/probe"
)

// LifeCycle  life cycle of  instance
type LifeCycle string

const (
	// LCStarting not ready for service
	LCStarting LifeCycle = "starting"
	// LCRunning  instance is up
	LCRunning LifeCycle = "running"
	// LCStopping instance is schedualed stopping or is stopping
	LCStopping LifeCycle = "stopping"
	// LCStopped instance already stopped
	LCStopped LifeCycle = "stopped"
)

// ServiceType show the instance is running, a daemon, a service, a onetime script  etc.
type ServiceType string

const (
	// ServiceUnknown unknows service type
	ServiceUnknown ServiceType = "unknown"
	// ServiceService  service type, which provide a service, others can use
	ServiceService ServiceType = "service"
	// ServiceDaemon  long run daemon process
	ServiceDaemon ServiceType = "daemon"
	// ServiceScript  short run daemon  process, do not need to restart
	ServiceScript ServiceType = "script"
)

// InstanceID instance id
type InstanceID UUID

// Addr listening addr
type Addr struct {
	IP   string
	Port int
}

type ConditionType string

const (
	HighMem        ConditionType = "highMemory"
	HighCPU        ConditionType = "highCPU"
	ProbError      ConditionType = "probeError"
	HighDiskIO     ConditionType = "highDiskIO"
	HighThreads    ConditionType = "highThreads"
	ProcessStopped ConditionType = "processStopped"
	ProbeCondition ConditionType = "probe"
)

type Condition struct {
	Type    ConditionType `json:"type,omitempty"`
	Message string        `json:"message,omitempty"`
}

// InstanceResUsage instance resource usage
type InstanceResUsage struct {
	Memory         uint64  `json:"memory,omitempty"`
	CPUTotal       float64 `json:"cpuTotal,omitempty"`
	CPUPercent     float64 `json:"cpuPercent,omitempty"`
	Threads        int     `json:"threads,omitempty"`
	DiskBytesRead  uint64  `json:"diskBytesRead,omitempty"`
	DiskBytesWrite uint64  `json:"diskBytesWrite,omitempty"`
}

// InstanceStatus instance prob status
type InstanceStatus string

const (
	// InstanceWarning  prob  status warning
	InstanceWarning InstanceStatus = "warning"
	// InstanceError prob error
	InstanceError InstanceStatus = "error"
	// InstanceSuccess  prob ok
	InstanceSuccess InstanceStatus = "success"
	// InstanceUnknown unknwon prob status
	InstanceUnknown InstanceStatus = "unknown"
)

// Instance common  instance info
type Instance struct {
	// identity
	ProjecType ProjectType `json:"projectType,omitempty"`
	ID         InstanceID  `json:"id,omitempty"`
	Pid        int         `json:"pid,omitempty"`
	DeployName DeployName  `json:"clusterName,omitempty"`

	// Host info
	User   string `json:"user,omitempty"`
	HostID HostID `json:"hostID,omitempty"`
	Host   string `json:"host,omitempty"`
	IP     string `json:"ip,omitempty"`
	Stage  Stage  `json:"stage,omitempty"`

	// Instance info
	ServiceType ServiceType `json:"serviceType,omitempty"`
	Version     string      `json:"version,omitempty"`
	StartTime   time.Time   `json:"startTime,omitempty"`
	StopTime    time.Time   `json:"stopTime,omitempty"`
	UpdateTime  time.Time   `json:"updateTime,omitempty"`
	LifeCycle   LifeCycle   `json:"lifeCycle,omitempty"`

	Listening []Addr `json:"listening,omitempyt,omitempty"`

	// status info
	Status     InstanceStatus    `json:"status,omitempty"`
	Conditions []*Condition      `json:"conditions,omitempty"` // error
	Events     []*Condition      `json:"events,omitempty"`     // warning
	ResUsage   *InstanceResUsage `json:"resUsage,omitempty"`

	// addional info  of specific type
	Private interface{} `json:"private,omitempyt,omitempty"`
}

// DeployKey deploy key of this instance
func (ins *Instance) DeployKey() DeployKey {

	return DeployKey(fmt.Sprintf("%v/%v", ins.ProjecType, ins.DeployName))
}

// InstanceInfor instance info
type InstanceInfor interface {
	GetExe() string
	GetEnvMap() map[string]string
	GetArgs() string
}

// InstanceParseFunc function to parse instances info
type InstanceParseFunc func(ins *Instance, ii InstanceInfor) error

// InstanceParser given an pid  parse instanse info  from it command line, env, etc.
type InstanceParser interface {
	//Parse(pid int) (*Instance, error)
	Parse(ins *Instance, envMap map[string]string, cmdline string) error
}

// Prober an instance prober
type Prober interface {
	Probe(ins *Instance) (probe.Result, error)
}

// StopCmdArgs cmd args to send to ctrl script to stop this instance
func (ins *Instance) StopCmdArgs() [3]string {
	ret := [3]string{}
	ret[0] = string(ins.ProjecType)
	ret[1] = string(ins.DeployName)
	ret[2] = string(ins.Pid)

	return ret
}

// UnmarshalJSON disable  UnmarshalJSON
func (ins *Instance) UnmarshalJSON(data []byte) error {
	return errors.New("instance should not Unmarshal")
}

// NewInstanceID create an new instanceID before instance start
func NewInstanceID() InstanceID {
	uid := uuid.New()
	return InstanceID(uid)
}
