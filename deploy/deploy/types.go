/*
Sniperkit-Bot
- Status: analyzed
*/

package deploy

import (
	"context"

	"we.com/dolphin/types"
)

// Backuper backup curret deploy or restore a privous deploy
// on the same host
type Backuper interface {
	Backup() (key types.UUID, err error)
	Restore(ctx context.Context, key types.UUID) error
}

type Configer interface {
	Render() error
	Check() error
}

type Deployer interface {
	Backuper
	Deploy(dc *types.DeployConfig) error
}
