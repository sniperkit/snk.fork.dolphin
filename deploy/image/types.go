package image

import (
	"context"
	"time"

	git "gopkg.in/src-d/go-git.v4"
	"we.com/dolphin/types"
)

// Info information of a image
type Info struct {
	Name        string
	Version     types.Version
	CommitID    string
	CreateDate  time.Time
	Author      string
	AuthorDate  time.Time
	Size        uint64
	Msg         string
	DeployCount int
}

// Manager  manages local images info
type Manager interface {
	// List gives a image name list
	List() ([]types.ImageName, error)
	// Update update an exists image or clone a new image
	Update(ctx context.Context, name types.ImageName) error
	// Delete delete a give image, even if there is one version deployed
	Delete(ctx context.Context, name types.ImageName) error
	// Info get image info of name
	Info(name types.ImageName) ([]Info, error)

	Worktrees(name types.ImageName) (wts []string, err error)
	Worktree(name types.ImageName, wtname string) (wt Worktree, err error)
	NewWorktree(ctx context.Context, name types.ImageName, wtname string, path string) (Worktree, error)
}

// Worktree likes a git worktree
type Worktree interface {
	Status() (git.Status, error)
	// Backup  take a snapshot of current worktree
	// returns a key which can be used  when  restore
	//  or an err
	// this will not change any in this worktree
	Backup(msg string) (key string, err error)
	Reset(ver string, mode git.ResetMode) error
	Remove(force error) error
}
