package registry

import (
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
