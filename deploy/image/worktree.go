/*
Sniperkit-Bot
- Status: analyzed
*/

package image

import (
	"fmt"
	"time"

	"github.com/influxdata/telegraf/deploy/types"
	"github.com/pkg/errors"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

const (
	backupPrefix = "refs/heads/bak"
)

type empty struct{}

var (
	ignoreTypes = map[string]empty{
		".log": empty{},
		".bak": empty{},
		".gz":  empty{},
		".tar": empty{},
		".swo": empty{},
		".swp": empty{},
		".out": empty{},
		".tmp": empty{},
	}
)

var (
	author = &object.Signature{
		Name:  "ops",
		Email: "ci.robot@corp.to8to.com",
	}
)

type worktree struct {
	*git.Worktree
	ignoreFileTypes map[string]empty
}

var _ types.Worktree = &worktree{}

func (wt *worktree) Status() (git.Status, error) {
	return wt.Worktree.Status()
}

func (wt *worktree) branchExist(b string) error {
	bs, err := wt.Repository().Branches()
	if err != nil {
		return err
	}

	err = errors.New("branch does not exist")
	bs.ForEach(func(h *plumbing.Reference) error {
		if h.String() == b {
			err = nil
			bs.Close()
		}
		return nil
	})

	return err
}

func (wt *worktree) Backup(msg string) (string, error) {
	gs, err := wt.Status()
	if err != nil {
		return "", err
	}

	r := wt.Repository()

	author.When = time.Now()
	// commit local changes
	if !gs.IsClean() {
		for k := range gs {
			wt.Add(k)
		}

		_, err := wt.Commit(msg, &git.CommitOptions{
			Author: author,
		})
		if err != nil {
			return "", err
		}
	}

	head, err := r.Head()
	if err != nil {
		return "", nil
	}

	// create backup branch if not already exists
	bakBranch := fmt.Sprintf("%v/%v", backupPrefix, wt.Name)
	rf, err := r.Reference(plumbing.ReferenceName(bakBranch), false)

	copt := &git.CommitOptions{
		Author: author,
	}
	// if backup branch not exists, create it and return
	if err == plumbing.ErrReferenceNotFound {

		copt.Orphan = true

	} else if err != nil {
		return "", errors.Wrap(err, "git: resolve reference name")
	} else {
		copt.Parents = []plumbing.Hash{
			rf.Hash(), head.Hash(),
		}
	}

	// restore  head branch
	defer func() {
		if err = r.Storer.SetReference(head); err != nil {
			err = errors.Wrap(err, "git: create backup branch, restore HEAD")
		}
	}()

	// commit in current  worktree, and drop
	// history,  later we show  restore to ch
	h, err := wt.Commit(msg, copt)
	if err != nil {
		return "", errors.Wrap(err, "git: create backup commit")
	}

	// update backup branch reference
	rf = plumbing.NewHashReference(plumbing.ReferenceName(bakBranch), h)
	// create a new  bakbranch,  set tip to HEAD
	if err = r.Storer.SetReference(rf); err != nil {
		err = errors.Wrap(err, "git: create backup branch")
		return "", err
	}

	return rf.Hash().String(), nil
}

func (wt *worktree) Reset(ver string, mode git.ResetMode) error {
	h, err := wt.Repository().ResolveRevision(plumbing.Revision(ver))

	if err != nil {
		return errors.Errorf("git: unknown version %v", ver)
	}

	opts := &git.ResetOptions{
		Mode:   mode,
		Commit: *h,
	}
	return wt.Worktree.Reset(opts)
}

func (wt *worktree) Remove(force error) error {
	return errors.New("worktree: remote currently supported, please do it manually")
}
