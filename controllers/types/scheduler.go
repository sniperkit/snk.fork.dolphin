package types

import (
	"we.com/dolphin/types"
	"we.com/jiabiao/common/labels"
)

// Selector select a list of hosts, meeting the given condidtion
type Selector interface {
	Select(s labels.Selector) (hosts []types.UUID, err error)
}

// HostEvaluator  given a host, get its score
type HostEvaluator interface {
	// Evaluat: evaluat a host for current state
	// a negtive score indicator hosts is under condition, and we should not schedual any
	// not instance to it
	Evaluat(hostID types.UUID) (score float64)
}

// Require requirement host should meet
type Require struct {
	HostSelector labels.Selector
	Resource     types.DeployResource
}

// Schedualer schedual a depoly to list of hosts
type Schedualer interface {
	Schedual(r *Require) (hosts []types.UUID, err error)
}
