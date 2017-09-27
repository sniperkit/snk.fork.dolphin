package types

import (
	"regexp"

	"github.com/pkg/errors"
)

// Namespace is namespace
// namespace must less then 64 chars
type Namespace string

var (
	nsregxp = regexp.MustCompile(`[a-z]+([.-][a-z]+)*`)
	maxlen  = 64
)

// Validate checks if ns is valid
func (ns Namespace) Validate() error {
	if len(ns) > 64 {
		return errors.New("ns: namespace name to long")
	}

	if len(ns) == 0 {
		return errors.New("ns: namespace name is empty")
	}

	if nsregxp.MatchString(string(ns)) {
		return nil
	}
	return errors.New("ns: invalid namespace")
}

// Stage which stage of the release process
type Stage string

const (
	Dev          Stage = "dev"
	Test         Stage = "test"
	QA           Stage = "qa"
	Intergration Stage = "int"
	Production   Stage = "prd"
)

// Empty empty struct
type Empty struct{}

var stages = map[Stage]Empty{
	Dev:          Empty{},
	Test:         Empty{},
	QA:           Empty{},
	Intergration: Empty{},
	Production:   Empty{},
}
