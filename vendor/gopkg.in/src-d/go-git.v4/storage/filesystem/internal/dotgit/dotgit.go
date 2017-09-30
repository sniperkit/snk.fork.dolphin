// https://github.com/git/git/blob/master/Documentation/gitrepository-layout.txt
package dotgit

import (
	"bufio"
	"errors"
	"fmt"
	stdioutil "io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	pkgerrs "github.com/pkg/errors"
	"gopkg.in/src-d/go-billy.v3/osfs"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/worktree"
	"gopkg.in/src-d/go-git.v4/utils/ioutil"

	"gopkg.in/src-d/go-billy.v3"
)

const (
	suffix         = ".git"
	packedRefsPath = "packed-refs"
	configPath     = "config"
	indexPath      = "index"
	shallowPath    = "shallow"
	modulePath     = "modules"
	objectsPath    = "objects"
	packPath       = "pack"
	refsPath       = "refs"
	worktreePath   = "worktrees"
	commondirPath  = "commondir"
	gitdirPath     = "gitdir"

	tmpPackedRefsPrefix = "._packed-refs"

	packExt = ".pack"
	idxExt  = ".idx"

	// DefaultWorktree default worktree name
	DefaultWorktree = worktree.DefaultWorktreeName
)

var (
	// ErrNotFound is returned by New when the path is not found.
	ErrNotFound = errors.New("path not found")
	// ErrIdxNotFound is returned by Idxfile when the idx file is not found
	ErrIdxNotFound = errors.New("idx file not found")
	// ErrPackfileNotFound is returned by Packfile when the packfile is not found
	ErrPackfileNotFound = errors.New("packfile not found")
	// ErrConfigNotFound is returned by Config when the config is not found
	ErrConfigNotFound = errors.New("config file not found")
	// ErrPackedRefsDuplicatedRef is returned when a duplicated reference is
	// found in the packed-ref file. This is usually the case for corrupted git
	// repositories.
	ErrPackedRefsDuplicatedRef = errors.New("duplicated ref found in packed-ref file")
	// ErrPackedRefsBadFormat is returned when the packed-ref file corrupt.
	ErrPackedRefsBadFormat = errors.New("malformed packed-ref")
	// ErrSymRefTargetNotFound is returned when a symbolic reference is
	// targeting a non-existing object. This usually means the repository
	// is corrupt.
	ErrSymRefTargetNotFound = errors.New("symbolic reference target not found")
)

// The DotGit type represents a local git repository on disk. This
// type is not zero-value-safe, use the New function to initialize it.
type DotGit struct {
	commonDir         billy.Filesystem
	gitDir            billy.Filesystem
	cachedPackedRefs  refCache
	packedRefsLastMod time.Time
}

// New returns a DotGit value ready to be used. The path argument must
// be the absolute path of a git repository directory (e.g.
// "/foo/bar/.git").
func New(gitDir billy.Filesystem) *DotGit {
	d, err := newFromGitDir(gitDir)
	if err != nil {
		d = &DotGit{
			commonDir:        gitDir,
			gitDir:           nil,
			cachedPackedRefs: make(refCache),
		}
	}

	return d
}

func newFromGitDir(gitDir billy.Filesystem) (*DotGit, error) {
	commonDir := gitDir
	gitdir := gitDir

	f, err := gitDir.Open(gitDir.Join(commondirPath))
	if err == nil {
		defer ioutil.CheckClose(f, &err)
		cd, _ := stdioutil.ReadAll(f)
		cdir := strings.TrimSpace(string(cd))
		commonDir = osfs.New(cdir)
	} else if os.IsNotExist(err) {
		gitdir = nil
	} else {
		return nil, pkgerrs.Wrap(err, "git: get git common dir :%v")
	}

	return &DotGit{commonDir: commonDir,
		gitDir:           gitdir,
		cachedPackedRefs: make(refCache),
	}, nil
}

