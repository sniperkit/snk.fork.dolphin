package scheduler

import (
	"context"

	"we.com/dolphin/types"
)

// Manager managers controllers
type Manager interface {
}

type manager struct {
}

func (m *manager) NewDeploy(ctx context.Context, dc *types.DeployConfig) error {

	return nil
}

func (m *manager) Update(key types.DeployKey) error {
	return nil
}

func (m *manager) move(key types.DeployKey, from types.HostID, to types.HostID) error {
	return nil
}
