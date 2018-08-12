/*
Sniperkit-Bot
- Status: analyzed
*/

package zk

import (
	"flag"
	"regexp"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	zkcfg "we.com/dolphin/controllers/java/zk/types"
	"we.com/dolphin/registry/generic"
	"we.com/dolphin/types"
	mytime "we.com/jiabiao/common/time"
)

func Test_manager_ReloadData(t *testing.T) {
	type args struct {
	}
	tests := []struct {
		name    string
		m       *manager
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.m.ReloadData(); (err != nil) != tt.wantErr {
				t.Errorf("manager.ReloadData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_manager_start(t *testing.T) {
	flag.Set("logtostderr", "true")
	flag.Set("v", "15")
	flag.Parse()
	cfg := zkcfg.EnvConfig{
		ENV:         types.Test,
		ZKServers:   []string{"192.168.1.34:9090"},
		DialTimeout: mytime.Duration(10 * time.Second),
		ZKPaths: []zkcfg.PathConfig{
			zkcfg.PathConfig{
				Base:      "/service",
				RegexpStr: `/service(/[^/]+){1,2}`,
				Regexp:    regexp.MustCompile(`/service(/[^/]+){1,2}`),
			},
			zkcfg.PathConfig{
				Base:      "/biz",
				RegexpStr: `/biz(/[^/]+){2}/(instance|daemon|policy/default)`,
				Regexp:    regexp.MustCompile(`/biz(/[^/]+){2}/(instance|daemon|policy/default)`),
			},
		},
	}
	etcdcfg := clientv3.Config{
		Endpoints:   []string{"192.168.1.68:2379"},
		DialTimeout: 5 * time.Second,
	}
	generic.SetEtcdConfig(etcdcfg)

	pi, _ := newSimplePathInfo()
	m, err := newManager(&cfg, pi)
	if err != nil {
		t.Errorf("create manager: %v", err)
	}

	time.Sleep(120 * time.Second)
	m.Destory()
}
