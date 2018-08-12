/*
Sniperkit-Bot
- Status: analyzed
*/

package types

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/blang/semver"
	"github.com/golang/glog"
	"github.com/pkg/errors"
)

// Version represent a build version num of an image
// Version is should meet the semver 2.0 format plus a leading char 'v'
// A version may have a build tag, specify which stage this version can be deployed to
// if not build tag, it can be deployed to all version
// samples: v1.1.1-rc1+qa.1, v1.1.1+prd
type Version struct {
	version    semver.Version
	gitCommit  string
	desc       string
	createTime time.Time
}

var (
	versionRexp = regexp.MustCompile(`^v.+$`)
)

// GetGitCommit return git commmit id of this version
func (v Version) GetGitCommit() string {
	return v.gitCommit
}

// GetDesc get description of this version
func (v Version) GetDesc() string {
	return v.desc
}

// GetCreateTime return create time of this version
func (v Version) GetCreateTime() time.Time {
	return v.createTime
}

// MustParseVersion parseVersion string, and panics when err happens
func MustParseVersion(ver string) *Version {
	v, err := ParseVersion(ver)
	if err != nil {
		glog.Fatal(err)
	}

	return v
}

// ParseVersion parse v version string  to Version
func ParseVersion(ver string) (*Version, error) {
	if !versionRexp.MatchString(ver) {
		return nil, errors.Errorf("version: version should start with a v followed by a semver string: %v", ver)
	}

	ver = ver[1:]

	v, err := semver.Parse(ver)
	if err != nil {
		return nil, errors.Wrap(err, "version: parse semver version string")
	}

	return &Version{version: v}, nil
}

// MarshalJSON json.Marshaler interface
func (v Version) MarshalJSON() ([]byte, error) {
	ver := fmt.Sprintf("v%s", v.version.String())

	return json.Marshal(ver)
}

// UnmarshalJSON implements json.Unmarshaler interface
func (v *Version) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	ver, err := ParseVersion(s)
	if err != nil {
		return err
	}

	v.version = ver.version
	return nil
}

// String Stringer
func (v Version) String() string {
	return "v" + v.version.String()
}

// LT check v is lt the o
func (v Version) LT(o Version) bool {
	return v.version.LT(o.version)
}

// EQ check if two version are equal
func (v Version) EQ(o *Version) bool {
	return v.version.Equals(o.version)
}

// GetStage which  stage this version can be deployed to
func (v Version) GetStage() Stage {
	b := v.version.Build
	if len(b) == 0 {
		return Production
	}

	var st string
	if len(b) > 1 {
		st = b[0]
	}

	s, err := ParseStage(st)
	if err != nil {
		glog.Warningf("Parse version stage error: %v", err)
	}

	return s
}
