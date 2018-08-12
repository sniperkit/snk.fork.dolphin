/*
Sniperkit-Bot
- Status: analyzed
*/

package template

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/golang/glog"
)

// fileInfo describes a configuration file and is returned by fileStat.
type fileInfo struct {
	UID  uint32
	GID  uint32
	Mode os.FileMode
	Md5  string
}

func appendPrefix(prefix string, keys []string) []string {
	s := make([]string, len(keys))
	for i, k := range keys {
		s[i] = path.Join(prefix, k)
	}
	return s
}

// isFileExist reports whether path exits.
func isFileExist(fpath string) bool {
	if _, err := os.Stat(fpath); os.IsNotExist(err) {
		return false
	}
	return true
}

// sameConfig reports whether src and dest config files are equal.
// Two config files are equal when they have the same file contents and
// Unix permissions. The owner, group, and mode must match.
// It return false in other cases.
func sameConfig(src, dest string) (bool, error) {
	if !isFileExist(dest) {
		return false, nil
	}
	d, err := fileStat(dest)
	if err != nil {
		return false, err
	}
	s, err := fileStat(src)
	if err != nil {
		return false, err
	}
	if d.UID != s.UID {
		glog.Info(fmt.Sprintf("%s has UID %d should be %d", dest, d.UID, s.UID))
	}
	if d.GID != s.GID {
		glog.Info(fmt.Sprintf("%s has GID %d should be %d", dest, d.GID, s.GID))
	}
	if d.Mode != s.Mode {
		glog.Info(fmt.Sprintf("%s has mode %s should be %s", dest, os.FileMode(d.Mode), os.FileMode(s.Mode)))
	}
	if d.Md5 != s.Md5 {
		glog.Info(fmt.Sprintf("%s has md5sum %s should be %s", dest, d.Md5, s.Md5))
	}
	if d.UID != s.UID || d.GID != s.GID || d.Mode != s.Mode || d.Md5 != s.Md5 {
		return false, nil
	}
	return true, nil
}

// recursiveFindFiles find files with pattern in the root with depth.
func recursiveFindFiles(root string, pattern string) ([]string, error) {
	files := make([]string, 0)
	findfile := func(path string, f os.FileInfo, err error) (inner error) {
		if err != nil {
			return
		}
		if f.IsDir() {
			return
		} else if match, innerr := filepath.Match(pattern, f.Name()); innerr == nil && match {
			files = append(files, path)
		}
		return
	}
	err := filepath.Walk(root, findfile)
	if len(files) == 0 {
		return files, err
	}
	return files, err

}
