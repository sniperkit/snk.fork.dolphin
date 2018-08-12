/*
Sniperkit-Bot
- Status: analyzed
*/

package registry

import (
	"regexp"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"we.com/dolphin/types"
)

// InstanceIdentifier identifier to check if an instance of this type
type InstanceIdentifier struct {
	// executable cmd, as java, beam, redis-server, etc. not path, eg /usr/bin/java
	Exec   string
	EnvMap map[string]string
	Args   string
	ArgsRe *regexp.Regexp
}

// TypeInfo  pidtype
type TypeInfo struct {
	Type       types.ProjectType
	Identifier *InstanceIdentifier
	Parse      types.InstanceParseFunc
	Prober     types.Prober
	Decoder    JSONInsDecoder
}

type typeCount struct {
	Type  types.ProjectType
	Count int64
}

var (
	lock     sync.RWMutex
	registry = make(map[types.ProjectType]*TypeInfo)

	sortedType []*typeCount
)

// Register  register a project type
func Register(pt TypeInfo) error {
	typ := pt.Type
	lock.Lock()
	defer lock.Unlock()
	if _, ok := registry[typ]; ok {
		return errors.New("project type already exists")
	}

	if pt.Parse == nil {
		return errors.New("ps: Parse cannot be nil when register")
	}

	if pt.Identifier == nil {
		return errors.Errorf("ps: identifier cannot be nil when register for :%v", typ)
	}

	registry[typ] = &pt
	sortedType = append(sortedType, &typeCount{Type: typ, Count: 0})
	return nil
}

// GetTypeInfo return TypeInfo of typ, if not exist return nil
func GetTypeInfo(typ types.ProjectType) *TypeInfo {
	lock.RLock()
	defer lock.RUnlock()

	ti, ok := registry[typ]
	if !ok {
		return nil
	}

	ret := *ti
	return &ret
}

const (
	// PTUnknown unknown project type
	PTUnknown = types.ProjectType("unknown")
)

// GetInstanceType given envmap and  cmdline args check which types a instance belongs to
func GetInstanceType(insInfor types.InstanceInfor) types.ProjectType {
	exe := insInfor.GetExe()
	idx := strings.LastIndex(exe, "/")
	if idx > 0 {
		exe = exe[idx+1:]
	}

	lock.Lock()
	defer lock.Unlock()

out:
	for idx, v := range sortedType {
		ti := registry[v.Type]
		// here ti is not nil
		if ti.Identifier.Exec != exe {
			continue
		}

		if len(ti.Identifier.EnvMap) > 0 {
			envMap := insInfor.GetEnvMap()
			for e, val := range ti.Identifier.EnvMap {
				if val != envMap[e] {
					continue out
				}
			}
		}

		if !strings.Contains(insInfor.GetArgs(), ti.Identifier.Args) {
			continue
		}

		if ti.Identifier.ArgsRe != nil {
			if !ti.Identifier.ArgsRe.MatchString(insInfor.GetArgs()) {
				continue out
			}
		}

		v.Count++
		if idx > 1 && sortedType[idx-1].Count < v.Count {
			pre := sortedType[idx-1]
			sortedType[idx-1] = v
			sortedType[idx] = pre
		}
		return v.Type
	}

	return PTUnknown
}
