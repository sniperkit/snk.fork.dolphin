package zk

import (
	"os"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	zkcfg "we.com/dolphin/controllers/zk/types"
	"we.com/dolphin/registry/generic"
	"we.com/jiabiao/common/yaml"
)

func TestStart(t *testing.T) {
	reader, err := os.Open("./types/cfg.yml")
	if err != nil {
		t.Error(err)
	}

	decode := yaml.NewYAMLOrJSONDecoder(reader, 4)

	cfg := zkcfg.Config{}
	err = decode.Decode(&cfg)
	if err != nil {
		t.Error(err)
	}

	err = cfg.Validate()
	if err != nil {
		t.Error(err)
	}

	type args struct {
		cfg *zkcfg.Config
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test",
			args: args{
				cfg: &cfg,
			},
			wantErr: false,
		},
	}

	ecfg := clientv3.Config{
		Endpoints:   []string{"192.168.1.68:2379"},
		DialTimeout: 2 * time.Second,
	}
	generic.SetEtcdConfig(ecfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Start(tt.args.cfg); (err != nil) != tt.wantErr {
				t.Errorf("Start() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	time.Sleep(300 * time.Second)
	Stop()
}
