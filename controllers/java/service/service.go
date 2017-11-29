package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"we.com/dolphin/controllers/java/router"
	"we.com/dolphin/controllers/java/zk"
	ctypes "we.com/dolphin/controllers/types"
	"we.com/dolphin/report"
	"we.com/dolphin/report/metric"
	"we.com/dolphin/types"
	"we.com/dolphin/types/ins/java"
	"we.com/jiabiao/common/alert"
	"we.com/jiabiao/common/probe"
	pjava "we.com/jiabiao/common/probe/java"
)

type conditionType string

var (
	ctVersion        = "version"
	ctInstanceNum    = "instance num"
	ctInstanceStatus = "instance status"
)

type condition struct {
	Since time.Time
	Times int
	Type  conditionType
	Msg   string
}

type apiVersion string
type service struct {
	Stage      types.Stage       `json:"stage,omitempty"`
	Name       types.DeployName  `json:"name,omitempty"`
	Types      types.ServiceType `json:"types,omitempty"`
	APIVersion apiVersion        `json:"apiVersion,omitempty"`

	Route         *router.RouteCfg `json:"route,omitempty"`
	RouteVersion  string           `json:"routeVersion,omitempty"`
	LatestVersion string           `json:"latestVersion,omitempty"`
	LastVersion   string           `json:"lastVersion,omitempty"`
	ExpectVersion string           `json:"expectVersion,omitempty"`
	FailRatio     failRatio

	ExpectInstance int                         `json:"expectInstance,omitempty"`
	Conditions     map[conditionType]condition `json:"conditions,omitempty"`
	esbFailRatio   map[string]*failRatio
	instances      []*types.Instance
}

type esb struct {
	Service service
	Host    string
	Port    string
}

type failRatio struct {
	Count int
	AVG1  float64
	AVG5  float64
	AVG15 float64
}

// Manager java service  checker
type Manager interface {
}

type manager struct {
	interval      time.Duration
	stage         types.Stage
	lock          sync.RWMutex
	provider      java.ProbeInterfaceProvider
	esbs          map[apiVersion][]*esb
	insInfor      ctypes.InstanceInfor
	zkManager     zk.Manager
	services      map[types.DeployName]*service
	mchan         chan metric.Metric
	stopC         chan struct{}
	inflluxClient *report.InfluxDB
}

/*
	需要检测的异常：
		1. 路由版本与实例版本不一致 <不一定是问题>
		2. zk上节点与实际在跑的实例不一致
		3. 服务拨测异常

	需要上报的信息：
		1. 路由版本
		2. zk上不同版本的个数
		3. zk上实例的个数
		4. 实际在跑的实例个数
		5. zk实例的版本，怎么处理多个版本的情况
*/

// NewManager create a new manager
func NewManager(stage types.Stage, diPV java.ProbeInterfaceProvider, info ctypes.InstanceInfor,
	zk zk.Manager, reporter *report.InfluxDB) (Manager, error) {
	if diPV == nil {
		return nil, errors.Errorf("controler: java service checker, javaprobeinterfaceProvider cannot be nil")
	}

	if info == nil {
		return nil, errors.New("controler: java service checker, instanceInfor cannot be nil")
	}

	if zk == nil {
		return nil, errors.New("controler: java service checker, zk.Manager cannot be nil")
	}

	if reporter == nil {
		return nil, errors.New("controler: java service checker, influxdb client cannot be nil")
	}

	ret := manager{
		interval:      5 * time.Second,
		stage:         stage,
		provider:      diPV,
		esbs:          map[apiVersion][]*esb{},
		insInfor:      info,
		zkManager:     zk,
		services:      map[types.DeployName]*service{},
		mchan:         make(chan metric.Metric, 200),
		stopC:         make(chan struct{}),
		inflluxClient: reporter,
	}

	return &ret, nil
}

func getRouteVersion(cfg *router.RouteCfg) string {
	if cfg == nil {
		return ""
	}

	for _, v := range cfg.RouteItems {
		if v.Dst.Key == "version" {
			if len(v.Dst.Value) >= 1 {
				return v.Dst.Value[0]
			}
			return ""
		}
	}

	return ""
}

