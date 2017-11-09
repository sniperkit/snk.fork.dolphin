package etcdkey

/*
	deploy info stored in etcd:
	base:
		stage/deploy/
	deploy config:  deploy config of a certain service
		config/{deployID}

	host deploy config:  instances expected running on a host
		hosts/{hostID}/{deployID}

	actual deploy:  actual running instances
		instances/{deployID}/{instanceID}

	agent should watch host deploy config: to start new or stop running instances
	agent is alse responable for  updat actual deployments, this information is important for
	replica controller to schedual deployments
*/

import (
	"fmt"

	"we.com/dolphin/types"
)

const (
	basedir      = "/dolphin/"
	deploydir    = "deploy/"
	deployconfig = "config/"
	deployExpect = "hosts/"
	deployActual = "instances/"
)

// BaseDir returns  etcd base dir
func BaseDir() string {
	return basedir
}

func StageBaseDir(stage types.Stage) string {
	return fmt.Sprintf("%v%v/", basedir, stage.String())
}

func DeployDir(stage types.Stage) string {
	return StageBaseDir(stage) + deploydir
}

func DeployConfigDir(stage types.Stage) string {
	return DeployDir(stage) + deployconfig
}

func DeployHostExpectDir(stage types.Stage) string {
	return DeployDir(stage) + deployExpect
}

func DeployInstanceDir(stage types.Stage) string {
	return DeployDir(stage) + deployActual
}

func DepoyConfigOfKey(stage types.Stage, key types.DeployKey) string {
	return fmt.Sprintf("%v%v", DeployConfigDir(stage), key)
}

func DeployInstanceDirOfKey(stage types.Stage, key types.DeployKey) string {
	return fmt.Sprintf("%v%v/", DeployInstanceDir(stage), key)
}

func DeployHostExpectDirOf(stage types.Stage, hostID types.HostID) string {
	return fmt.Sprintf("%v%v/", DeployHostExpectDir(stage), hostID)
}

func DeployHostExpectPathOf(stage types.Stage, hostID types.HostID, key types.DeployKey) string {
	return fmt.Sprintf("%v%v", DeployHostExpectDirOf(stage, hostID), key)
}
