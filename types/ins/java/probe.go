package java

import (
	"errors"
	"fmt"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
	"we.com/dolphin/types"
	"we.com/jiabiao/common/probe"
	"we.com/jiabiao/common/probe/java"
)

// ProbeInterfaceProvider from which get can get interface to dial
type ProbeInterfaceProvider interface {
	GetProbeInterfaces(key types.DeployName) ProbeInterfaces
}

var (
	diProvider ProbeInterfaceProvider
)

// ProbeInterfaces probes interfaces of a given deployment
type ProbeInterfaces map[string]*ProbeInterface

// ProbeInterface  a single  probe interface
type ProbeInterface struct {
	Name        string            `json:"name,omitempty"`
	Desc        string            `json:"desc,omitempty"`
	Data        string            `json:"data,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Stages      []string          `json:"env,omitempty"`
	Matches     map[string]string `json:"matches,omitempty"`
	DontMatches map[string]string `json:"dontMatches,omitempty"`
}

// Validate test if a ProbeInterface is valid
func (pi *ProbeInterface) Validate() error {
	var err *multierror.Error

	if pi.Desc == "" {
		terr := fmt.Errorf("interface description cannot be nil")
		err = multierror.Append(err, terr)
	}

	if pi.Data == "" {
		terr := fmt.Errorf("interface data cannot be nil")
		err = multierror.Append(err, terr)
	}

	return err.ErrorOrNil()
}

// Prober probe a java server
type Prober struct {
}

func (p *Prober) lg(ins *types.Instance) (probe.LoadGenerator, error) {
	if diProvider == nil {
		return nil, nil
	}
	ii, ok := ins.Private.(*InstanceInfo)
	if !ok {
		return nil, errors.New("not an java instance")
	}

	if len(ins.Listening) == 0 {
		return nil, nil
	}

	if len(ins.Listening) > 1 {
		return nil, errors.New("instance is listening to port, probe which")
	}

	return func() interface{} {
		addr := ins.Listening[0]
		url := fmt.Sprintf("http://%v:%v", addr.IP, addr.Port)
		dis := diProvider.GetProbeInterfaces(ins.DeployName)

		ret := make([]*java.Args, 0, len(dis))
		for _, di := range dis {
			args := java.Args{
				Name:    di.Name,
				Cluster: string(ii.NodeName),
				Data:    strings.NewReader(di.Data),
				URL:     url,
				Headers: di.Headers,
			}
			ret = append(ret, &args)
		}
		return ret
	}, nil
}

// Probe probe backend java server
func (p *Prober) Probe(ins *types.Instance) (probe.Result, error) {
	if ins == nil {
		return probe.Success, nil
	}

	lg, err := p.lg(ins)
	if err != nil {
		return probe.Unknown, err
	}

	if lg == nil {
		return probe.Success, nil
	}

	ret, _, err := java.Probe(lg)
	return ret, err
}