func (m *manager) checkHostRunningAndZKinstances() error {
	// 获取zk上所有的java服务
	names := m.zkManager.ListDeployment()
	var merr *multierror.Error

	var alerts []alert.Message

	for _, v := range names {
		ss, err := m.zkManager.GetInstanceList(v)
		if err != nil {
			merr = multierror.Append(merr, err)
			continue
		}

		insMap := m.insInfor.RunningInstance(types.DeployKey(fmt.Sprintf("java/%v", v)))

		if len(ss) != len(insMap) {
			zkhosts := []string{}
			actualHosts := []string{}
			for _, i := range ss {
				zkhosts = append(zkhosts, i.Host)
				sort.StringSlice(zkhosts).Sort()
			}
			for _, i := range insMap {
				actualHosts = append(actualHosts, i.IP)
				sort.StringSlice(actualHosts).Sort()
			}

			parts := strings.Split(string(v), ":")
			alerts = append(alerts, alert.Message{
				Labels: map[string]string{
					"proj": parts[0],
					"env":  m.stage.String(),
					"from": "dolphin",
					"why":  "zk实例不一致",
				},
				Annotations: map[string]string{
					"time":       time.Now().Local().Format(time.Kitchen),
					"msg":        fmt.Sprintf("%v zk上节点个数：%v, 实际的实例数: %v", v, len(ss), len(insMap)),
					"zkhosts":    strings.Join(zkhosts, ", "),
					"actualHost": strings.Join(actualHosts, ", "),
				},
			})
		}

	}

	err := merr.ErrorOrNil()
	if err != nil {
		alerts = append(alerts, alert.Message{
			Labels: map[string]string{
				"env":  m.stage.String(),
				"from": "dolphin",
				"why":  "check zk实例异常",
			},
			Annotations: map[string]string{
				"time": time.Now().Local().Format(time.Kitchen),
				"msg":  err.Error(),
			},
		})
	}

	if len(alerts) > 0 {
		go alert.SendAlerts(alerts...)
	}

	return err
}

func (m *manager) checkZKInstanceNum() error {

	return nil
}

func (m *manager) checkZKVersion() error {
	names := m.zkManager.ListDeployment()
	var merr *multierror.Error

	var alerts []alert.Message

	for _, v := range names {
		ss, err := m.zkManager.GetInstanceList(v)
		if err != nil {
			merr = multierror.Append(merr, err)
			continue
		}

		if len(ss) == 0 {
			continue
		}

		rcfg, err := m.zkManager.GetRouteConfig(v)
		if err != nil {
			merr = multierror.Append(merr, errors.WithMessage(err, fmt.Sprintf("get route config: %v ", v)))
			continue
		}

		ver := getRouteVersion(rcfg)
		if ver == "" {
			continue
		}

		msg := ""
		for _, v := range ss {
			if v.Version != ver {
				msg = msg + fmt.Sprintf("%v:%v\n", v.NodeName, v.Version)
			}
		}
		if len(msg) > 0 {
			parts := strings.Split(string(v), ":")
			alerts = append(alerts, alert.Message{
				Labels: map[string]string{
					"proj": parts[0],
					"env":  m.stage.String(),
					"from": "dolphin",
					"why":  "版本不一致",
				},
				Annotations: map[string]string{
					"time": time.Now().Local().Format(time.Kitchen),
					"msg":  fmt.Sprintf("当前路由版本为：%v, 不一致的实例\n%v", ver, msg),
				},
			})
		}

	}
	return nil
}

func (m *manager) load() error {
	// 获取zk上所有的java服务
	names := m.zkManager.ListDeployment()
	var merr *multierror.Error
	for _, v := range names {
		ss, err := m.zkManager.GetInstanceList(v)
		if err != nil {
			merr = multierror.Append(merr, err)
			continue
		}

		if len(ss) == 0 {
			continue
		}
		s := ss[0]

		typ := types.ServiceDaemon
		if s.Type == 1 || s.Port > 0 {
			typ = types.ServiceService
		}

		rcfg, _ := m.zkManager.GetRouteConfig(v)
		insMap := m.insInfor.RunningInstance(types.DeployKey(fmt.Sprintf("java/%v", v)))

		ins := make([]*types.Instance, 0, len(insMap))
		for _, v := range insMap {
			ins = append(ins, v)
		}

		serv := service{
			Stage:          m.stage,
			Name:           v,
			Types:          typ,
			APIVersion:     apiVersion(s.APIVersion),
			Route:          rcfg,
			ExpectInstance: 0,
			Conditions:     map[conditionType]condition{},
			esbFailRatio:   map[string]*failRatio{},
			instances:      ins,
		}

		m.services[v] = &serv

	}

	return nil
}

