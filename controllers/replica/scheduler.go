package replica

import (
	"context"
	"math/rand"
	"os"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	ctypes "we.com/dolphin/controllers/types"
	"we.com/dolphin/registry/etcdkey"
	"we.com/dolphin/registry/generic"
	"we.com/dolphin/types"
	"we.com/jiabiao/common/fields"
	"we.com/jiabiao/common/labels"
)

const (
	emptyHost = types.HostID("")
)

type config struct {
	updatePolicy *types.UpdateOption
}

type scheduler struct {
	require    *ctypes.Require
	avaliable  []types.HostID
	bestEffect bool
	stage      types.Stage
	key        types.DeployKey
}

func newScheduler() ctypes.Scheduler {
	return &scheduler{}
}

func (s *scheduler) selectHost(r labels.Selector) ([]types.HostID, error) {
	// Selector selector host match  the labels selector
	pred := generic.SelectionPredicate{
		Label: r,
		Field: fields.Everything(),
		GetAttrs: func(obj interface{}) (labels.Set, fields.Set, error) {
			ins := obj.(*types.HostInfo)
			return labels.Set((*ins).Labels), nil, nil
		},
	}

	store, err := getStore()
	if err != nil {
		return nil, err
	}

	dir := etcdkey.HostInfoDir(s.stage)
	hinfs := []*types.HostInfo{}
	if err = store.List(context.Background(), dir, pred, &hinfs); err != nil {
		return nil, err
	}

	if len(hinfs) == 0 {
		return nil, nil
	}
	ret := make([]types.HostID, len(hinfs))
	for k, v := range hinfs {
		ret[k] = types.HostID(v.HostID)
	}

	return ret, nil
}

func checkHostStatus(stage types.Stage, hid types.HostID, require types.DeployResource, ins []*types.Instance) error {
	st, err := getHostStatus(stage, hid)
	if err != nil {
		return errors.Wrap(err, "scheduler")
	}

	inf, err := getHostInfo(stage, hid)
	if err != nil {
		return errors.Wrap(err, "scheduler")
	}

	if st == nil || inf == nil {
		return errors.Errorf("scheduler: %v both host info and status is nil", hid)
	}

	free := inf.GetResource()
	rsv := inf.ResourceReserved
	used := st.ResourceUsed()

	free.Subtract(rsv)
	if used != nil {
		free.Subtract(*used)
	}

	sc := free.Devide(require)
	if sc <= 0 {
		return ErrHostShortOfResource
	}

	for _, v := range ins {
		if v.HostID == hid {
			return errors.New("already exists")
		}
	}
	return nil
}

func (s *scheduler) findSuitableHost() (types.HostID, error) {
	r := s.require
	ins, err := getRunningInstances(s.stage, s.key)
	if err != nil {
		return emptyHost, err
	}

	maxTry := 50
	best := emptyHost

	avaliable := s.avaliable
	defer func() { s.avaliable = avaliable }()

	for len(avaliable) > 0 && maxTry > 0 {
		maxTry--

		idx := rand.Intn(len(avaliable))
		hid := avaliable[idx]

		avaliable[idx] = avaliable[len(avaliable)-1]
		avaliable = avaliable[:len(avaliable)-1]

		terr := checkHostStatus(s.stage, hid, r.Resource, ins)
		if terr == nil {
			return hid, nil
		}

		if os.IsExist(terr) {
			best = hid
			err = terr
		}

		glog.Warningf("scheduler: %v", terr)
		if best == emptyHost {
			err = terr
		}
	}

	return best, err
}

func (s *scheduler) NextHost() (types.HostID, error) {

	if len(s.avaliable) == 0 {
		hosts, err := s.selectHost(s.require.HostSelector)
		if err != nil {
			return emptyHost, err
		}

		if len(hosts) == 0 {
			return emptyHost, ErrNoHostMeetCondition
		}

		s.avaliable = hosts
	}
	return s.findSuitableHost()
}
