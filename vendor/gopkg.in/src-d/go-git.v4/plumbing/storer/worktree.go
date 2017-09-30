package storer

import "gopkg.in/src-d/go-git.v4/plumbing/format/worktree"

// WorktreeStorer  manages a worktree under dotgit
type WorktreeStorer interface {
	// ListWOrktrees excepts the  default one
	ListWorktrees() ([]worktree.Info, error)
	// SwitchToWorktree switch to the given worktree
	// if error happens, origin repo not changed
	SwitchToWorktree(name string) error
	// SetWorktree init a  new worktree
	SetWorktree(wt worktree.Info) error
}
