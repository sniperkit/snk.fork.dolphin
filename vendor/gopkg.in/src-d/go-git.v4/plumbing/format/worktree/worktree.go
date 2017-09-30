package worktree

import (
	"errors"

	"gopkg.in/src-d/go-git.v4/plumbing"
)

var (
	ErrInfoNil       = errors.New("Info is nil")
	ErrHeadNil       = errors.New("Info head is nil")
	ErrWorktreeEmpty = errors.New("Info worktree empty")
	ErrDefaultName   = errors.New("name is *default*")
	ErrNameEmpty     = errors.New("name is empty")
)

const (
	DefaultWorktreeName = "default"
)

// Info information of a  worktree
type Info struct {
	Name string
	Wt   string
	HEAD *plumbing.Reference
}

// Validate check if info is valid
func (i *Info) Validate() error {
	if i == nil {
		return ErrInfoNil
	}

	if i.Name == "" {
		return ErrNameEmpty
	}

	if i.Name == DefaultWorktreeName {
		return ErrDefaultName
	}

	if i.HEAD == nil {
		return ErrHeadNil
	}

	if i.Wt == "" {
		return ErrWorktreeEmpty
	}

	return nil
}
