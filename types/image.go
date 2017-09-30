package types

import (
	"encoding/json"
	"regexp"

	"github.com/pkg/errors"
)

/*
	deploy has two parts:
		-  code
		-  config

	config: is auto generated through  config template

	the code part is called  image
	the config part is call charts (k8s.io/helm)

	different deployment can use the same image

	conventions:
		imagename:  java/crm-server:v1.1.1+build.1
					db/es:v1.3.7
					db/redis:v2.8.3
					db/mysql
*/

// ImageUpdatePolicy how to update local image
type ImageUpdatePolicy string

const (
	// NotExist update local image only when its not exists
	NotExist ImageUpdatePolicy = "notExist"
	// AlwaysUpdate update local image before every deployment
	AlwaysUpdate ImageUpdatePolicy = "always"
)

// ImageName  has not verion info
type ImageName string

// Validate checks if is a valid image name;
func (in ImageName) Validate() error {
	if imageName.MatchString(string(in)) {
		return nil
	}
	return errors.New("invalid imageName")
}

func (in ImageName) String() string {
	return string(in)
}

// Image stand for the code part of a deploy
type Image struct {
	Name         ImageName         `json:"name,omitempty"`
	UpdatePolicy ImageUpdatePolicy `json:"updatePolicy,omitempty"`
	Version      *Version          `json:"version,omitempty"`
}

var (
	// imageName name without version
	imageName = regexp.MustCompile(`^([a-z]+([.-][a-z]+)*/[a-z]+([.-][a-z]+)*)$`)
	// image have two parts, a namespace and a name
	// and can optional have a  semver,  namespace and name must of format ^[a-z]+([.-][a-z]+)*$
	image = regexp.MustCompile(`^([a-z]+([.-][a-z]+)*/[a-z]+([.-][a-z]+)*)(:(v[^:]*))?$`)
)

//Validate  check is a valid  image
func (i Image) Validate() error {
	if !image.MatchString(string(i.Name)) {
		return errors.Errorf("image name format err: %v", i.Name)
	}

	return nil
}

// MarshalJSON json.Marshaler
func (i Image) MarshalJSON() ([]byte, error) {
	if err := i.Validate(); err != nil {
		return nil, err
	}

	if i.UpdatePolicy == "" {
		ret := string(i.Name)
		if i.Version != nil {
			ret += ":" + i.Version.String()
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

// MustParseImageName parse image name and  panics when err happends
func MustParseImageName(name string) *Image {
	i, err := ParseImageName(name)
	if err != nil {
		panic(err)
	}
	return i
}

// ParseImageName  convert an image name to Image struct
// is name is not valid return err, image is nil
func ParseImageName(name string) (*Image, error) {
	part := image.FindStringSubmatch(name)
	if len(part) != 6 {
		return nil, errors.Errorf("image name error, got %v", part)
	}

	n := ImageName(part[1])
	ver := part[5]

	var v *Version
	if len(ver) > 0 {
		var err error
		v, err = ParseVersion(ver)
		if err != nil {
			return nil, err
		}
	}

	return &Image{
		Name:    n,
		Version: v,
	}, nil
}

// Template a unparsed template file
type Template struct {
	// Name  path where to store the parsed template
	Name string
	// Data tempalte  content
	Data []byte
}

// Charts is the config of an image
type Charts struct {
	Image       Image
	keyWords    []string
	Description string
	Owner       []string
	Values      map[string]string
	Templates   []Template
}

// LoadCharts given an image name, load charts config of that charts
func LoadCharts(image *Image) (*Charts, error) {

	return nil, nil
}
