/*
Sniperkit-Bot
- Status: analyzed
*/

package scheduler

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"we.com/dolphin/registry/generic"
	_ "we.com/dolphin/types/all"
	"we.com/dolphin/types/ins/registry"
)

func Test_loadDeployConfig(t *testing.T) {
	ret := map[string]*registry.Instance{}
	generic.SetEtcdConfig(clientv3.Config{
		Endpoints: []string{"192.168.1.68:2379"},
	})

	s, err := generic.GetStoreInstance("/dolphin/dev", false)
	if err != nil {
		t.Errorf("get store: %v", err)
	}

	err = s.List(context.Background(), "/dolphin/dev/deploy/instances/java/", generic.Everything, ret)
	if err != nil {
		t.Errorf("list: %v", err)
	}

	for k, v := range ret {
		d, _ := json.MarshalIndent(v, "", "\t")
		t.Logf("%v: %T", k, v.Private)
		t.Logf("%v: %s", k, d)

	}
}

func Test_timer(t *testing.T) {
	ch := make(chan bool)
	tm := time.AfterFunc(time.Second, func() {
		defer func() { ch <- false }()
		t.Logf("after timer")
	})

	tm.Reset(5 * time.Second)
	<-ch
}
