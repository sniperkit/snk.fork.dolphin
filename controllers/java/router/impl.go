package router

import (
	"fmt"
	"strings"

	log "github.com/golang/glog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"we.com/dolphin/controllers/java/zk"
	"we.com/dolphin/types"
)

type router struct {
	pathInfo zk.PathInfor
	zkClient *zk.Client
}

func NewRouter(zkClient *zk.Client, pi zk.PathInfor) (Router, error) {
	ret := router{
		pathInfo: pi,
		zkClient: zkClient,
	}

	return &ret, nil
}

func (v *router) GetConfig(name types.DeployName) (*RouteCfg, error) {
	path := v.pathInfo.GetRoutePath(name)
	if path == "" {
		return nil, nil
	}

	content, err := v.zkClient.GetNodeValue(path)
	if err != nil {
		return nil, err
	}

	return parse(content, APIV2)
}

func (v *router) SetConfig(name types.DeployName, cfg *RouteCfg) error {
	path := v.pathInfo.GetRoutePath(name)
	if path == "" {
		return errors.Errorf("dont known zk path form %v", name)
	}

	if cfg == nil {
		return errors.Errorf("route cfg cannot be nil")
	}

	val := cfg.String()
	return v.zkClient.SetNodeValue(path, val)
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

func parse(val string, version string) (*RouteCfg, error) {
	parts := strings.Split(val, "\n")
	var merr *multierror.Error
	ret := &RouteCfg{APIVersion: version, RouteItems: []RouteItem{}}
	for _, p := range parts {
		ri, err := parseRouteItem(p, version)
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
