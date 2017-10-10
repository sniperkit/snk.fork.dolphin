package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	git "gopkg.in/src-d/go-git.v4"
	"we.com/dolphin/deploy/image"
	"we.com/dolphin/types"
)

type manager struct {
	stage        types.Stage
	deployName   types.DeployKey
	backuper     Backuper
	imageManager image.Manager
}

// New create a new manager
//TODO
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
	wt, err := m.getWorktree(dc)
	if err != nil {
		return err
	}

	s, err := wt.Status()
	if err != nil {
		return err
	}
	if !s.IsClean() {
		return &terror{code: worktreeNotClean}
	}
	// generate and check config file
	if err := m.generateConfig(dc); err != nil {
		return err
	}

	// put config and  image file to the desire place
	if err := wt.Reset(dc.Image.Version.String(), git.HardReset); err != nil {
		return err
	}

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
		if vi.Version.EQ(v) {
			return nil
		}
	}

	return err
}

// assume all version info meet semver v2 format
// return last version avaliable for stage
func getLatestVersion(image types.ImageName, manager image.Manager,
	stage types.Stage) (*types.Version, error) {
	infs, err := manager.Info(image)
	if err != nil {
		return nil, err
	}

	if len(infs) == 0 {
		return nil, errors.Errorf("cannot get version info of image:%v", image)
	}

	for i := len(infs) - 1; i >= 0; i-- {
		if infs[i].Version.GetStage() >= stage {
			return &infs[i].Version, nil
		}
	}

	return nil, nil
}

func getVersion(dc *types.DeployConfig, manager image.Manager, stage types.Stage) (string, error) {
	version := dc.Image.Version
	if version != nil {
		return version.String(), nil
	}

	// image deploy has not specify a version
	// get the last one for current Stage environmet

	v, err := getLatestVersion(dc.Image.Name, manager, stage)
	if err != nil {
		return "", err
	}

	// update dc.Image.Version
	dc.Image.Version = v

	return v.String(), nil
}

// getWorktree returns a worktree to prepare for the deploy:
// the caller should make sure that the needed version of image exists
// if there is not worktree, create a new one
// this also respect the deploy policy
// the directory structure for three different deploy policy are same:
// Inplace:
//  deployDir/deployName
// ABWorld:
// 	deployDir/{deployName-A, deployName-B, deployName}, deployName is a symbolic link to A or B
// Versioned:
// 	deployDir/{deployName-Version, deployName}, deployName is a symbolic link to current workdir
func (m *manager) getWorktree(dc *types.DeployConfig) (image.Worktree, error) {
	dd := dc.GetDeployDir()
	var name = string(dc.Name)

	ensureWt := func(ctx context.Context, name string, path string) (image.Worktree, error) {
		wa, err := m.imageManager.Worktree(dc.Image.Name, name)
		if os.IsNotExist(err) {
			return m.imageManager.NewWorktree(context.Background(), dc.Image.Name, name, path)
		}
		return wa, err
	}

	switch dc.DeployPolicy {
	case types.Inplace:
		p := filepath.Join(dd, name)
		return ensureWt(context.Background(), name, p)

	case types.ABWorld:
		// test current is a or b
		a := fmt.Sprintf("%v-A", name)
		b := fmt.Sprintf("%v-B", name)

		p := filepath.Join(dd, a)
		wa, err := ensureWt(context.Background(), a, p)
		if err != nil {
			return nil, err
		}

		p = filepath.Join(dd, b)
		wb, err := ensureWt(context.Background(), b, p)
		if err != nil {
			return nil, err
		}

		s := filepath.Join(dd, name)
		r, err := filepath.EvalSymlinks(s)
		if err != nil {
			return nil, err
		}

		bs := filepath.Base(r)
		if bs == a {
			return wb, nil
		}

		return wa, nil

	case types.Versioned:
		ver, err := getVersion(dc, m.imageManager, m.stage)
		if err != nil {
			return nil, err
		}
		a := fmt.Sprintf("%v-%v", name, ver)

		p := filepath.Join(dd, a)
		return ensureWt(context.Background(), a, p)

	default:
		return nil, errors.New("unknown Deploy Policy")
	}
}

func (m *manager) checkWorktree(dc *types.DeployConfig) error {

	return nil
}

// generateConfig generat config based on charts templates
func (m *manager) generateConfig(dc *types.DeployConfig) error {

	return nil
}
