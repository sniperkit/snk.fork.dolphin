/*
Sniperkit-Bot
- Status: analyzed
*/

package etcdkey

import (
	"we.com/dolphin/types"
)

const (
	javaBase        = "java/"
	javaProjectInfo = "java/project/"
	javaProbe       = "java/probe/"
	javaService     = "java/services/"
	javaVersion     = "java/version/"
	javaZKBase      = "java/zk/"
	javaZKRoute     = "java/zk/route/"
	javaZKInstance  = "java/zk/instances/"
	javaZKConfig    = "java/zk/config/"
)

// JavaProbeDir Probe config dir
func JavaProbeDir(stage types.Stage) string {
	return StageBaseDir(stage) + javaProbe
}

// JavaProbePath java probe config path for a given deployname
func JavaProbePath(stage types.Stage, name types.DeployName) string {
	return JavaProbeDir(stage) + string(name)
}

func JavaProjectDir() string {
	return basedir + javaProjectInfo
}

func JavaProjectPath(name types.DeployName) string {
	return JavaProjectDir() + string(name)
}

func JavaZKDir(stage types.Stage) string {
	return StageBaseDir(stage) + javaZKBase
}

func JavaZKRouteDir(stage types.Stage) string {
	return StageBaseDir(stage) + javaZKRoute
}

func JavaZKInstanceDir(stage types.Stage) string {
	return StageBaseDir(stage) + javaZKInstance
}

func JavaZKConfigDir(stage types.Stage) string {
	return StageBaseDir(stage) + javaZKConfig
}

func JavaZKRelConfigDir() string {
	return javaZKConfig
}

func JavaZKRelInstanceDir() string {
	return javaZKInstance
}

func JavaZKRelRouteDir() string {
	return javaZKRoute
}
