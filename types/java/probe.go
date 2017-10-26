package java

import (
	"errors"
	"fmt"
	"net/http/httputil"
	"strings"
	"sync"

	multierror "github.com/hashicorp/go-multierror"
	"we.com/dolphin/types"
	"we.com/jiabiao/common/probe"
	"we.com/jiabiao/common/probe/java"
)

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

// GetProbeInterfaces return probeinterface of bin and env
func (p *Prober) getProbeInterfaces(stage types.Stage, key types.DeployName) []*ProbeInterface {
	if p == nil {
		return nil
	}

	p.lock.RLock()

	k := dikey{
		stage: stage,
		dname: key,
	}
	pis, ok := p.dialInterfaces[k]
	p.lock.RUnlock()
	if !ok {
		return nil
	}

	ret := []*ProbeInterface{}
	for _, v := range pis {
		if len(v.Stages) == 0 {
			// copy
			t := *v
			ret = append(ret, &t)
			continue
		}
	}
	return ret
}

type dikey struct {
	stage types.Stage
	dname types.DeployName
}

// Prober probe a java server
type Prober struct {
	dialInterfaces map[dikey]map[string]*ProbeInterface
	lock           sync.RWMutex
}

func (p *Prober) lg(ii *InstanceInfo) (probe.LoadGenerator, error) {
	if len(ii.Listening) == 0 {
		return nil, nil
	}

	if len(ii.Listening) > 1 {
		return nil, errors.New("instance is listening to port, probe which")
	}

	return func() interface{} {
		addr := ii.Listening[0]
		url := fmt.Sprintf("http://%v:%v", addr.IP, addr.Port)
		dis := p.getProbeInterfaces(ii.Stage, ii.DeployName)

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

	ii, ok := ins.Private.(*InstanceInfo)
	if !ok {
		return probe.Unknown, errors.New("not an java instance")
	}

	lg, err := p.lg(ii)
	if err != nil {
		return probe.Unknown, err
	}

	ret, _, err := java.Probe(lg)
	return ret, err
}

// ProxyProbe a reverse proxy server
// which accept an incomming http request, and send it to another server
type ProxyProbe struct {
	httputil.ReverseProxy
}
