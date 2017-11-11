package java

import (
	"encoding/json"
	"io"

	"we.com/dolphin/types"
	"we.com/jiabiao/common/yaml"
)

type decode struct{}

func (d *decode) Decode(r io.Reader) (*types.Instance, error) {
	ret := types.Instance{}

	dr := yaml.NewYAMLOrJSONDecoder(r, 4)
	if err := dr.Decode(&ret); err != nil {
		return nil, err
	}

	if ret.Private != nil {
		dat, _ := json.Marshal(ret.Private)
		private := &InstanceInfo{}
		err := json.Unmarshal(dat, private)
		if err != nil {
			return nil, err
		}
		ret.Private = private
		private.ins = &ret
	}

	return &ret, nil
}