func (m *manager) lg(name types.DeployName, esb *esb) (probe.LoadGenerator, error) {
	if len(name) == 0 || esb == nil {
		return nil, nil
	}

	ifs := m.provider.GetProbeInterfaces(name)
	if ifs == nil {
		return nil, nil
	}

	return func() interface{} {
		url := fmt.Sprintf("http://%v:%v", esb.Host, esb.Port)

		ret := make([]*pjava.Args, 0, len(ifs))
		for _, di := range ifs {
			args := pjava.Args{
				Name:    di.Name,
				Data:    strings.NewReader(di.Data),
				URL:     url,
				Headers: di.Headers,
			}
			ret = append(ret, &args)
		}
		return ret
	}, nil
}

func (m *manager) getService(name types.DeployName) *service {
	m.lock.RLock()
	defer m.lock.RUnlock()
	ret := m.services[name]
	return ret
}

func (m *manager) getEsbs(ver apiVersion) []*esb {
	m.lock.RLock()
	defer m.lock.RUnlock()
	ret := m.esbs[ver]
	return ret
}

type probeResult struct {
	name   types.DeployName
	err    error
	msg    string
	result probe.Result
}

type interfaceIterator struct {
	idx        int
	interfaces []*java.ProbeInterface
}

func (iter *interfaceIterator) Next() *java.ProbeInterface {
	if iter == nil || len(iter.interfaces) == 0 {
		return nil
	}

	iter.idx++

	if iter.idx >= len(iter.interfaces) {
		iter.idx = 0
	}
	return iter.interfaces[iter.idx]
}

func toIterator(ifsMap java.ProbeInterfaces) *interfaceIterator {
	if len(ifsMap) == 0 {
		return nil
	}
	ifs := make([]*java.ProbeInterface, 0, len(ifsMap))
	for _, v := range ifsMap {
		ifs = append(ifs, v)
	}
	return &interfaceIterator{
		idx:        len(ifs),
		interfaces: ifs,
	}
}

func (m *manager) StartProbe(ctx context.Context, name types.DeployName, ch chan<- probeResult) {
	s := m.getService(name)
	if s == nil {
		ch <- probeResult{
			name:   name,
			err:    errors.Errorf("java deployment %v not exist", name),
			result: probe.Unknown,
		}
		return
	}

	esbs := m.getEsbs(s.APIVersion)
	if len(esbs) == 0 {
		ch <- probeResult{
			name:   name,
			err:    errors.Errorf("no esb for apiVersion %v", s.APIVersion),
			result: probe.Unknown,
		}
		return
	}

	ifs := m.provider.GetProbeInterfaces(name)
	iter := toIterator(ifs)
	lg := func(e *esb, di *java.ProbeInterface) func() interface{} {
		return func() interface{} {
			url := fmt.Sprintf("http://%v:%v", e.Host, e.Port)
			ret := make([]*pjava.Args, 1)

			ret[0] = &pjava.Args{
				Name:    di.Name,
				Data:    strings.NewReader(di.Data),
				URL:     url,
				Headers: di.Headers,
			}

			return ret
		}
	}

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	updateTimer := time.NewTimer(5 * time.Minute)

	idx := 0
	for {
		select {
		case <-updateTimer.C:
			esbs = m.getEsbs(s.APIVersion)
			if len(esbs) == 0 {
				ch <- probeResult{
					name:   name,
					err:    errors.Errorf("no esb for apiVersion %v", s.APIVersion),
					result: probe.Unknown,
				}
			}
			ifs = m.provider.GetProbeInterfaces(name)
			if ifs == nil {
				ch <- probeResult{
					name:   name,
					err:    errors.Errorf("no dial interface configed to prbe"),
					result: probe.Unknown,
				}
			}
			iter = toIterator(ifs)
		case <-ticker.C:
			if len(esbs) == 0 {
				continue
			}
			iface := iter.Next()
			if iface == nil {
				continue
			}
			if idx > len(esbs) {
				idx = 0
			}

			e := esbs[idx]
			l := lg(e, iface)

			ret, msg, err := pjava.Probe(l)
			if err != nil {
				glog.Errorf("java probe: %v err: %v, msg: %v", name, err.Error(), msg)
			}

			s.FailRatio.update(ret)

			url := fmt.Sprintf("%v:%v", e.Host, e.Port)
			ef := s.esbFailRatio[url]
			if ef == nil {
				ef = &failRatio{}
				s.esbFailRatio[url] = ef
			}
			ef.update(ret)
			label, field := m.newLabelsAndFields(name, url, ef)
			label["version"] = string(s.APIVersion)
			field["numInstances"] = len(s.instances)
			field["numVersions"] = s.getNumVersion()
			mtr, _ := metric.New(measurement, label, field, time.Now())
			m.mchan <- mtr
			e.Service.FailRatio.update(ret)

		case <-ctx.Done():
			return
		case <-m.stopC:
			return
		}
	}
}

