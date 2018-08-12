/*
Sniperkit-Bot
- Status: analyzed
*/

package zk

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	zk "github.com/samuel/go-zookeeper/zk"
	"we.com/dolphin/types"
)

func Test_nodeWalk(t *testing.T) {
	c, err := NewClient([]string{"192.168.1.34:9090"})
	if err != nil {
		t.Errorf("cannot connect to zk")
	}
	type args struct {
		prefix string
		c      *Client
		vars   map[string][]byte
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test",
			args: args{
				prefix: "/",
				c:      c,
				vars:   map[string][]byte{},
			}, wantErr: false,
		},
	}

	exp := regexp.MustCompile(`^(/[^/]+){3}/(policy/default)/`)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := nodeWalk(tt.args.prefix, tt.args.c, exp, true, tt.args.vars); (err != nil) != tt.wantErr {
				t.Errorf("nodeWalk() error = %v, wantErr %v", err, tt.wantErr)
			}
			for k := range tt.args.vars {
				glog.Errorf("var:%v", k)
			}
		})
	}
}

func TestClient_WatchPrefix(t *testing.T) {
	c, err := NewClient([]string{"192.168.1.34:9090"})
	if err != nil {
		t.Errorf("cannot connect to zk")
	}

	HandlerEvent := func(event zk.Event) error {
		switch event.Type {
		case zk.EventNodeCreated:
		case zk.EventNodeChildrenChanged:
		case zk.EventNodeDataChanged:
		case zk.EventNodeDeleted:
		case zk.EventNotWatching:
		case zk.EventSession:
		default:
		}
		glog.Errorf("receiver event: %v", event)
		return nil
	}

	ctx, cf := context.WithTimeout(context.Background(), 25*time.Second)
	defer cf()

	ech := make(chan zk.Event, 2)
	go func() {
		for {
			select {
			case event := <-ech:
				HandlerEvent(event)
			}
		}
	}()

	type args struct {
		ctx         context.Context
		path        string
		pathMatcher *regexp.Regexp
		ech         chan<- zk.Event
	}
	tests := []struct {
		name string
		c    *Client
		args args
	}{
		{
			name: "test watch",
			c:    c,
			args: args{
				ctx:         ctx,
				path:        "/adsfadfs",
				pathMatcher: nil,
				ech:         ech,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.c.WatchPrefix(tt.args.ctx, tt.args.path, tt.args.pathMatcher, tt.args.ech)
		})
	}

	<-ctx.Done()
}

func TestMarshalByteArr(t *testing.T) {

	decode := func(dat []byte, obj interface{}) error {
		if d, ok := obj.(*[]byte); ok {
			*d = dat
			return nil
		}

		return json.Unmarshal(dat, obj)
	}

	abc := []byte("111111111231314asfsalfjqftrhaskfdiajhrfwlkjf2i sa1894y9y@^*3SFASF")

	var ret []byte

	err := decode(abc, &ret)
	if err != nil {
		t.Error(err)
	}

	if len(abc) != len(ret) {
		t.Error(fmt.Errorf("abc != ret"))
	}

	for i, c := range abc {
		if c != ret[i] {
			t.Error(fmt.Errorf("abc != ret"))
		}
	}
}

func TestArray(t *testing.T) {
	arr := []int{1, 2, 3}
	t.Logf("%v", arr[3:])
}

func TestZKPATHv2(t *testing.T) {
	var s = "crm-server"

	parts := strings.Split("/service/com.crm/1_134", "/")

	matches := binRe.FindStringSubmatch(parts[3])
	if len(matches) == 0 {
		err := errors.Errorf("zk: GetDeployName bin format error: %v", parts[3])
		t.Errorf("%v", err)
	}

	t.Logf("%v", matches)
	bin := strings.TrimSuffix(parts[3], matches[1])
	t.Logf("%v", types.DeployName(s+":"+bin))
}
