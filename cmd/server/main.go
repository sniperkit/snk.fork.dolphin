/*
Sniperkit-Bot
- Status: analyzed
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"we.com/dolphin/api/deploy"
	"we.com/dolphin/api/host"
	"we.com/dolphin/api/java"
	"we.com/dolphin/controllers/java/zk"
	zktypes "we.com/dolphin/controllers/java/zk/types"
	"we.com/dolphin/controllers/scheduler"
	ctypes "we.com/dolphin/controllers/types"
	"we.com/dolphin/controllers/types/impl"
	"we.com/dolphin/logger"
	"we.com/dolphin/registry/generic"
	"we.com/dolphin/types"
	_ "we.com/dolphin/types/all"
	"we.com/jiabiao/common/yaml"
)

var (
	srvaddr = flag.String("srv.addr", ":8989", "addr to listen to")
	cfgFile = flag.String("c", "/etc/dolphin/dolphin.yml", "config file address")
)

var (
	envInfos = map[types.Stage]*stageInfo{}
)

type stageInfo struct {
	insInfo   ctypes.InstanceInfor
	hcManager ctypes.HostConfigManager
	dcManager ctypes.DeployConfigManager
	scheduler scheduler.Manager
	zkSyner   zk.Manager
	ctx       context.Context
	df        context.CancelFunc
}

func (si *stageInfo) destroy() error {
	if si.df != nil {
		si.df()
		si.df = nil
	}

	if si.zkSyner != nil {
		si.zkSyner.Destory()
	}

	// TODO: check
	if si.scheduler != nil {

	}

	if si.hcManager != nil {
		si.hcManager.Destroy()
	}

	return nil
}

func newStageInfo(env types.Stage, zkcfg *zktypes.EnvConfig, pi zk.PathInfor) (*stageInfo, error) {
	lease := time.Hour
	m, err := zk.NewManager(zkcfg, pi)
	if err != nil {
		err = errors.Wrap(err, "create zk sync manager")
		return nil, err
	}

	ret := &stageInfo{
		zkSyner: m,
	}

	defer func() {
		if err != nil {
			ret.destroy()
		}
	}()

	insInfo := impl.NewInfor(env)
	ctx := context.Background()
	ctx, df := context.WithCancel(ctx)
	ret.ctx = ctx
	ret.df = df
	if err = insInfo.Start(ctx); err != nil {
		err = errors.Wrap(err, "get instance info")
		return nil, err
	}

	ret.insInfo = insInfo

	hcManager, err := impl.NewHCManager(env)
	if err != nil {
		err = errors.Wrap(err, "create host config manager")
		return nil, err
	}
	ret.hcManager = hcManager
	dcManager, err := impl.NewDeployConfigManager(env)
	if err != nil {
		err = errors.Wrap(err, "create deploy config manager")
		return nil, err
	}

	ret.dcManager = dcManager
	sm, err := scheduler.NewSchedular(env, lease, insInfo, hcManager)
	if err != nil {
		err = errors.Wrap(err, "create scheduler manager")
		return nil, err
	}

	envInfos[env] = ret
	return ret, nil
}

func destroy() error {
	for _, m := range envInfos {
		m.zkSyner.Destory()
	}

	return nil
}

func reload() error {
	// 解析配置文件
	f, err := os.Open(*cfgFile)
	if err != nil {
		return err
	}

	cfg := config{}
	decode := yaml.NewYAMLOrJSONDecoder(f, 4)
	if err := decode.Decode(&cfg); err != nil {
		return err
	}

	// etcd
	generic.SetEtcdConfig(cfg.Etcd)

	// influxdb

	// zk
	pi, err := zk.NewSimplePathInfo()
	if err != nil {
		return err
	}

	var merr *multierror.Error

	for env, zkCfg := range cfg.ZKs.Envs {
		if _, err := newStageInfo(env, &zkCfg, pi); err != nil {
			merr = multierror.Append(merr, err)
		}
	}

	return merr.ErrorOrNil()
}

func main() {
	flag.Parse()
	logger.InitLogs()

	var (
		hup       = make(chan os.Signal)
		hupReady  = make(chan bool)
		term      = make(chan os.Signal)
		webReload = make(chan struct{})
		stopC     = make(chan struct{})
		err       error
	)

	if err := generic.SetEtcdConfigFile("/etc/dolphin/etcd.yml"); err != nil {
		glog.Fatalf("%v", err)
	}

	if err = reload(stopC); err != nil {
		glog.Fatalf("%v", err)
	}

	router := mux.NewRouter()

	router.PathPrefix("/debug/").Handler(http.DefaultServeMux)
	router.PathPrefix("/metrics").Handler(prometheus.Handler())
	deploy.Install(router)
	host.Install(router)
	java.Install(router)

	go listen(router)

	fmt.Println("server started")

	signal.Notify(hup, syscall.SIGHUP)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-hupReady
		for {
			select {
			case <-hup:
			case <-webReload:
			}
			reload(stopC)
		}
	}()

	// Wait for reload or termination signals.
	close(hupReady) // Unblock SIGHUP handler.

	<-term

	glog.Infoln("Received SIGTERM, exiting gracefully...")
	close(stopC)

	<-term
}

func listen(router *mux.Router) {
	if err := http.ListenAndServe(*srvaddr, router); err != nil {
		glog.Fatal(err)
	}
}
