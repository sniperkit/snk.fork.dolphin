package etcdkey

/*
	difference between:  info, config
*/

import "we.com/dolphin/types"

const (
	hostKey    = "hosts/"
	hostStat   = "stat/"
	hostInfo   = "info/"
	hostConfig = "config/"
)

func HostBaseDir(stage types.Stage) string {
	return StageBaseDir(stage) + hostKey
}

func HostStatDir(stage types.Stage) string {
	return HostBaseDir(stage) + hostStat
}

func HostStatPath(stage types.Stage, hostID types.HostID) string {
	return HostStatDir(stage) + string(hostID)
}

func HostInfoDir(stage types.Stage) string {
	return HostBaseDir(stage) + hostInfo
}

func HostInfoPath(stage types.Stage, hostID types.HostID) string {
	return HostInfoDir(stage) + string(hostID)
}

func HostConfigDir(stage types.Stage) string {
	return HostBaseDir(stage) + hostConfig
}

func HostConfigPath(stage types.Stage, hostName string) string {
	return HostConfigDir(stage) + hostName
}
