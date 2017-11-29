package zk

import (
	"context"
	"fmt"
	"path"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"we.com/dolphin/controllers/java/project"
	"we.com/dolphin/controllers/java/router"
	"we.com/dolphin/registry/etcdkey"
	"we.com/dolphin/registry/generic"
	"we.com/dolphin/registry/watch"
	"we.com/dolphin/types"
)

const (
	api2 = router.APIV2
	api4 = router.APIV4
)

type simplePathInfo struct {
	lock  sync.RWMutex
	stopC chan struct{}

	// 以下两个map都只针对api2.0
	serviceMap map[string]string // serviceName --> projectName
	projectMap map[string]string // project name --> service Name
}

func (sp *simplePathInfo) getServiceName(deployName types.DeployName) (string, string, error) {
	parts := strings.Split(string(deployName), ":")
	if len(parts) != 2 {
		return "", "", errors.Errorf("zk: parse java deploy name error: %v", deployName)
	}
	project := parts[0]
	sp.lock.RLock()
	defer sp.lock.RUnlock()
	s := sp.projectMap[project]
	if s == "" {
		return api4, fmt.Sprintf("%v/%v", parts[0], parts[1]), nil
	}
	return api2, s, nil
}

func (sp *simplePathInfo) getProjectName(servicename string) (string, string) {
	sp.lock.RLock()
	defer sp.lock.RUnlock()
	s := sp.serviceMap[servicename]
	if s == "" {
		return api4, servicename
	}
	return api2, s
}

func (sp *simplePathInfo) GetRoutePath(name types.DeployName) (string, error) {
	ver, s, err := sp.getServiceName(name)
	if err != nil {
		return "", err
	}

	if ver == api2 {
		return "/service/" + s, nil
	}

	return fmt.Sprintf("/biz/%v/policy/default/route", s), nil
}

func (sp *simplePathInfo) GetInstancePath(name types.DeployName) (string, error) {
	ver, s, err := sp.getServiceName(name)
	if err != nil {
		return "", err
	}

	if ver == api2 {
		return "/service/" + s + "/", nil
	}

	return fmt.Sprintf("/biz/%v/instance/", s), nil
}

var (
	binRe = regexp.MustCompile(`(_[0-9]{1,3})?$`)
)

// GetDeployName 给定一个zkpath,　解析其deployName,
// 当给定的前两种格式时， 返回的是服务（项目）名，而不deployName
// zkPath需要满足下面的条件：
// 	1.	/service/{serviceName}
// 	2.	/config/{serviceName}[/*]
//	3.	/service/{serviceName}/{instanceID}
// 	4.	/biz/{projectName}/{binName}[/*]
func (sp *simplePathInfo) GetDeployName(zkPath string) (types.DeployName, error) {
	parts := strings.Split(zkPath, "/")

	err := errors.Errorf("zk: GetDeployName, zkpath format not correct, %v", zkPath)

	if len(parts) <= 2 {
		return types.DeployName(""), err
	}

	name := parts[2]
	ver := router.APIV2
	if parts[1] == "biz" {
		if len(parts) < 4 {
			return types.DeployName(""), err
		}
		ver = router.APIV4
		name = name + ":" + parts[3]
		return types.DeployName(name), nil
	}

	ver, s := sp.getProjectName(name)
	if ver != api2 {
		return types.DeployName(""), errors.Errorf("zk: GetDeployName unknown api2.0 service: %v", name)
	}

	if len(parts) == 3 || parts[1] != "service" {
		return types.DeployName(s), nil
	}

	matches := binRe.FindStringSubmatch(parts[3])

	bin := strings.TrimSuffix(parts[3], matches[1])

	return types.DeployName(s + ":" + bin), nil
}

func (sp *simplePathInfo) GetAPIVersion(name types.DeployName) string {
	ver, _, _ := sp.getServiceName(name)
	return ver
}

type zkTyp string

const (
	zkConfig   zkTyp = "config"
	zkRoute    zkTyp = "route"
	zkInstance zkTyp = "instance"
)

// GetEtcdPath zkPath是zk上的绝对路径，目前可以识别的路径格式为
//   /service/{serviceName}, /service/{serviceName}/{instanceName}
//   /biz/{projectName}/{binName}, /bin/{projectName}/{binName}/....
//   /config/{serviceName}/....
func (sp *simplePathInfo) GetEtcdPath(env types.Stage, zkPath string) (zkTyp, string, error) {
	var typ zkTyp
	if len(zkPath) == 0 {
		return typ, "", errors.New("zkpath not allow empty")
	}

	var p string
	var err error
	if strings.HasPrefix(zkPath, "/biz/") {
		typ, p, err = parseZKPathv4(zkPath)
	} else {
		typ, p, err = sp.parseZKPathv2(zkPath)
	}

	if err != nil {
		return typ, "", err
	}

	return typ, path.Join(etcdkey.StageBaseDir(env), p), nil
}

