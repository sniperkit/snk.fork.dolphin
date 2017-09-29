package etcdkey

import (
	"fmt"

	"we.com/dolphin/types"
)

const (
	basedir      = "/dolphin/"
	deploydir    = "deploy/"
	deployconfig = "config/"
	deployExpect = "expect/"
	deployActual = "actual/"
)

// BaseDir returns  etcd base dir
func BaseDir() string {
	return basedir
}

func StageBaseDir(stage types.Stage) string {
	return fmt.Sprintf("%v%v/", basedir, stage)
}

func DeployDir(stage types.Stage) string {
	return StageBaseDir(stage) + deploydir
}

func DeployConfigDir(stage types.Stage) string {
	return StageBaseDir(stage) + deployconfig
}

func DeployExpectDir(stage types.Stage) string {
	return StageBaseDir(stage) + deployExpect
}

func DeployActualDir(stage types.Stage) string {
	return StageBaseDir(stage) + deployActual
}

func DepoyConfigOfKey(stage types.Stage, key types.DeployKey) string {
	return fmt.Sprintf("%v/%v/", DeployConfigDir(stage), key)
}

func DeployExpectDirOfHost(stage types.Stage, hostID types.HostID) string {
	return fmt.Sprintf("%v/%v/", DeployExpectDir(stage), hostID)
}

func DeployActualDirOfHost(stage types.Stage, hostID types.HostID) string {
	return fmt.Sprintf("%v/%v/", DeployActualDir(stage), hostID)
}
