package zk

import (
	"strings"
	"sync"

	"we.com/dolphin/controllers/java/project"
	"we.com/dolphin/controllers/java/router"
	"we.com/dolphin/registry/etcdkey"
	"we.com/dolphin/types"
)

type simplePathInfo struct {
	lock  sync.RWMutex
	vers  map[types.DeployName]string
	infos map[types.DeployName]*project.Info
}

func (sp *simplePathInfo) GetRoutePath(name types.DeployName) string {
	return etcdkey.JavaZKRelRouteDir() + string(name)
}

func (sp *simplePathInfo) GetInstancePath(name types.DeployName) string {
	return etcdkey.JavaZKRelInstanceDir() + string(name)
}

func (sp *simplePathInfo) GetDeployName(path string) types.DeployName {
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		return "unknown"
	}

	name := parts[2]
	ver := router.APIV2
	if parts[1] == "biz" && len(parts) >= 3 {
		ver = router.APIV4
		name = name + "." + parts[3]
	}

	sp.vers[types.DeployName(name)] = ver

	return types.DeployName(name)
}

func (sp *simplePathInfo) GetAPIVersion(name types.DeployName) string {
	v := sp.vers[name]

	return v
}
