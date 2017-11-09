package deploy

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"we.com/dolphin/api/utils"
	"we.com/dolphin/registry/etcdkey"
	"we.com/dolphin/registry/generic"
	"we.com/dolphin/types"
	"we.com/jiabiao/common/fields"
	"we.com/jiabiao/common/labels"
)

const (
	envName  = "env"
	typeName = "type"
	nameName = "name"
)

// Install deploy config handler
func Install(r *mux.Router) error {
	s := r.PathPrefix("/deployconfig").Subrouter()

	s.HandleFunc("/{env}/{type}/{name}", utils.HandlefuncWrap(add)).Methods(http.MethodPut)

	s.HandleFunc("/{env}/{type}/{name}", utils.HandlefuncWrap(remove)).Methods(http.MethodDelete)

	s.HandleFunc("/{env}/{type}/{name}", utils.HandlefuncWrap(get)).Methods(http.MethodGet)

	s.HandleFunc("/{env}", utils.HandlefuncWrap(list)).Methods(http.MethodPost)

	return nil
}

func getDeploykey(typ types.ProjectType, name types.DeployName) types.DeployKey {
	return types.DeployKey(fmt.Sprintf("%v/%v", typ, name))
}

func getStageTypeAndName(r *http.Request) (types.Stage, types.ProjectType, types.DeployName, error) {
	vars := mux.Vars(r)
	env := vars[envName]
	typ := vars[typeName]
	name := vars[nameName]

	stage, err := types.ParseStage(env)
	if err != nil {
		return types.UnknownStage, types.ProjectType(""), types.DeployName(""), errors.Wrap(err, "parse env")
	}

	t := types.ProjectType(typ)
	n := types.DeployName(name)

	return stage, t, n, nil
}

// /{env}/{key}
func add(w http.ResponseWriter, r *http.Request) (utils.Model, error) {
	stage, typ, name, err := getStageTypeAndName(r)
	if err != nil {
		return nil, err
	}

	dc := types.DeployConfig{}
	if err := utils.Receive(r, &dc); err != nil {
		return nil, err
	}

	dc.Stage = stage
	dc.Type = typ
	dc.Name = name

	if err := dc.Validate(); err != nil {
		return nil, errors.Wrap(err, "validate deploy config")
	}

	err = addDeployConfig(&dc, false)

	return nil, err
}

func get(w http.ResponseWriter, r *http.Request) (utils.Model, error) {
	stage, typ, name, err := getStageTypeAndName(r)
	if err != nil {
		return nil, err
	}

	key := getDeploykey(typ, name)
	dc, err := getDeployConfig(stage, key)

	return dc, err
}

func remove(w http.ResponseWriter, r *http.Request) (utils.Model, error) {
	stage, typ, name, err := getStageTypeAndName(r)

	if err != nil {
		return nil, err
	}

	key := getDeploykey(typ, name)

	dc, err := deleteDeployConfig(stage, key)

	return dc, err
}

//
func list(w http.ResponseWriter, r *http.Request) (utils.Model, error) {
	query := types.Selector{}
	if err := utils.Receive(r, query); err != nil {
		return nil, err
	}

	s, err := query.ToSelector()
	if err != nil {
		return query, err
	}

	vars := mux.Vars(r)
	str := vars[envName]
	stage, err := types.ParseStage(str)
	if err != nil {
		return nil, err
	}

	dcs, err := queryDeployConfig(stage, s)

	return dcs, err
}

func getStore(stage types.Stage) (generic.Interface, error) {
	prefix := etcdkey.StageBaseDir(stage)
	return generic.GetStoreInstance(prefix, false)
}

func getDeployConfig(stage types.Stage, key types.DeployKey) (*types.DeployConfig, error) {
	store, err := getStore(stage)
	if err != nil {
		return nil, err
	}

	path := etcdkey.DeployInstanceDirOfKey(stage, key)

	ret := types.DeployConfig{}

	if err := store.Get(context.Background(), path, &ret, false); err != nil {
		return nil, err
	}

	return &ret, nil
}

func addDeployConfig(dc *types.DeployConfig, ignoreExist bool) error {
	if dc == nil {
		return nil
	}

	store, err := getStore(dc.Stage)
	if err != nil {
		return err
	}

	path := etcdkey.DeployInstanceDirOfKey(dc.Stage, dc.Key())

	if ignoreExist {
		return store.Update(context.Background(), path, dc, nil, 0)
	}

	return store.Create(context.Background(), path, dc, nil, 0)
}

func deleteDeployConfig(stage types.Stage, key types.DeployKey) (*types.DeployConfig, error) {
	store, err := getStore(stage)
	if err != nil {
		return nil, err
	}

	dc := types.DeployConfig{}

	path := etcdkey.DeployInstanceDirOfKey(stage, key)

	if err := store.Delete(context.Background(), path, &dc); err != nil {
		return nil, err
	}

	return &dc, nil
}

func queryDeployConfig(stage types.Stage, s labels.Selector) ([]*types.DeployConfig, error) {
	store, err := getStore(stage)
	if err != nil {
		return nil, err
	}

	pred := generic.SelectionPredicate{
		Field: fields.Everything(),
		GetAttrs: func(obj interface{}) (labels.Set, fields.Set, error) {
			dc := obj.(*types.DeployConfig)

			return labels.Set((*dc).Labels), nil, nil
		},
	}

	path := etcdkey.DeployConfigDir(stage)
	ret := []*types.DeployConfig{}

	if err = store.List(context.Background(), path, pred, &ret); err != nil {
		return nil, err
	}

	return ret, nil
}