func parseZKPathv4(zkPath string) (zkTyp, string, error) {
	var typ zkTyp
	if !strings.HasPrefix(zkPath, "/biz/") {
		return typ, "", errors.Errorf("invalid it4.0 zkpath, %v", zkPath)
	}

	p := strings.TrimPrefix(zkPath, "/biz/")

	parts := strings.Split(p, "/")

	if len(parts) < 3 {
		return typ, "", errors.Errorf("uncomplete  it4.0 zkPath, %v", zkPath)
	}

	var prefix string
	typStr := parts[2]
	if typStr == "daemon" || typStr == "instance" {
		typ = zkInstance
		prefix = etcdkey.JavaZKRelInstanceDir()
	} else if typStr == "policy" {
		typ = zkRoute
		prefix = etcdkey.JavaZKRelRouteDir()
	} else if typStr == "config" {
		typ = zkConfig
		prefix = etcdkey.JavaZKRelConfigDir()
	}

	if typ == "" {
		return typ, "", errors.Errorf("cannot parse it4.0 zkpath: %v", zkPath)
	}

	service := strings.Join(parts[0:2], ".")

	tmp := make([]string, 0, len(parts)+2)
	tmp = append(tmp, prefix, "4", service)
	tmp = append(tmp, parts[3:]...)

	return typ, path.Join(tmp...), nil
}

func (sp *simplePathInfo) parseZKPathv2(zkPath string) (zkTyp, string, error) {
	var typ zkTyp
	parts := strings.Split(zkPath, "/")

	if len(parts) < 3 {
		err := errors.New("zkpath is not valid")
		return typ, "", err
	}

	typStr := parts[1]

	if typStr != "config" && typStr != "service" {
		return typ, "", errors.Errorf("unknown zkpath: %v", zkPath)
	}

	var prefix string
	if typStr == "service" {
		if len(parts) == 3 {
			typ = zkRoute
			prefix = etcdkey.JavaZKRelRouteDir()
			glog.Infof("route: %v", prefix)
		} else {
			typ = zkInstance
			prefix = etcdkey.JavaZKRelInstanceDir()
		}
	} else if typStr == "config" {
		typ = zkConfig
		prefix = etcdkey.JavaZKRelConfigDir()
	}

	sp.lock.RLock()
	projectname := sp.serviceMap[parts[2]]
	sp.lock.RUnlock()

	if projectname == "" {
		return typ, "", errors.Errorf("unknonw api2.0 service %v", parts[2])
	}

	tmp := make([]string, 0, len(parts)+2)
	tmp = append(tmp, prefix, "2", projectname)
	others := parts[3:]
	tmp = append(tmp, others...)

	return typ, path.Join(tmp...), nil
}

func (sp *simplePathInfo) load() error {
	dir := etcdkey.JavaProjectDir()
	fmt.Printf("dir: %v", dir)
	store, err := generic.GetStoreInstance(dir, false)
	if err != nil {
		return err
	}

	typ := reflect.TypeOf(project.Info{})

	projects := map[string]*project.Info{}
	if err := store.List(context.Background(), "", generic.Everything, projects); err != nil {
		return err
	}

	for _, v := range projects {
		if v.APIVersion != api2 {
			continue
		}
		sp.serviceMap[v.ServiceName] = v.Project
		sp.projectMap[v.Project] = v.ServiceName
	}

	w, err := store.Watch(context.Background(), "", generic.Everything, true, typ)
	if err != nil {
		return err
	}

	h := func(e watch.Event) error {
		glog.Infof("new event: %v", e)
		dat, ok := e.Object.(*project.Info)
		if !ok {
			glog.Fatalf("event object must be an instance of *project.Info, got %T", e.Object)
		}

		if dat.APIVersion != api2 {
			return nil
		}

		switch e.Type {
		case watch.Added, watch.Modified:
			sp.lock.Lock()
			sp.serviceMap[dat.ServiceName] = dat.Project
			sp.projectMap[dat.Project] = dat.ServiceName
			sp.lock.Unlock()

		case watch.Deleted:
			sp.lock.Lock()
			delete(sp.serviceMap, dat.ServiceName)
			delete(sp.projectMap, dat.Project)
			sp.lock.Unlock()
		}

		return nil
	}

	go func() {
		for {
			select {
			case <-sp.stopC:
				w.Stop()
				return
			case event := <-w.ResultChan():
				glog.V(10).Infof("receive an instance event: %v", event)
				switch event.Type {
				case watch.Error:
					err, ok := event.Object.(error)
					if !ok {
						glog.Warningf("event type if error, but event.Object is not an error")
						err = fmt.Errorf("watch got error :%v", event.Object)
					}
					glog.Warningf("watch err: %v", err)
				default:
					if err := h(event); err != nil {
						glog.Errorf("java: zk path info %v", err)
					}
				}
			}
		}
	}()

	return nil
}

func newSimplePathInfo() (PathInfor, error) {
	ret := simplePathInfo{
		serviceMap: map[string]string{},
		projectMap: map[string]string{},
	}

	if err := ret.load(); err != nil {
		return nil, err
	}

	return &ret, nil
}

func (sp *simplePathInfo) Stop() {
	close(sp.stopC)
}

// PathInfor get java zk path info
type PathInfor interface {
	GetRoutePath(name types.DeployName) (string, error)
	GetInstancePath(name types.DeployName) (string, error)
	GetDeployName(path string) (types.DeployName, error)
	GetAPIVersion(name types.DeployName) string
	GetEtcdPath(env types.Stage, zkPath string) (zkTyp, string, error)
}
