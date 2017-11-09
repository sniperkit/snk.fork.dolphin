package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"we.com/dolphin/api/deploy"
	"we.com/dolphin/api/host"
	"we.com/dolphin/api/java"
	"we.com/dolphin/logger"
	"we.com/dolphin/registry/generic"
	_ "we.com/dolphin/types/all"
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
