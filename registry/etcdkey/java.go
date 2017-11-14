package etcdkey

import (
	"we.com/dolphin/types"
)

const (
	javaBase       = "java/"
	javaProbe      = "java/probe/"
	javaService    = "java/services/"
	javaVersion    = "java/version/"
	javaZKRoute    = "java/zk/route/"
	javaZKInstance = "java/zk/instances/"
)

// JavaProbeDir Probe config dir
func JavaProbeDir(stage types.Stage) string {
	return StageBaseDir(stage) + javaProbe
}

// JavaProbePath java probe config path for a given deployname
func JavaProbePath(stage types.Stage, name types.DeployName) string {
	return JavaProbeDir(stage) + string(name)
}
