/*
Sniperkit-Bot
- Status: analyzed
*/

package zk

import (
	"testing"
)

func Test_parseZKPathv2(t *testing.T) {
	type args struct {
		zkPath string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "route",
			args: args{
				zkPath: "/service/com.crm",
			},
			want:    "java/zk/route/2/com.crm",
			wantErr: false,
		}, {
			name: "/config/com.crm",
			args: args{
				zkPath: "/config/com.crm",
			},
			want:    "java/zk/config/2/com.crm",
			wantErr: false,
		}, {
			name: "service instance",
			args: args{
				zkPath: "/service/com.crm/server_234",
			},
			want:    "java/zk/instances/2/com.crm/server_234",
			wantErr: false,
		}, {
			name: "config item",
			args: args{
				zkPath: "/config/com.crm/server/logback.xml",
			},
			want:    "java/zk/config/2/com.crm/server/logback.xml",
			wantErr: false,
		}, {
			name: "testErr1",
			args: args{
				zkPath: "/abcd/com.crm/server/logback.xml",
			},
			want:    "",
			wantErr: true,
		}, {
			name: "testErr2",
			args: args{
				zkPath: "/",
			},
			want:    "",
			wantErr: true,
		}, {
			name: "testErr2",
			args: args{
				zkPath: "/service",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got, err := parseZKPathv2(tt.args.zkPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseZKPathv2() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseZKPathv2() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseZKPathv4(t *testing.T) {
	type args struct {
		zkPath string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "route0",
			args: args{
				zkPath: "/biz/t8t-it-sem/app/policy",
			},
			want:    "java/zk/route/4/t8t-it-sem.app",
			wantErr: false,
		},
		{
			name: "route1",
			args: args{
				zkPath: "/biz/t8t-it-sem/app/policy/default",
			},
			want:    "java/zk/route/4/t8t-it-sem.app/default",
			wantErr: false,
		},
		{
			name: "route",
			args: args{
				zkPath: "/biz/t8t-it-sem/app/policy/default/route",
			},
			want:    "java/zk/route/4/t8t-it-sem.app/default/route",
			wantErr: false,
		}, {
			name: "config",
			args: args{
				zkPath: "/biz/t8t-it-sem/app/config/com.crm",
			},
			want:    "java/zk/config/4/t8t-it-sem.app/com.crm",
			wantErr: false,
		}, {
			name: "instance1",
			args: args{
				zkPath: "/biz/t8t-it-sem/app/instance/com.crm",
			},
			want:    "java/zk/instances/4/t8t-it-sem.app/com.crm",
			wantErr: false,
		}, {
			name: "instance2",
			args: args{
				zkPath: "/biz/t8t-it-sem/app/daemon/com.crm",
			},
			want:    "java/zk/instances/4/t8t-it-sem.app/com.crm",
			wantErr: false,
		}, {
			name: "err",
			args: args{
				zkPath: "/biz/t8t-it-sem/app/abc/com.crm",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got, err := parseZKPathv4(tt.args.zkPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseZKPathv4() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseZKPathv4() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getZKPath0(t *testing.T) {
	type args struct {
		etcdPath string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "etcdV4Config",
			args: args{
				etcdPath: "config/4/t8t-fi-abc.app/abc/efg",
			},
			want:    "/biz/t8t-fi-abc/app/config/abc/efg",
			wantErr: false,
		}, {
			name: "etcdV4route",
			args: args{
				etcdPath: "route/4/t8t-fi-abc.app/default/route",
			},
			want:    "/biz/t8t-fi-abc/app/policy/default/route",
			wantErr: false,
		}, {
			name: "etcdV4route1",
			args: args{
				etcdPath: "route/4/t8t-fi-abc.app/abtest/route",
			},
			want:    "/biz/t8t-fi-abc/app/policy/abtest/route",
			wantErr: false,
		}, {
			name: "etcdV4instance",
			args: args{
				etcdPath: "instances/4/t8t-fi-abc.app/server_abcfslk",
			},
			want:    "",
			wantErr: true,
		}, {
			name: "etcdV2instance",
			args: args{
				etcdPath: "instances/2/t8t-fi-abc.app/server_abcfslk",
			},
			want:    "",
			wantErr: true,
		}, {
			name: "etcdV2route",
			args: args{
				etcdPath: "route/2/t8t-fi-abc.ef",
			},
			want:    "/service/t8t-fi-abc.ef",
			wantErr: false,
		}, {
			name: "etcdV2config",
			args: args{
				etcdPath: "config/2/com.redis/abc/efg",
			},
			want:    "/config/com.redis/abc/efg",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getZKPath0(tt.args.etcdPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("getZKPath0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getZKPath0() = %v, want %v", got, tt.want)
			}
		})
	}
}