func (m *manager) newLabelsAndFields(name types.DeployName, esb string, fr *failRatio) (map[string]string, map[string]interface{}) {
	labesl := map[string]string{
		"env":     m.stage.String(),
		"service": string(name),
		"esb":     "all",
	}

	fields := map[string]interface{}{
		"failRatio1":  fr.AVG1,
		"failRatio5":  fr.AVG5,
		"failRatio15": fr.AVG15,
		"probecount":  fr.Count,
	}

	return labesl, fields
}

const (
	ns          = "monitor"
	measurement = "java_service"
)

func (s *service) getNumVersion() int {
	verMap := map[string]int{}
	for _, v := range s.instances {
		verMap[v.Version] = 0
	}
	return len(verMap)
}

func (m *manager) check() {
	javaMap := map[types.DeployKey]struct{}{}
	// 获取java的deployname列表
	keys := m.insInfor.ListDeploykeys()
	for _, v := range keys {
		if !strings.HasPrefix(string(v), "java/") {
			continue
		}
		javaMap[v] = struct{}{}
	}

	for k := range javaMap {
		name := types.DeployName(strings.TrimPrefix(string(k), "java/"))

		m.insInfor.RunningInstance(k)

		m.zkManager.GetRouteConfig(name)

	}

	// 对于每个项目（deployname),  查询hostconfig 配置， running instances, zk nodes,
	// version config, 等信息

	// 获取java 的deployname列表，

}

func (m *manager) report() {
	go func() {
		for {
			select {
			case <-m.stopC:
				return
			case mtr := <-m.mchan:
				if m.inflluxClient != nil {
					m.inflluxClient.Write([]metric.Metric{mtr})
				}
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case now := <-ticker.C:
				if m.inflluxClient == nil {
					break
				}
				metrics := make([]metric.Metric, len(m.services))
				m.lock.RLock()
				for n, v := range m.services {
					labels, fields := m.newLabelsAndFields(n, "all", &v.FailRatio)
					labels["version"] = string(v.APIVersion)
					fields["numInstances"] = len(v.instances)
					fields["numVersions"] = v.getNumVersion()
					mtr, _ := metric.New(measurement, labels, fields, now)
					metrics = append(metrics, mtr)
				}
				m.lock.RUnlock()

				m.inflluxClient.Write(metrics)
			case <-m.stopC:
				return
			}
		}
	}()
}

func (fr *failRatio) update(result probe.Result) {
	fr.Count++
	r := 0.0
	if result == probe.Failure {
		r = 1.0
	} else if result == probe.Warning {
		r = 0.5
	}

	fr.AVG1 = calFailRatio(fr.AVG1, exp1, r)
	fr.AVG5 = calFailRatio(fr.AVG5, exp5, r)
	fr.AVG15 = calFailRatio(fr.AVG15, exp15, r)
}

const (
	fshift = 11
	fixed1 = 1 << fshift
	exp1   = 1884
	exp5   = 2014
	exp15  = 2037
)

// http://www.linuxjournal.com/article/9001?page=0,1
func calFailRatio(old, exp, n float64) float64 {
	old *= exp
	old += n * (fixed1 - exp)
	old = old / fixed1
	return old
}
