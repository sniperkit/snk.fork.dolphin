package project

import "we.com/dolphin/types"

type Info struct {
	APIVersion  string
	ServiceName string
	Name        types.DeployName
	ZKRoute     string
	ZKInstance  string
}
