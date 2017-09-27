/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package generic

import (
	"reflect"
	"testing"

	"we.com/dolphin/types"
	"we.com/jiabiao/common/fields"
	"we.com/jiabiao/common/labels"

	"github.com/coreos/etcd/integration"
	"golang.org/x/net/context"
)

func TestCreate(t *testing.T) {
	ctx, store, cluster := testSetup(t)
	defer cluster.Terminate(t)
	etcdClient := cluster.RandClient()

	key := "/testkey"
	out := &types.Instance{}
	obj := &types.Instance{UUID: "11231313", Node: "testNode"}

	// verify that kv pair is empty before set
	getResp, err := etcdClient.KV.Get(ctx, key)
	if err != nil {
		t.Fatalf("etcdClient.KV.Get failed: %v", err)
	}
	if len(getResp.Kvs) != 0 {
		t.Fatalf("expecting empty result on key: %s", key)
	}

	err = store.Create(ctx, key, obj, out, 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	// basic tests of the output
	if obj.UUID != out.UUID {
		t.Errorf("instance  uuid want=%s, get=%s", obj.UUID, out.UUID)
	}
	if obj.Node != out.Node {
		t.Errorf("obj.Node ne out.out: get %v, want %v", out.Node, obj.Node)
	}

	// verify that kv pair is not empty after set
	getResp, err = etcdClient.KV.Get(ctx, key)
	if err != nil {
		t.Fatalf("etcdClient.KV.Get failed: %v", err)
	}
	if len(getResp.Kvs) == 0 {
		t.Fatalf("expecting non empty result on key: %s", key)
	}
}

func TestCreateWithTTL(t *testing.T) {
	ctx, store, cluster := testSetup(t)
	defer cluster.Terminate(t)

	input := &types.Instance{UUID: "testTTL"}
	key := "/somekey"

	out := &types.Instance{}
	if err := store.Create(ctx, key, input, out, 1); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
}

func TestCreateWithKeyExist(t *testing.T) {
	ctx, store, cluster := testSetup(t)
	defer cluster.Terminate(t)
	obj := &types.Instance{UUID: "testMe"}
	key, _ := testPropogateStore(ctx, t, store, obj)
	out := &types.Instance{}
	err := store.Create(ctx, key, obj, out, 0)
	if err == nil || !IsNodeExist(err) {
		t.Errorf("expecting key exists error, but get: %s", err)
	}
}

func TestGet(t *testing.T) {
	ctx, store, cluster := testSetup(t)
	defer cluster.Terminate(t)
	key, storedObj := testPropogateStore(ctx, t, store, &types.Instance{UUID: "testIIIII"})

	tests := []struct {
		key               string
		ignoreNotFound    bool
		expectNotFoundErr bool
		expectedOut       *types.Instance
	}{{ // test get on existing item
		key:               key,
		ignoreNotFound:    false,
		expectNotFoundErr: false,
		expectedOut:       storedObj,
	}, { // test get on non-existing item with ignoreNotFound=false
		key:               "/non-existing",
		ignoreNotFound:    false,
		expectNotFoundErr: true,
	}, { // test get on non-existing item with ignoreNotFound=true
		key:               "/non-existing",
		ignoreNotFound:    true,
		expectNotFoundErr: false,
		expectedOut:       &types.Instance{},
	}}

	for i, tt := range tests {
		out := &types.Instance{}
		err := store.Get(ctx, tt.key, out, tt.ignoreNotFound)
		if tt.expectNotFoundErr {
			if err == nil || !IsNotFound(err) {
				t.Errorf("#%d: expecting not found error, but get: %s", i, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if !reflect.DeepEqual(tt.expectedOut, out) {
			t.Errorf("#%d: pod want=%#v, get=%#v", i, tt.expectedOut, out)
		}
	}
}

func TestUnconditionalDelete(t *testing.T) {
	ctx, store, cluster := testSetup(t)
	defer cluster.Terminate(t)
	key, storedObj := testPropogateStore(ctx, t, store, &types.Instance{UUID: "test11111111"})

	tests := []struct {
		key               string
		expectedObj       *types.Instance
		expectNotFoundErr bool
	}{{ // test unconditional delete on existing key
		key:               key,
		expectedObj:       storedObj,
		expectNotFoundErr: false,
	}, { // test unconditional delete on non-existing key
		key:               "/non-existing",
		expectedObj:       nil,
		expectNotFoundErr: true,
	}}

	for i, tt := range tests {
		out := &types.Instance{} // reset
		err := store.Delete(ctx, tt.key, out)
		if tt.expectNotFoundErr {
			if err == nil || !IsNotFound(err) {
				t.Errorf("#%d: expecting not found error, but get: %s", i, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
		if !reflect.DeepEqual(tt.expectedObj, out) {
			t.Errorf("#%d: pod want=%#v, get=%#v", i, tt.expectedObj, out)
		}
	}
}

func TestList(t *testing.T) {
	cluster := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer cluster.Terminate(t)
	store := newStore(cluster.RandClient(), false, "", nil)
	ctx := context.Background()

	// Setup storage with the following structure:
	//  /
	//   - one-level/
	//  |            - test
	//  |
	//   - two-level/
	//               - 1/
	//              |   - test
	//              |
	//               - 2/
	//                  - test
	preset := []struct {
		key       string
		obj       *types.Instance
		storedObj *types.Instance
	}{{
		key: "/one-level/test",
		obj: &types.Instance{UUID: "foo"},
	}, {
		key: "/two-level/1/test",
		obj: &types.Instance{UUID: "foo"},
	}, {
		key: "/two-level/2/test",
		obj: &types.Instance{UUID: "foo"},
	}}

	for i, ps := range preset {
		preset[i].storedObj = &types.Instance{}
		err := store.Create(ctx, ps.key, ps.obj, preset[i].storedObj, 0)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	}

	tests := []struct {
		prefix      string
		pred        SelectionPredicate
		expectedOut []*types.Instance
	}{{ // test List on existing key
		prefix:      "/one-level/",
		pred:        Everything,
		expectedOut: []*types.Instance{preset[0].storedObj},
	}, { // test List on non-existing key
		prefix:      "/non-existing/",
		pred:        Everything,
		expectedOut: nil,
	}, { // test List with pod name matching
		prefix: "/one-level/",
		pred: SelectionPredicate{
			Label: labels.Everything(),
			Field: fields.ParseSelectorOrDie("uuid!=" + string(preset[0].storedObj.UUID)),
			GetAttrs: func(obj interface{}) (labels.Set, fields.Set, error) {
				pod := obj.(**types.Instance)
				return nil, fields.Set{"uuid": (string)((**pod).UUID)}, nil
			},
		},
		expectedOut: nil,
	}, { // test List with multiple levels of directories and expect flattened result
		prefix:      "/two-level/",
		pred:        Everything,
		expectedOut: []*types.Instance{preset[1].storedObj, preset[2].storedObj},
	}}

	for i, tt := range tests {
		out := []*types.Instance{}
		err := store.List(ctx, tt.prefix, tt.pred, &out)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(tt.expectedOut) != len(out) {
			t.Errorf("#%d: length of list want=%d, get=%d", i, len(tt.expectedOut), len(out))
			continue
		}
		for j, wantPod := range tt.expectedOut {
			getPod := (out)[j]
			if !reflect.DeepEqual(wantPod, getPod) {
				t.Errorf("#%d: pod want=%#v, get=%#v", i, wantPod, getPod)
			}
		}
	}
}

func testSetup(t *testing.T) (context.Context, *store, *integration.ClusterV3) {
	cluster := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	store := newStore(cluster.RandClient(), false, "", nil)
	ctx := context.Background()
	return ctx, store, cluster
}

// testPropogateStore helps propogates store with objects, automates key generation, and returns
// keys and stored objects.
func testPropogateStore(ctx context.Context, t *testing.T, store *store, obj *types.Instance) (string, *types.Instance) {
	// Setup store with a key and grab the output for returning.
	key := "/testkey"
	setOutput := &types.Instance{}
	err := store.Create(ctx, key, obj, setOutput, 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	return key, setOutput
}
