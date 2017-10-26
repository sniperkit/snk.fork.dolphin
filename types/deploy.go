package types

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"we.com/jiabiao/common/labels"
)

const (
	// all deployments are placed under this folder
	// into order to ensure all host deploys to the same folder
	// this value  do not allowed to modify
	deployBaseDir = "/data/deploy/"
)

// RestartType when to restart
type RestartType string

// RestartPolicy action taken when  process exits
// when Type is Onetime, until should also be set, to indicat agent that it should not start it again on  agent restart
type RestartPolicy struct {
	Type  RestartType `json:"type,omitempty"`
	Until time.Time   `json:"value,omitempty"`
}

// MarshalJSON json.Marshal interface
func (rp RestartPolicy) MarshalJSON() ([]byte, error) {
	if rp.Type == OneTime || rp.Type == Always {
		return json.Marshal(rp.Type)
	}

	if rp.Type == "" && rp.Until.IsZero() {
		return json.Marshal(Always)
	}

	type p RestartPolicy

	return json.Marshal(p(rp))
}

// UnmarshalJSON  json.Unmarshal  interface
func (rp *RestartPolicy) UnmarshalJSON(data []byte) error {
	var s RestartType
	err := json.Unmarshal(data, &s)

	if err == nil {
		if string(s) == "" {
			s = Always
		}

		if s != OneTime && s != Always {
			return errors.Errorf("unknown restart policy: %v", s)
		}

		rp.Type = s
		return nil
	}

	type p struct {
		Type  RestartType `json:"type,omitempty"`
		Value interface{} `json:"value,omitempty"`
	}

	t := p{}
	if err := json.Unmarshal(data, &t); err != nil {
		return err
	}

	if t.Type != Until {
		return errors.Errorf("unknown restart policy: %v", t.Type)
	}

	switch a := t.Value.(type) {
	case string:
		var format string
		switch len(a) {
		case 5:
			format = "15:04"
		case 10:
			format = "2006-01-02"
		case 16:
			format = "2006-01-02 15:04"
		case len(time.RFC3339):
			format = time.RFC3339
		default:
			return errors.Errorf("unknown date format for restart type %v, %v", t.Type, a)
		}

		d, err := time.Parse(format, a)
		if err != nil {
			return err
		}
		if format != time.RFC3339 {
			d = d.Add(8 * time.Hour)
		}

		rp.Type = t.Type
		rp.Until = d
		return nil

	case float64:
		rp.Until = time.Unix(int64(a), 0)
		rp.Type = t.Type
	default:
		return errors.Errorf("unknown value for restart policy of %v, %v", t.Type, a)
	}

	return nil
}

// Selector host selector
type Selector map[string]string

var (
	labelValueRegexp = regexp.MustCompile(`^(=|!=|in |notin )?.*`)
)

// ToSelector check if selector is valid, parse it to a labels.Selector
func (s Selector) ToSelector() (labels.Selector, error) {
	raw := ""
	for k, v := range s {
		if k == "" {
			return nil, errors.New("dc: selector key cannot be empty")
		}

		parts := labelValueRegexp.FindStringSubmatch(v)
		if len(parts) != 2 {
			return nil, errors.New("dc: selector value format error")
		}

		if len(parts[0]) == 0 {
			raw = raw + k + ","
		} else if len(parts[1]) == 0 {
			raw = raw + k + "=" + v + ","
		}
	}

	raw = strings.TrimSuffix(raw, ",")
	return labels.Parse(raw)
}

var (
	// OneTime 执行一次,  程序退出后(正常或异常)， 什么都不干
	OneTime RestartType = "onetime"
	// Always 总是重启， 程序退出后自动拉起
	Always RestartType = "always"
	// Until 在一个时间之前， 总是重启， 之后停掉
	// 对于长时间运行的程序， 在停掉前会先告警
	Until RestartType = "until"
)

// DeployPolicy how to deploy the image
// this shoud not changed after deploy take out
// if really need to change: here is the solution:
// 		first find host that has not deplyment
// 		schedual new deployments to those hosts
// 		then delete old deployment, and clean the deployed dirs
type DeployPolicy string

const (
	// Inplace default for java applications
	Inplace DeployPolicy = "inplace"
	// ABWorld default for php or web applicatsion,
	// this is the prefered choice
	ABWorld DeployPolicy = "abworld"
	// Versioned every version has its own folder
	Versioned DeployPolicy = "versioned"
)

