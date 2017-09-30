package types

import (
	"context"
	"regexp"

	"github.com/pkg/errors"
	git "gopkg.in/src-d/go-git.v4"
)

// Name name of the image, a name is consist of two parts:
// a namespace of format:  [a-z]+([.-][a-z]+)*
// a name of the same format as namespace
// two parts are seprated by a '/', like "a/b"
type Name string

var (
	// Name have two parts, a namespace and a name
	// and can optional have a  semver,  namespace and name must of format ^[a-z]+([.-][a-z]+)*$
	imageName = regexp.MustCompile(`^([a-z]+([.-][a-z]+)*/[a-z]+([.-][a-z]+)*)(:v([^:]*))?$`)
)

// Validate check  if a image name is valid
func (in Name) Validate() error {
	if imageName.MatchString(string(in)) {
		return nil
	}

	return errors.New("invalid image name")
}

// LocalStorer  local storer on an host
// this can only update from remote repo, and will not push local changes to remote
type LocalStorer interface {
	// List gives a list of image names, stored locally
	List() (names []Name, err error)
	// Versions get current versions of an image
	Versions(name Name) ([]string, error)
	// Update updates local repo  from remote only, and will not change any local worktrees
	Update(ctx context.Context, name Name) error
	// Worktree
	Worktree(name Name, wtname string) (Worktree, error)
	// Worktress list of worktrees
	Worktrees(name Name) (wts []string, err error)
	NewWorktree(ctx context.Context, name Name, wtname string, path string) (Worktree, error)
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
