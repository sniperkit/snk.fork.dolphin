package java

import (
	"context"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"we.com/dolphin/registry/etcdkey"
	"we.com/dolphin/registry/generic"
	"we.com/dolphin/types"
	"we.com/dolphin/types/ins/java"
)

func Test_diProvider_watch(t *testing.T) {
	cfg := clientv3.Config{
		Endpoints: []string{"192.168.1.68:2379"},
	}
	generic.SetEtcdConfig(cfg)

	dp := diProvider{
		baseDir:    etcdkey.JavaProbeDir(types.UAT),
		interfaces: map[types.DeployName]java.ProbeInterfaces{},
	}

	t.Logf("dir: %v", dp.baseDir)
	ctx := context.Background()

	store, err := generic.GetStoreInstance(dp.baseDir, false)
	if err != nil {
		t.Errorf("get store: %v", err)
	}

	err = store.Update(ctx, "crm-server-1", java.ProbeInterfaces{
		"test": &java.ProbeInterface{
			Name: "test",
			Desc: "test",
			Data: "test data",
		},
	}, nil, 5)
	if err != nil {
		t.Errorf("get store: %v", err)
	}

	ctx, df := context.WithCancel(ctx)

	go dp.watch(ctx)

	time.Sleep(time.Second)
	df()
	t.Logf("success")
}