// Initialize creates all the folder scaffolding.
func (d *DotGit) Initialize() error {
	mustExists := []string{
		d.commonDir.Join("objects", "info"),
		d.commonDir.Join("objects", "pack"),
		d.commonDir.Join("refs", "heads"),
		d.commonDir.Join("refs", "tags"),
	}

	for _, path := range mustExists {
		_, err := d.commonDir.Stat(path)
		if err == nil {
			continue
		}

		if !os.IsNotExist(err) {
			return err
		}

		if err := d.commonDir.MkdirAll(path, os.ModeDir|os.ModePerm); err != nil {
			return err
		}
	}

	return nil
}

// ConfigWriter returns a file pointer for write to the config file
func (d *DotGit) ConfigWriter() (billy.File, error) {
	return d.commonDir.Create(configPath)
}

// Config returns a file pointer for read to the config file
func (d *DotGit) Config() (billy.File, error) {
	return d.commonDir.Open(configPath)
}

// IndexWriter returns a file pointer for write to the index file
func (d *DotGit) IndexWriter() (billy.File, error) {
	if d.gitDir != nil {
		return d.gitDir.Create(indexPath)
	}
	return d.commonDir.Create(indexPath)
}

// Index returns a file pointer for read to the index file
func (d *DotGit) Index() (billy.File, error) {
	if d.gitDir != nil {
		return d.gitDir.Open(indexPath)
	}
	return d.commonDir.Open(indexPath)
}

// ShallowWriter returns a file pointer for write to the shallow file
func (d *DotGit) ShallowWriter() (billy.File, error) {
	return d.commonDir.Create(shallowPath)
}

// Shallow returns a file pointer for read to the shallow file
func (d *DotGit) Shallow() (billy.File, error) {
	f, err := d.commonDir.Open(shallowPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	return f, nil
}

// NewObjectPack return a writer for a new packfile, it saves the packfile to
// disk and also generates and save the index for the given packfile.
func (d *DotGit) NewObjectPack() (*PackWriter, error) {
	return newPackWrite(d.commonDir)
}

// ObjectPacks returns the list of availables packfiles
func (d *DotGit) ObjectPacks() ([]plumbing.Hash, error) {
	packDir := d.commonDir.Join(objectsPath, packPath)
	files, err := d.commonDir.ReadDir(packDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	var packs []plumbing.Hash
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), packExt) {
			continue
		}

		n := f.Name()
		h := plumbing.NewHash(n[5 : len(n)-5]) //pack-(hash).pack
		packs = append(packs, h)

	}

	return packs, nil
}

// ObjectPack returns a fs.File of the given packfile
func (d *DotGit) ObjectPack(hash plumbing.Hash) (billy.File, error) {
	file := d.commonDir.Join(objectsPath, packPath, fmt.Sprintf("pack-%s.pack", hash.String()))

	pack, err := d.commonDir.Open(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrPackfileNotFound
		}

		return nil, err
	}

	return pack, nil
}

// ObjectPackIdx returns a fs.File of the index file for a given packfile
func (d *DotGit) ObjectPackIdx(hash plumbing.Hash) (billy.File, error) {
	file := d.commonDir.Join(objectsPath, packPath, fmt.Sprintf("pack-%s.idx", hash.String()))
	idx, err := d.commonDir.Open(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrPackfileNotFound
		}

		return nil, err
	}

	return idx, nil
}

// NewObject return a writer for a new object file.
func (d *DotGit) NewObject() (*ObjectWriter, error) {
	return newObjectWriter(d.commonDir)
}

