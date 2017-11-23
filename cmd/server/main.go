package main

import (
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
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"we.com/dolphin/api/deploy"
	"we.com/dolphin/api/host"
	"we.com/dolphin/api/java"
	"we.com/dolphin/controllers/java/zk"
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
	insInfos   map[types.Stage]ctypes.InstanceInfor
	hcManagers map[types.Stage]ctypes.HostConfigManager
	schedulers map[types.Stage]scheduler.Manager
)

func reload(stopC chan struct{}) error {
	f, err := os.Open(*cfgFile)
	defer f.Close()
	if err != nil {
		return err
	}
	d := yaml.NewYAMLOrJSONDecoder(f, 4)
	cfg := config{}
	if err := d.Decode(&cfg); err != nil {
		return err
	}

	pi, err := zk.NewZKPathInfor()
	if err != nil {
		return err
	}

	if err := zk.Start(&cfg.ZKs); err != nil {
		return err
	}

	// TODO
	var merr *multierror.Error
	infos := map[types.Stage]ctypes.InstanceInfor{}
	for e, zk := range cfg.ZKs.Envs {
		infor := impl.NewInfor(e)
		insInfos[e] = infor
		m, err := impl.NewHCManager(e)
		if err != nil {
		}
		hcManagers[e] = m

		sched, err := scheduler.NewSchedular(e, time.Hour, infor, m)
		if err != nil {
			merr = multierror.Append(merr, err)
			continue
		}
		schedulers[e] = sched
	}

	return nil
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
