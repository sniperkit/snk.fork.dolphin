/*
Sniperkit-Bot
- Status: analyzed
*/

package types

import (
	"context"

	"we.com/dolphin/types"
)

// Engine a render engine, to generat config from config templates
type Engine interface {
	Render(types.Charts, map[string]string) (map[string]string, error)
}

// LocalStorer  local storer on an host
// this can only update from remote repo, and will not push local changes to remote
type LocalStorer interface {
	// List gives a list of image names, stored locally
	List() (names []types.ImageName, err error)
	// Versions get current versions of an image
	Versions(name types.ImageName) ([]string, error)
	// Update updates local repo  from remote only, and will not change any local worktrees
	Update(ctx context.Context, name types.ImageName) error
	// Worktree
	//	Worktree(name types.ImageName, wtname string) (Worktree, error)
	// Worktress list of worktrees
	Worktrees(name types.ImageName) (wts []string, err error)
	//	NewWorktree(ctx context.Context, name types.ImageName, wtname string, path string) (Worktree, error)
}
