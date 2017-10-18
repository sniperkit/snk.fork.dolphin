package types

import (
	"we.com/dolphin/types"
	"we.com/jiabiao/common/labels"
)

// Selector select a list of hosts, meeting the given condidtion
type Selector interface {
	Select(s labels.Selector) (hosts []types.HostID, err error)
}

// HostEvaluator  given a host, get its score
type HostEvaluator interface {
	// Evaluat: evaluat a host for current state
	// a negtive score indicator hosts is under condition, and we should not schedual any
	// not instance to it
	Evaluat(hostID types.HostID) (score float64)
}

// Require requirement host should meet
type Require struct {
	HostSelector labels.Selector
	Resource     types.DeployResource
}

// Scheduler schedual a depoly to list of hosts
type Scheduler interface {
	NextHost() (hostid types.HostID, err error)
}
