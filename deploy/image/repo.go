package image

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"time"

	"github.com/blang/semver"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	wt "gopkg.in/src-d/go-git.v4/plumbing/format/worktree"

	"we.com/dolphin/deploy/sync"
	"we.com/dolphin/types"
)

func init() {
	tmp := os.Getenv(repoENV)
	if strings.HasPrefix(tmp, "git@") {
		repoBase = tmp
	}

	releaseTag = os.Getenv("RELEASE_STAG")

	idx := 0
	for i, v := range stages {
		if v == releaseTag {
			idx = i
			break
		}
	}

	for ; idx < len(stages); idx++ {
		ref := fmt.Sprintf("+refs/tags/%v/*:refs/origin/tags/%v/*", stages[idx], stages[idx])
		fetchSpec = append(fetchSpec, config.RefSpec(ref))
	}

	if len(fetchSpec) == 0 {
		ref := fmt.Sprintf("+refs/tags/%v/*:refs/origin/tags/%v/*", "prod", "prod")
		fetchSpec = []config.RefSpec{config.RefSpec(ref)}
	}

}

const (
	repoENV    = "REMOTE_REPO_BASE"
	imageBase  = "images"
	chartsBase = "charts"
)

var (
	localBase = "/data/repos/"
	repoBase  = "git@repo.we.com"

	stages     = []string{"dev", "test", "int", "prod"}
	releaseTag = ""

	fetchSpec = []config.RefSpec{}

	executPool = sync.NewExclusivePool()
)

// gitStore implements  LocalStorer
type gitStore struct {
	LocalRepoBase      string
	MaxHistory         int
	AutoUpdateInterval time.Duration
	DefaultTimeout     time.Duration
}

// NewManager returns a localstore implemented with git
func NewManager() (Manager, error) {
	gs := &gitStore{}

	return gs, nil
}

func (gs *gitStore) List() (Names []types.ImageName, err error) {
	path := filepath.Join(localBase, chartsBase)

	paths, err := filepath.Glob(fmt.Sprintf("%v/*/*.git", path))
	if err != nil {
		return nil, err
	}
	ret := make([]types.ImageName, 0, len(paths))
	for _, v := range paths {
		n := strings.TrimLeft(v, path)
		n = strings.TrimRight(n, ".git")
		ret = append(ret, types.ImageName(n))
	}

	return ret, nil
}

// Info returns a list of versions, the given image has
// versions are tags  while meet the semver spec
func (gs *gitStore) Info(name types.ImageName) ([]Info, error) {
	if err := name.Validate(); err != nil {
		return nil, err
	}

	path := filepath.Join(localBase, chartsBase, fmt.Sprintf("%v.git", name))
	r, err := git.PlainOpen(path)
	if err != nil {
		return nil, errors.Wrap(err, "image: open local git dir")
	}

	rfs, err := r.Tags()
	if err != nil {
		return nil, errors.Wrap(err, "image: get tags of local repo")
	}

	ret := []Info{}
	rfs.ForEach(func(v *plumbing.Reference) error {
		n := v.Name().String()
		n = strings.TrimLeft(n, "refs/tags/")
		sv := n[1:]
		e := Info{
			Name:     name.String(),
			Version:  n,
			CommitID: v.Hash().String(),
		}
		// only tags with semver v2 format
		if _, err := semver.Parse(sv); err != nil {
			return nil
		}
		if co, err := r.CommitObject(v.Hash()); err == nil {
			e.CreateDate = co.Committer.When
			e.Author = co.Author.Name
			e.AuthorDate = co.Author.When
			e.Msg = co.Message
		} else {
			glog.Warningf("git: get commit object of hash: %v, %v", name, e.CommitID)
		}

		ret = append(ret, e)
		return nil
	})

	return ret, nil
}

func (gs *gitStore) Update(ctx context.Context, name types.ImageName) error {
	if err := name.Validate(); err != nil {
		return err
	}

	// charts first
	url := getChartsRepo(name)
	path := getChartsPath(name)
	if err := gs.updateFromRemote(ctx, url, path); err != nil {
		return err
	}

	// update images
	ipath := getImagePath(name)
	iurl := getImageRepo(name)
	if err := gs.updateFromRemote(ctx, iurl, ipath); err != nil {
		return err
	}

	// TODO: maybe shoud check consistancy of image and charts
	return nil
}

func (gs *gitStore) Worktree(name types.ImageName, wtname string) (wt Worktree, err error) {
	path := getImagePath(name)
	r, err := git.PlainOpen(path)

	if err != nil {
		return
	}

	wtt, err := r.SwitchToWorkTree(wtname)
	if err != nil {
		return
	}

	wt = &worktree{Worktree: wtt}
	return
}

func (gs *gitStore) Worktrees(name types.ImageName) (wts []string, err error) {
	path := getImagePath(name)
	r, err := git.PlainOpen(path)

	if err != nil {
		return
	}

	wtinfos, err := r.Worktrees()
	if err != nil {
		return
	}

	for _, v := range wtinfos {
		wts = append(wts, v.Name)
	}
	return
}

func (gs *gitStore) NewWorktree(ctx context.Context, name types.ImageName, wtname string, path string) (Worktree, error) {
	gitPath := getImagePath(name)
	r, err := git.PlainOpen(gitPath)
	if err != nil {
		return nil, err
	}

	info := wt.Info{
		Name: wtname,
		Wt:   path,
	}
	if err = r.NewWorktree(info); err != nil {
		return nil, err
	}

	return gs.Worktree(name, wtname)
}

func (gs *gitStore) Delete(ctx context.Context, name types.ImageName) error {
	return nil
}

// update an existing  repo or clone a new one if not exist
func (gs *gitStore) updateFromRemote(ctx context.Context, url, path string) error {
	_, err := os.Stat(path)

	if err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, fmt.Sprintf("stat gitdir %v", path))
	}

	// if git dir not exists, init it
	if os.IsNotExist(err) {
		// gitDir not exists yet,  init a new one
		r, err := git.PlainInit(path, true)
		if err != nil {
			return errors.Wrap(err, "image: init local gitdir")
		}
		cfg, err := r.Config()
		if err != nil {
			return errors.Wrap(err, "image: get git repo config")
		}

		rcfg := &config.RemoteConfig{
			Name:  "origin",
			URLs:  []string{url},
			Fetch: fetchSpec,
		}
		err = rcfg.Validate()
		if err != nil {
			return errors.Wrap(err, "image: valid remote config")
		}

		if cfg.Remotes == nil {
			cfg.Remotes = map[string]*config.RemoteConfig{}
		}
		cfg.Remotes[rcfg.Name] = rcfg

		r.Storer.SetConfig(cfg)

	}

	r, err := git.PlainOpen(path)
	if err != nil {
		return errors.Wrap(err, "image: open git dir")
	}
	err = r.FetchContext(ctx, &git.FetchOptions{
		RemoteName: "origin",
		Depth:      gs.MaxHistory,
	})
	if err != nil {
		return errors.Wrap(err, "image: fetch from upstream")
	}

	return nil
}

func getChartsPath(name types.ImageName) string {
	return filepath.Join(localBase, chartsBase, fmt.Sprintf("%v.git", name))
}

func getChartsRepo(name types.ImageName) string {
	return fmt.Sprintf("%v:%v/%v.git", localBase, chartsBase, name)
}

func getImagePath(name types.ImageName) string {
	return filepath.Join(localBase, repoBase, fmt.Sprintf("%v.git", name))
}

func getImageRepo(name types.ImageName) string {
	return fmt.Sprintf("%v:%v/%v.git", repoBase, imageBase, name)
}
