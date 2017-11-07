package java

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	"we.com/dolphin/types"
	"we.com/dolphin/types/ins/registry"
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

	RouteInfo string `json:"routeInfo,omitempty"`

	// A pointer to outer common instance info
	ins *types.Instance
}

func fillInstanceInfo(ins *types.Instance, insInfor types.InstanceInfor) error {
	if ins == nil {
		return nil
	}

	ii := &InstanceInfo{
		ins: ins,
	}
	ins.Private = ii

	ins.ProjecType = Type

	cmdline := insInfor.GetArgs()
	args := strings.Split(cmdline, " ")

	for _, v := range args {
		parts := strings.Split(v, "=")
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "-Djava.apps.version":
			ins.Version = parts[1]
		case "-Djava.apps.prog":
			v := parts[1]
			idx := strings.LastIndex(parts[1], "-")
			if idx == -1 {
				continue
			}
			dn := fmt.Sprintf("%v%v%v", v[:idx], fieldSperator, v[idx+1:])
			ins.DeployName = types.DeployName(dn)
		case "-Dinstance.sequence":
			v := parts[1]
			ii.NodeName = v
		}
	}

	return nil
}

func init() {
	typeInfo := registry.TypeInfo{
		Type:    Type,
		Parse:   fillInstanceInfo,
		Prober:  &Prober{},
		Decoder: &decode{},
	}

	err := registry.Register(typeInfo)
	if err != nil {
		glog.Fatal(err)
	}
}
