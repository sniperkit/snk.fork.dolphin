package router

import (
	"reflect"
	"testing"
)

func Test_parseRouteItem(t *testing.T) {
	type args struct {
		val     string
		version string
	}
	tests := []struct {
		name    string
		args    args
		want    *RouteItem
		wantErr bool
	}{
		{
			name: "test v2",
			args: args{
				val:     `policy=random`,
				version: APIV2,
			},
			want: &RouteItem{
				Src: Match{},
				Dst: Match{
					Key:   "policy",
					OP:    OPeq,
					Value: []string{"random"},
				},
			},
			wantErr: false,
		},
		{
			name: "test v2 ins",
			args: args{
				val:     `instances=1;2`,
				version: APIV2,
			},
			want: &RouteItem{
				Src: Match{},
				Dst: Match{
					Key:   "instances",
					OP:    OPeq,
					Value: []string{"1", "2"},
				},
			},
			wantErr: false,
		},
		{
			name: "test v2 comment",
			args: args{
				val:     `#instances=1;2`,
				version: APIV2,
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "test v4 ",
			args: args{
				val:     `=> version=14`,
				version: APIV4,
			},
			want: &RouteItem{
				Src: Match{},
				Dst: Match{
					Key:   "version",
					OP:    OPeq,
					Value: []string{"14"},
				},
			},
			wantErr: false,
		},

		{
			name: "test v4 ",
			args: args{
				val:     `host != localhost => version=14`,
				version: APIV4,
			},
			want: &RouteItem{
				Src: Match{
					Key:   "host",
					OP:    OPne,
					Value: []string{"localhost"},
				},
				Dst: Match{
					Key:   "version",
					OP:    OPeq,
					Value: []string{"14"},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRouteItem(tt.args.val, tt.args.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRouteItem() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseRouteItem() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parse(t *testing.T) {
	type args struct {
		val     string
		version string
	}
	tests := []struct {
		name    string
		args    args
		want    *RouteCfg
		wantErr bool
	}{
		{
			name: "crm-server",
			args: args{
				val: `alias=crm.utils,crm.bid,crm.tools,crm.item,crm.sv,crm.biddb,crm.finance,crm.itemdb,crm.mainpage,crm.query,crm.svdb,crm.sms,crm.configNode
				#instances=2;4;6
				#instances=5;3;1
				policy=random
				version=215`,
				version: APIV2,
			},
			want: &RouteCfg{
				APIVersion: APIV2,
				RouteItems: []RouteItem{
					RouteItem{
						Dst: Match{
							Key:   "alias",
							OP:    OPeq,
							Value: []string{`crm.utils,crm.bid,crm.tools,crm.item,crm.sv,crm.biddb,crm.finance,crm.itemdb,crm.mainpage,crm.query,crm.svdb,crm.sms,crm.configNode`},
						},
						Src: Match{},
					},
					RouteItem{
						Src: Match{},
						Dst: Match{
							Key:   "policy",
							OP:    OPeq,
							Value: []string{"random"},
						},
					},
					RouteItem{
						Src: Match{},
						Dst: Match{
							Key:   "version",
							OP:    OPeq,
							Value: []string{"215"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "apiv4",
			args: args{
				val:     ` method = find*,list*,get*,is* => host = 172.22.3.94,172.22.3.95,172.22.3.96`,
				version: APIV4,
			},
			want: &RouteCfg{
				APIVersion: APIV4,
				RouteItems: []RouteItem{
					RouteItem{
						Dst: Match{
							Key:   "host",
							OP:    OPeq,
							Value: []string{"172.22.3.94", "172.22.3.95", "172.22.3.96"},
						},
						Src: Match{
							Key:   "method",
							OP:    OPeq,
							Value: []string{"find*", "list*", "get*", "is*"},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parse(tt.args.val, tt.args.version)
			t.Logf("got: %v", got)
			if (err != nil) != tt.wantErr {
				t.Errorf("parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parse() = %v, want %v", got, tt.want)
			}
		})
	}
}
