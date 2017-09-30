package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
	"we.com/dolphin/deploy/conf/backends"
	"we.com/dolphin/deploy/conf/template"
)

func main() {
	flag.Parse()

	if err := initConfig(); err != nil {
		glog.Fatal(err.Error())
	}

	glog.Info("Starting confd")

	var storeClient backends.StoreClient

	templateConfig.StoreClient = storeClient
	if onetime {
		if err := template.Process(templateConfig); err != nil {
			glog.Fatal(err.Error())
		}
		os.Exit(0)
	}

	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, 10)

	var processor template.Processor
	switch {
	case config.Watch:
		processor = template.WatchProcessor(templateConfig, stopChan, doneChan, errChan)
	default:
		processor = template.IntervalProcessor(templateConfig, stopChan, doneChan, errChan, config.Interval)
	}

	go processor.Process()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case err := <-errChan:
			glog.Error(err.Error())
		case s := <-signalChan:
			glog.Info(fmt.Sprintf("Captured %v. Exiting...", s))
			close(doneChan)
		case <-doneChan:
			os.Exit(0)
		}
	}
}
