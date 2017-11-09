package types

import (
	"encoding/json"
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
type Stage int32

const (
	UnknownStage Stage = iota
	Dev
	Test
	Intergration
	QA
	Production
)

// Empty empty struct
type Empty struct{}

var stages = map[Stage]string{
	Dev:          "dev",
	Test:         "test",
	QA:           "qa",
	Intergration: "int",
	Production:   "prd",
	UnknownStage: "unknown",
}

func (s Stage) String() string {
	st, ok := stages[s]
	if !ok {
		st = stages[UnknownStage]
	}

	return st
}

// MarshalJSON json.Marshaler interface
func (s Stage) MarshalJSON() ([]byte, error) {
	st := s.String()
	return json.Marshal(st)
}

// ParseStage  string to stage
func ParseStage(st string) (Stage, error) {
	for k, v := range stages {
		if v == st {
			return k, nil
		}
	}

	return UnknownStage, errors.Errorf("unknown stage: %v", st)
}

// UnmarshalJSON implements json.Unmarshaler interface
func (s *Stage) UnmarshalJSON(data []byte) error {
	var st string
	if err := json.Unmarshal(data, &st); err != nil {
		return err
	}

	stage, err := ParseStage(st)
	if err != nil {
		return err
	}

	*s = stage
	return nil
}
