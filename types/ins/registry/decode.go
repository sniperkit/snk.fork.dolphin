package registry

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/pkg/errors"
	"we.com/dolphin/types"
)

// JSONInsDecoder read from io.Reader and decode data to an types.Instanse
type JSONInsDecoder interface {
	Decode(r io.Reader) (*types.Instance, error)
}

type decode struct {
}

func (d *decode) Decode(r io.Reader, typ types.ProjectType) (*types.Instance, error) {
	if r == nil {
		return nil, errors.New("reader is nil")
	}

	lock.RLock()
	defer lock.RUnlock()
	t, ok := registry[typ]
	if !ok {
		return nil, errors.Errorf("unknown project type %v", typ)
	}

	return t.Decoder.Decode(r)
}

// Instance is an types.Instance
type Instance types.Instance

// UnmarshalJSON  implements json.UnmarshalJSON interface
func (i *Instance) UnmarshalJSON(dat []byte) error {
	type tmpType struct {
		ProjecType types.ProjectType `json:"projectType,omitempty"`
	}

	d := tmpType{}
	if err := json.Unmarshal(dat, &d); err != nil {
		return err
	}

	decode := GetTypeInfo(d.ProjecType)
	if decode == nil {
		return errors.Errorf("unknown project type %v", d.ProjecType)
	}

	tmp, err := decode.Decoder.Decode(bytes.NewReader(dat))
	if err != nil {
		return err
	}
	ins := (*Instance)(tmp)
	*i = *ins

	return nil
}