// DeployName is uniq within a projecttype
type DeployName UUID

// DeployKey is universally uniq
type DeployKey UUID

// DeployConfig config  how an project should be deployed
type DeployConfig struct {
	Type                ProjectType `json:"projectType,omitempty"`
	Name                DeployName  `json:"name,omitempty"` // cluster unique defines an project of type Type
	NumOfInstance       int         `json:"numOfInstance,omitempty"`
	ServiceType         ServiceType `json:"serviceType,omitempty"`
	Stage               Stage       `json:"stage,omitempty"`
	MaxInstancesPerNode int         `json:"maxInstancesPerNode,omitempty"`

	Image     *Image `json:"image,omitempty"`
	DeployDir string `json:"deployDir,omitempty"`
	// Values is config map used to render the config files
	Values       map[string]interface{} `json:"values,omitempty"`
	DeployPolicy DeployPolicy           `json:"deployPolicy,omitempty"`

	// these fields used to select which hosts can start this project
	Selector         Selector `json:"selector,omitempty"`
	selector         labels.Selector
	ResourceQuota    *ResourceSize   `json:"resourceQuota,omitempty"`
	ResourceRequired *DeployResource `json:"resourceRequired,omitempty"`

	// RestartPolicy action taken, when process exits,
	// default always restart
	RestartPolicy *RestartPolicy `json:"restartPolicy,omitempty"`
	// UpdatePolicy how to update running  process to new version
	// for service:  default  is NewDeploy
	// for deamon:  default is  rollingupdate
	// for onetime script: not used, we would update onetime running scripts
	UpdatePolicy *UpdateOption `json:"updatePolicy,omitempty"`
}

// GetDefaultUpdateOption  return defalut UpdateOption for serviceType
func GetDefaultUpdateOption(typ ServiceType) *UpdateOption {
	var upo *UpdateOption
	switch typ {
	case ServiceService:
		upo = &UpdateOption{
			Policy:  NewDeploy,
			Step:    30 * time.Second,
			Timeout: 5 * time.Minute,
		}
	case ServiceDaemon, ServiceUnknown:
		upo = &UpdateOption{
			Policy:  RollingUpdate,
			Step:    30 * time.Second,
			Timeout: 5 * time.Minute,
		}
	case ServiceScript:
		upo = &UpdateOption{
			Policy:  RollingUpdate,
			Step:    30 * time.Second,
			Timeout: 5 * time.Minute,
		}
	default:
		upo = nil
	}

	return upo
}

// GetDeployDir deploy dir
// for ABWorld:  deployed to GetDeployDir()/[A|B]/
// for versioned:  deploy to GetDeployDir()/version/
// for Inplace: deployed to GetDeployDir()/
func (dc *DeployConfig) GetDeployDir() string {
	if strings.HasPrefix(dc.DeployDir, deployBaseDir) {
		return dc.DeployDir
	}

	return filepath.Join(deployBaseDir, dc.DeployDir)
}

// GetSelector get host selector
func (dc *DeployConfig) GetSelector() labels.Selector {
	return dc.selector
}

// Key get deployKey of this config
func (dc *DeployConfig) Key() DeployKey {
	return DeployKey(fmt.Sprintf("%v/%v", dc.Type, dc.Name))
}

// ParseDeployKey parse deploy key to projectType
func ParseDeployKey(key DeployKey) (ProjectType, string, error) {
	idx := strings.Index(string(key), "/")
	if idx == -1 || idx+1 == len(key) {
		return ProjectType(""), "", errors.New("invalid deploy key")
	}

	s := string(key)
	pt := ProjectType(s[:idx])
	name := s[idx+1:]
	return pt, name, nil
}

type DeployVer string
type DeploySpec struct {
	Info map[DeployVer]int `json:"info,omitempty"`
}

type HostDeployment struct {
	HostID      HostID                   `json:"hostID,omitempty"`
	DeploySpecs map[DeployKey]DeploySpec `json:"deployments,omitempty"`
}

type Phase string

const (
	PhaseWaiting    Phase = "wating"
	PhasePullImage  Phase = "pull image"
	PhasePrepare    Phase = "prepare deployment"
	PhaseApply      Phase = "apply Patch"
	PhaseRestarting Phase = "restarting service"
	PhaseDone       Phase = "done"
)