// Objects returns a slice with the hashes of objects found under the
// .git/objects/ directory.
func (d *DotGit) Objects() ([]plumbing.Hash, error) {
	files, err := d.commonDir.ReadDir(objectsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	var objects []plumbing.Hash
	for _, f := range files {
		if f.IsDir() && len(f.Name()) == 2 && isHex(f.Name()) {
			base := f.Name()
			d, err := d.commonDir.ReadDir(d.commonDir.Join(objectsPath, base))
			if err != nil {
				return nil, err
			}

			for _, o := range d {
				objects = append(objects, plumbing.NewHash(base+o.Name()))
			}
		}
	}

	return objects, nil
}

// Object return a fs.File pointing the object file, if exists
func (d *DotGit) Object(h plumbing.Hash) (billy.File, error) {
	hash := h.String()
	file := d.commonDir.Join(objectsPath, hash[0:2], hash[2:40])

	return d.commonDir.Open(file)
}

func (d *DotGit) SetRef(r *plumbing.Reference) error {
	var content string
	switch r.Type() {
	case plumbing.SymbolicReference:
		content = fmt.Sprintf("ref: %s\n", r.Target())
	case plumbing.HashReference:
		content = fmt.Sprintln(r.Hash().String())
	}

	fs := d.commonDir
	if r.Name() == plumbing.HEAD && d.gitDir != nil {
		fs = d.gitDir
	}

	f, err := fs.Create(r.Name().String())
	if err != nil {
		return err
	}

	defer ioutil.CheckClose(f, &err)

	_, err = f.Write([]byte(content))
	return err
}

// Refs scans the git directory collecting references, which it returns.
// Symbolic references are resolved and included in the output.
func (d *DotGit) Refs() ([]*plumbing.Reference, error) {
	var refs []*plumbing.Reference
	var seen = make(map[plumbing.ReferenceName]bool)
	if err := d.addRefsFromRefDir(&refs, seen); err != nil {
		return nil, err
	}

	if err := d.addRefsFromPackedRefs(&refs, seen); err != nil {
		return nil, err
	}

	if err := d.addRefFromHEAD(&refs); err != nil {
		return nil, err
	}

	return refs, nil
}

// Ref returns the reference for a given reference name.
func (d *DotGit) Ref(name plumbing.ReferenceName) (*plumbing.Reference, error) {
	ref, err := d.readReferenceFile(".", name.String())
	if err == nil {
		return ref, nil
	}

	return d.packedRef(name)
}

func (d *DotGit) syncPackedRefs() error {
	fi, err := d.commonDir.Stat(packedRefsPath)
	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		return err
	}

	if d.packedRefsLastMod.Before(fi.ModTime()) {
		d.cachedPackedRefs = make(refCache)
		f, err := d.commonDir.Open(packedRefsPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		defer ioutil.CheckClose(f, &err)

		s := bufio.NewScanner(f)
		for s.Scan() {
			ref, err := d.processLine(s.Text())
			if err != nil {
				return err
			}

			if ref != nil {
				d.cachedPackedRefs[ref.Name()] = ref
			}
		}

		d.packedRefsLastMod = fi.ModTime()

		return s.Err()
	}

	return nil
}

func (d *DotGit) packedRef(name plumbing.ReferenceName) (*plumbing.Reference, error) {
	if err := d.syncPackedRefs(); err != nil {
		return nil, err
	}

	if ref, ok := d.cachedPackedRefs[name]; ok {
		return ref, nil
	}

	return nil, plumbing.ErrReferenceNotFound
}

// RemoveRef removes a reference by name.
func (d *DotGit) RemoveRef(name plumbing.ReferenceName) error {
	path := d.commonDir.Join(".", name.String())
	_, err := d.commonDir.Stat(path)
	if err == nil {
		return d.commonDir.Remove(path)
	}

	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return d.rewritePackedRefsWithoutRef(name)
}

func (d *DotGit) addRefsFromPackedRefs(refs *[]*plumbing.Reference, seen map[plumbing.ReferenceName]bool) (err error) {
	if err := d.syncPackedRefs(); err != nil {
		return err
	}

	for name, ref := range d.cachedPackedRefs {
		if !seen[name] {
			*refs = append(*refs, ref)
			seen[name] = true
		}
	}

	return nil
}

func (d *DotGit) rewritePackedRefsWithoutRef(name plumbing.ReferenceName) (err error) {
	f, err := d.commonDir.Open(packedRefsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	// Creating the temp file in the same directory as the target file
	// improves our chances for rename operation to be atomic.
	tmp, err := d.commonDir.TempFile("", tmpPackedRefsPrefix)
	if err != nil {
		return err
	}

	s := bufio.NewScanner(f)
	found := false
	for s.Scan() {
		line := s.Text()
		ref, err := d.processLine(line)
		if err != nil {
			return err
		}

		if ref != nil && ref.Name() == name {
			found = true
			continue
		}

		if _, err := fmt.Fprintln(tmp, line); err != nil {
			return err
		}
	}

	if err := s.Err(); err != nil {
		return err
	}

	if !found {
		return nil
	}

	if err := f.Close(); err != nil {
		ioutil.CheckClose(tmp, &err)
		return err
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	return d.commonDir.Rename(tmp.Name(), packedRefsPath)
}

// process lines from a packed-refs file
func (d *DotGit) processLine(line string) (*plumbing.Reference, error) {
	if len(line) == 0 {
		return nil, nil
	}

	switch line[0] {
	case '#': // comment - ignore
		return nil, nil
	case '^': // annotated tag commit of the previous line - ignore
		return nil, nil
	default:
		ws := strings.Split(line, " ") // hash then ref
		if len(ws) != 2 {
			return nil, ErrPackedRefsBadFormat
		}

		return plumbing.NewReferenceFromStrings(ws[1], ws[0]), nil
	}
}

func (d *DotGit) addRefsFromRefDir(refs *[]*plumbing.Reference, seen map[plumbing.ReferenceName]bool) error {
	return d.walkReferencesTree(refs, []string{refsPath}, seen)
}

func (d *DotGit) walkReferencesTree(refs *[]*plumbing.Reference, relPath []string, seen map[plumbing.ReferenceName]bool) error {
	files, err := d.commonDir.ReadDir(d.commonDir.Join(relPath...))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	for _, f := range files {
		newRelPath := append(append([]string(nil), relPath...), f.Name())
		if f.IsDir() {
			if err = d.walkReferencesTree(refs, newRelPath, seen); err != nil {
				return err
			}

			continue
		}

		ref, err := d.readReferenceFile(".", strings.Join(newRelPath, "/"))
		if err != nil {
			return err
		}

		if ref != nil && !seen[ref.Name()] {
			*refs = append(*refs, ref)
			seen[ref.Name()] = true
		}
	}

	return nil
}

func (d *DotGit) addRefFromHEAD(refs *[]*plumbing.Reference) error {
	ref, err := d.readReferenceFile(".", "HEAD")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	*refs = append(*refs, ref)
	return nil
}

func (d *DotGit) readReferenceFile(path, name string) (ref *plumbing.Reference, err error) {
	fs := d.commonDir
	if name == "HEAD" && d.gitDir != nil {
		fs = d.gitDir
		path = d.gitDir.Join(path, d.gitDir.Join(name))
	} else {
		path = d.commonDir.Join(path, d.commonDir.Join(strings.Split(name, "/")...))
	}

	f, err := fs.Open(path)
	if err != nil {
		return nil, err
	}
	defer ioutil.CheckClose(f, &err)

	b, err := stdioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	line := strings.TrimSpace(string(b))
	return plumbing.NewReferenceFromStrings(name, line), nil
}

// Module return a billy.Filesystem poiting to the module folder
func (d *DotGit) Module(name string) (billy.Filesystem, error) {
	return d.commonDir.Chroot(d.commonDir.Join(modulePath, name))
}

// Worktrees return a list of worktrees of this repo
// not include the default one
func (d *DotGit) Worktrees() ([]worktree.Info, error) {
	var ret []worktree.Info

	var merr *multierror.Error

	ds, err := d.commonDir.ReadDir(d.commonDir.Join(worktreePath))
	if err != nil {
		if os.IsNotExist(err) {
			return ret, nil
		}
		return ret, err
	}

	for _, wd := range ds {
		if !wd.IsDir() {
			err = pkgerrs.Errorf("git: worktree %v is should be a dir", wd.Name())
			merr = multierror.Append(merr, err)
			continue
		}
		f, err := d.commonDir.Open(d.commonDir.Join(worktreePath, wd.Name(), gitdirPath))
		if err != nil {
			merr = multierror.Append(merr, err)
			continue
		}
		content, err := stdioutil.ReadAll(f)
		ioutil.CheckClose(f, &err)
		if err != nil {
			err = pkgerrs.Wrap(err, fmt.Sprintf("git: read content of gitdir: %v", wd.Name()))
			merr = multierror.Append(merr, err)
			continue
		}
		path := strings.TrimSpace(string(content))

		// locked worktree not support
		// workdir is outside of dotgit, so here we ues os.Stat
		st, err := os.Stat(path)
		if err != nil && !os.IsNotExist(err) {
			merr = multierror.Append(merr, err)
			continue
		} else if os.IsNotExist(err) {
			//  .git may not exist under worktree
			// so we further check, if the worktree dir exists
			dir := filepath.Dir(path)
			if _, err := os.Stat(dir); err != nil {
				err = pkgerrs.Wrap(err, "dotgit: stat worktree")
				merr = multierror.Append(merr, err)
				continue
			}
		} else if st.IsDir() {
			err = pkgerrs.Errorf("git: expect gitdir to be a file, %v", path)
			merr = multierror.Append(merr, err)
			continue
		}

		name := filepath.Join(worktreePath, wd.Name(), "HEAD")
		r, err := d.readReferenceFile(".", name)

		ret = append(ret, worktree.Info{
			Wt:   filepath.Dir(path),
			HEAD: r,
			Name: wd.Name(),
		})
	}

	return ret, merr.ErrorOrNil()
}

// SwitchToWorktree switch to worktree name
func (d *DotGit) SwitchToWorktree(name string) error {
	if name == DefaultWorktree {
		d.gitDir = nil
		return nil
	}

	gitPath := filepath.Join(worktreePath, name)
	if _, err := d.commonDir.Stat(gitdirPath); os.IsNotExist(err) {
		return err
	}
	gitDir, _ := d.commonDir.Chroot(gitPath)

	gd, err := newFromGitDir(gitDir)
	if err != nil {
		return err
	}

	d.gitDir = gd.gitDir
	return nil
}

// SetWorktree create the worktree dirs, releted file
// except  index file
func (d *DotGit) SetWorktree(info worktree.Info) error {
	if err := info.Validate(); err != nil {
		return err
	}

	path := d.commonDir.Join(worktreePath, info.Name)
	if err := d.commonDir.MkdirAll(path, 0755); err != nil {
		return err
	}

	gitDirContent := filepath.Join(info.Wt, ".git")
	gitDir := d.commonDir.Join(worktreePath, info.Name, gitdirPath)
	if err := stdioutil.WriteFile(gitDir, []byte(gitDirContent), 0644); err != nil {
		return err
	}

	gcdf := d.commonDir.Join(worktreePath, info.Name, commondirPath)

	if err := stdioutil.WriteFile(gcdf, []byte("../.."), 0644); err != nil {
		return err
	}

	if err := d.SetRef(info.HEAD); err != nil {
		return err
	}

	return nil
}

func isHex(s string) bool {
	for _, b := range []byte(s) {
		if isNum(b) {
			continue
		}
		if isHexAlpha(b) {
			continue
		}

		return false
	}

	return true
}

func isNum(b byte) bool {
	return b >= '0' && b <= '9'
}

func isHexAlpha(b byte) bool {
	return b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}

type refCache map[plumbing.ReferenceName]*plumbing.Reference
