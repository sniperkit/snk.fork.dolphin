package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"we.com/dolphin/logger"
	"we.com/dolphin/registry/generic"
	"we.com/jiabiao/common/etcd"
	"we.com/jiabiao/monitor/api"
)

var (
	srvaddr = flag.String("srv.addr", ":8989", "addr to listen to")
)

func reload(stopC chan struct{}) error {
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
	)
	etcdConfig, err := etcd.NewEtcdConfig("/etc/dolphin/etcd.yml")
	if err != nil {
		glog.Fatalf("%v", err)
	}
	generic.SetEtcdConfig(etcdConfig)

	if err = reload(stopC); err != nil {
		glog.Fatalf("%v", err)
	}

	router := mux.NewRouter()

	router.PathPrefix("/debug/").Handler(http.DefaultServeMux)
	router.PathPrefix("/metrics").Handler(prometheus.Handler())

	apiH := &api.APIHandler{}
	apiH.Registry(router.PathPrefix("/api").Subrouter())

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