type ProcessStatus string

const (
	PsStarting ProcessStatus = "starting"
	PsStopping ProcessStatus = "stopping"
	PsStarted  ProcessStatus = "started"
	PsStopped  ProcessStatus = "stopped"
)

type DeployStatus struct {
	DeployPhase   Phase         `json:"deployPhase,omitempty"`
	ProcessStatus ProcessStatus `json:"processStatus,omitempty"`
}

// Deployment reprents an actual deploy on a host
type Deployment struct {
	Type         ProjectType  `json:"type,omitempty"`
	Name         DeployName   `json:"cluster,omitempty"`
	Stage        Stage        `json:"stage,omitempty"`
	Host         HostID       `json:"host,omitempty"`
	HostName     HostName     `json:"hostname,omitempty"`
	Status       DeployStatus `json:"status,omitempty"`
	Version      Version      `json:"version,omitempty"`
	RestartCount int          `json:"restartCount,omitempty"`
	DeployTime   time.Time    `json:"deployTime,omitempty"`
	UpdateTime   time.Time    `json:"updateTime,omitempty"`
}

// UpdatePolicyName how to update
type UpdatePolicyName string

const (
	// RollingUpdate  update one instance a time, util
	// all instance are up to date
	RollingUpdate UpdatePolicyName = "rollingUpdate"
	// NewDeploy Leave old instance untouched, need be stopped some time later
	// schedual a new deployment according to deploy config
	NewDeploy UpdatePolicyName = "deployNew"
	// MixedUpdate mix RollingUpdate and NewDeploy:
	// first create half the num (round to ceiling) of deployment,
	// then rollingupdate the other half
	MixedUpdate UpdatePolicyName = "mixed"
)

// UpdateOption  update options
type UpdateOption struct {
	Policy     UpdatePolicyName
	Step       time.Duration
	NewPercent float64
	Timeout    time.Duration
}

// Validate  check updateOption is valid
func (uo *UpdateOption) Validate() error {
	if uo == nil {
		return nil
	}
	switch uo.Policy {
	case RollingUpdate, NewDeploy:
		if uo.Step > uo.Timeout {
			return errors.New("update policy config: step should not greater then  timout")
		}
	case MixedUpdate:
		if uo.NewPercent > 1 || uo.NewPercent < 0 {
			return errors.New("udate policy config: mixed, new ratio invalid, must between [0, 100]")
		}
	default:
		return errors.Errorf("unknonwn update policy, valids are %v, %v, %v", RollingUpdate, NewDeploy, MixedUpdate)
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler interface
func (uo *UpdateOption) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}

	parts := strings.Split(s, ":")

	var tos string
	if len(parts) > 3 {
		return errors.New("update policy config: format error")
	} else if len(parts) == 3 {
		tos = parts[2]
	} else if len(parts) == 2 {
		tos = parts[1]
	}

	p := UpdatePolicyName(parts[0])
	var timeout = 5 * time.Minute

	switch p {
	case RollingUpdate, NewDeploy:
		var step = 30 * time.Second
		if len(parts) == 3 {
			if step, err = time.ParseDuration(parts[1]); err != nil {
				return err
			}
		}
		uo.Step = step
	case MixedUpdate:
		var p = 0.5
		if len(parts) == 3 {
			p, err = strconv.ParseFloat(parts[1], 64)
			if err != nil {
				return err
			}
			p = p / 100
		}
		uo.NewPercent = p
	}

	if len(tos) > 0 {
		if timeout, err = time.ParseDuration(tos); err != nil {
			return err
		}
	}
	uo.Policy = p
	uo.Timeout = timeout

	return uo.Validate()
}

// MarshalJSON implements json.Marshaler interface
func (uo UpdateOption) MarshalJSON() (data []byte, err error) {
	if err := uo.Validate(); err != nil {
		return nil, err
	}
	ret := ""
	switch uo.Policy {
	case NewDeploy, RollingUpdate:
		ret = fmt.Sprintf("%v:%v:%v", uo.Policy, uo.Step.String(), uo.Timeout.String())
	case MixedUpdate:
		ret = fmt.Sprintf("%v:%v:%v", uo.Policy, int(100*uo.NewPercent), uo.Timeout.String())
	default:
		return nil, errors.New("update policy config: invalid policy name")
	}

	return json.Marshal(ret)
}
