package types

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/blang/semver"
	"github.com/pkg/errors"
)

/*
	deploy has two parts:
		- the code
		- the config

	config: is auto generated through  config template

	the code part is called  image
	the config part is call chartt (k8s.io/helm)
*/

type ImageUpdatePolicy string

const (
	NotExist     ImageUpdatePolicy = "notExist"
	AlwaysUpdate ImageUpdatePolicy = "always"
)

// Image stand for the code part of a deploy
type Image struct {
	Name         string            `json:"name,omitempty"`
	Version      semver.Version    `json:"version,omitempty"`
	UpdatePolicy ImageUpdatePolicy `json:"updatePolicy,omitempty"`
	CommitID     string            `json:"commitID,omitempty"`
}

var (
	// imageName have two parts, a namespace and a name
	// and can optional have a  semver,  namespace and name must of format ^[a-z]+([.-][a-z]+)*$
	imageName = regexp.MustCompile(`^([a-z]+([.-][a-z]+)*/[a-z]+([.-][a-z]+)*)(:v([^:]*))?$`)
)

//Validate  check is a valid  image
func (i Image) Validate() error {
	if !imageName.MatchString(i.Name) {
		return errors.Errorf("image name format err: %v", i.Name)
	}

	return i.Version.Validate()
}

// MarshalJSON json.Marshaler
func (i Image) MarshalJSON() ([]byte, error) {
	if err := i.Validate(); err != nil {
		return nil, err
	}

	if string(i.UpdatePolicy) == "" && i.CommitID == "" {
		ret := i.Name
		v := i.Version.String()
		if v != "0.0.0" {
			ret = fmt.Sprintf("%v:v%v", ret, v)
		}
		return json.Marshal(ret)
	}

	type p Image
	return json.Marshal((p)(i))
}

// UnmarshalJSON json.Unmarshler
func (i *Image) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		tm, err := ParseImageName(s)
		if err != nil {
			return err
		}
		*i = *tm
		return nil
	}

	type p Image

	return json.Unmarshal(data, (*p)(i))
}

// ParseImageName  convert an image name to Image struct
// is name is not valid return err, image is nil
func ParseImageName(name string) (*Image, error) {
	part := imageName.FindStringSubmatch(name)
	if len(part) != 6 {
		return nil, errors.Errorf("image name error, got %v", part)
	}

	n := part[1]
	ver := part[5]
	var v semver.Version
	var err error
	if len(ver) > 0 {
		v, err = semver.Parse(ver)
		if err != nil {
			return nil, errors.Wrap(err, "parse version info")
		}
	}

	return &Image{
		Name:    n,
		Version: v,
	}, nil
}
