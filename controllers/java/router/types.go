/*
Sniperkit-Bot
- Status: analyzed
*/

package router

import (
	"fmt"
	"regexp"
	"strings"

	log "github.com/golang/glog"
	multierror "github.com/hashicorp/go-multierror"
	"we.com/dolphin/types"
)

type OP string

const (
	OPeq       OP = "="
	OPne       OP = "!="
	v2FieldSEP    = "="
	v2SEP         = ""
	v4SEP         = "=>"
	v2fieldSEP    = ";"
	v4fieldSEP    = ","
	APIV2         = "2.0"
	APIV4         = "4.0"
)

// Match route config
type Match struct {
	Key   string
	OP    OP
	Value []string
}

func (m Match) getValue(sep string) string {
	val := strings.Join(m.Value, sep)

	return strings.Trim(fmt.Sprintf("%s%s%s", m.Key, m.OP, val), " \t")
}

// RouteItem one  item in a routeCfg
type RouteItem struct {
	Src Match
	Dst Match
}

func (ri RouteItem) getValue(sep string, fsep string) string {
	src := ri.Src.getValue(fsep)
	dest := ri.Dst.getValue(fsep)

	ret := fmt.Sprintf("%s %s %s\n", src, sep, dest)

	return strings.Trim(ret, " \t")
}

var (
	// host = "134294,1231" =>  version=123,455
	//  => version=123
	//  version=123
	v2Re        = regexp.MustCompile(`^\s*([-_\w]+)\s*(=)\s*([-_/.:\w;,]+)\s*$`)
	v4Re        = regexp.MustCompile(`^\s*(([-\w_\*]+)\s*(!?=)\s*([-\w,._:/\*]+))?\s*=>\s*(([-_\w\*]+)\s*(!?=)\s*([-_/,.:\w\*]+))\s*$`)
	commentRe   = regexp.MustCompile(`^\s*#`)
	emptyLineRe = regexp.MustCompile(`^\s*$`)
)

// RouteCfg route config of a java service
type RouteCfg struct {
	APIVersion string
	RouteItems []RouteItem
}

func (rc RouteCfg) String() string {
	ret := ""
	var sep string
	if rc.APIVersion == APIV2 {
		ret = "# auto generated, please donnot  modify\n"
		sep = v2SEP
	} else if rc.APIVersion == APIV4 {
		sep = v4SEP
	}

	fsep := v2fieldSEP
	if rc.APIVersion == APIV4 {
		fsep = v4fieldSEP
	}
	for _, item := range rc.RouteItems {
		ret = ret + item.getValue(sep, fsep)
	}

	log.V(10).Infof("RouteCfg: %v, Value: %s", rc, ret)
	return ret
}

// Router a java router config
type Router interface {
	GetConfig(name types.DeployName) (*RouteCfg, error)
	SetConfig(name types.DeployName, cfg *RouteCfg) error
}

// ZKInfor zk infor of an deployment
type ZKInfor interface {
	GetZkInstances(name types.DeployName) ([]*ServiceNode, error)
}

func parseRouteItem(val string, version string) (*RouteItem, error) {
	p := val
	switch version {
	case APIV2:
		if commentRe.MatchString(p) || emptyLineRe.MatchString(p) {
			return nil, nil
		}

		items := v2Re.FindStringSubmatch(p)
		if items == nil {
			log.Errorf("unknown route item: %s", p)
			err := fmt.Errorf("unknown route item: %s", p)
			return nil, err
		}
		dest := Match{Key: items[1], OP: OP(items[2]), Value: strings.Split(items[3], v2fieldSEP)}
		ni := RouteItem{Src: Match{}, Dst: dest}
		return &ni, nil

	case APIV4:
		if emptyLineRe.MatchString(p) {
			return nil, nil
		}

		items := v4Re.FindStringSubmatch(p)
		if items == nil {
			log.Errorf("unknown route item: %s", p)
			return nil, fmt.Errorf("unknown route item: %s", p)
		}
		src := Match{Key: items[2], OP: OP(items[3])}
		if len(items[4]) > 0 {
			src.Value = strings.Split(items[4], v4fieldSEP)
		}
		dest := Match{Key: items[6], OP: OP(items[7]), Value: strings.Split(items[8], v4fieldSEP)}
		ni := RouteItem{Src: src, Dst: dest}
		return &ni, nil
	default:
		return nil, fmt.Errorf("unknown version %s", version)
	}
}

// Parse parse java zk route config,
func Parse(content string, apiVersion string) (*RouteCfg, error) {
	parts := strings.Split(content, "\n")
	var merr *multierror.Error
	ret := &RouteCfg{APIVersion: apiVersion, RouteItems: []RouteItem{}}
	for _, p := range parts {
		ri, err := parseRouteItem(p, apiVersion)
		if err != nil {
			merr = multierror.Append(merr, err)
			continue
		}
		if ri == nil {
			continue
		}
		if len(ret.RouteItems) == 0 {
			ret.RouteItems = []RouteItem{}
		}

		ret.RouteItems = append(ret.RouteItems, *ri)
	}

	return ret, merr.ErrorOrNil()
}
