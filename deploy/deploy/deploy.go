package deploy

import (
	"context"
	"errors"
	"strings"
	"time"

	"we.com/dolphin/deploy/image"
	"we.com/dolphin/types"
)

// manager manages  image deployments
type manager struct {
	deployName   types.DeployKey
	imageManager image.Manager
}

// New create a new manager
func New() (Deployer, error) {

	return &manager{}, nil
}

// Log deploy Log entity
type Log struct {
	Time    time.Time
	Version types.Version
}

func (m *manager) History() []Log {

	return nil
}

// Backup backup current  deployment
func (m *manager) Backup() (types.UUID, error) {

	return types.UUID(""), nil
}

func (m *manager) Restore(ctx context.Context, key types.UUID) error {

	return nil
}

func (m *manager) Deploy(dc *types.DeployConfig) error {
	if dc == nil {
		return nil
	}

	// update local image
	if err := m.pullImage(dc.Image); err != nil {
		return err
	}

	// check if local worktree is clean

	// generate and check config file

	// put config and  image file to the desire place

	// restart/clean cache if needed

	// backup  current deployment

	return nil
}

// pullImage update local image repo if need,
// and checks if the expected version exists
func (m *manager) pullImage(image *types.Image) error {
	if image == nil {
		return &terror{code: imageCanntEmpty}
	}

	name := image.Name
	err := m.imageExist(image)
	if image.UpdatePolicy == types.AlwaysUpdate || err != nil {
		if err := m.updateImage(name); err != nil {
			return err
		}
	}

	return m.imageExist(image)
}

func (m *manager) updateImage(name types.ImageName) error {
	ctx := context.Background()
	ctx, cf := context.WithTimeout(ctx, 5*time.Minute)
	defer cf()
	return m.imageManager.Update(ctx, name)
}

func (m *manager) imageExist(image *types.Image) error {
	inf, err := m.imageManager.Info(image.Name)
	if len(inf) == 0 && err != nil {
		return err
	}

	if err == nil {
		err = &terror{code: imageVersionNotExist}
	}
	// not tag info found
	if len(inf) == 0 {
		return err
	}

	// if image version not specify,  any version will meet
	v := image.Version
	if len(inf) > 0 && v == nil {
		return nil
	}

	for _, vi := range inf {
		if vi.Version == v.String() {
			return nil
		}
	}

	return err
}

func (m *manager) checkWorktree(dc *types.DeployConfig) error {
	dd := dc.GetDeployDir()

	var name = string(dc.Name)
	switch dc.DeployPolicy {
	case types.Inplace:
	case types.ABWorld:
		// test current is a or b
		wts, err := m.imageManager.Worktrees(dc.Image.Name)
		if err != nil {
			return err
		}
		for _, v := range wts {
			if strings.HasPrefix(v, name) {

			}
		}

	case types.Versioned:
	default:
		return errors.New("unknown Deploy Policy")
	}

	return nil
}
