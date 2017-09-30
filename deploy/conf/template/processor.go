package template

import (
	"sync"
	"time"

	"github.com/golang/glog"
)

// Processor  process interface
type Processor interface {
	Process()
}

// Process process  a config
func Process(config Config) error {
	ts, err := getResources(config)
	if err != nil {
		return err
	}
	return process(ts)
}

func process(ts []*Resource) error {
	var lastErr error
	for _, t := range ts {
		if err := t.process(); err != nil {
			glog.Error(err.Error())
			lastErr = err
		}
	}
	return lastErr
}

type intervalProcessor struct {
	config   Config
	stopChan chan bool
	doneChan chan bool
	errChan  chan error
	interval int
}

// IntervalProcessor return a Interval processor
func IntervalProcessor(config Config, stopChan, doneChan chan bool, errChan chan error, interval int) Processor {
	return &intervalProcessor{config, stopChan, doneChan, errChan, interval}
}

func (p *intervalProcessor) Process() {
	defer close(p.doneChan)
	for {
		ts, err := getResources(p.config)
		if err != nil {
			glog.Fatal(err.Error())
			break
		}
		process(ts)
		select {
		case <-p.stopChan:
			break
		case <-time.After(time.Duration(p.interval) * time.Second):
			continue
		}
	}
}

type watchProcessor struct {
	config   Config
	stopChan chan bool
	doneChan chan bool
	errChan  chan error
	wg       sync.WaitGroup
}

// WatchProcessor returns a watch processor
func WatchProcessor(config Config, stopChan, doneChan chan bool, errChan chan error) Processor {
	return &watchProcessor{config, stopChan, doneChan, errChan, sync.WaitGroup{}}
}

func (p *watchProcessor) Process() {
	defer close(p.doneChan)
	ts, err := getResources(p.config)
	if err != nil {
		glog.Fatal(err.Error())
		return
	}
	for _, t := range ts {
		t := t
		p.wg.Add(1)
		go p.monitorPrefix(t)
	}
	p.wg.Wait()
}

func (p *watchProcessor) monitorPrefix(t *Resource) {
	defer p.wg.Done()
	keys := appendPrefix(t.Prefix, t.Keys)
	for {
		index, err := t.storeClient.WatchPrefix(t.Prefix, keys, t.lastIndex, p.stopChan)
		if err != nil {
			p.errChan <- err
			// Prevent backend errors from consuming all resources.
			time.Sleep(time.Second * 2)
			continue
		}
		t.lastIndex = index
		if err := t.process(); err != nil {
			p.errChan <- err
		}
	}
}

func getResources(config Config) ([]*Resource, error) {
	var lastError error
	templates := make([]*Resource, 0)
	glog.V(10).Infof("Loading template resources from confdir %v", config.ConfDir)
	if !isFileExist(config.ConfDir) {
		glog.Warningf("Cannot load template resources: confdir '%s' does not exist", config.ConfDir)
		return nil, nil
	}
	paths, err := recursiveFindFiles(config.ConfigDir, "*toml")
	if err != nil {
		return nil, err
	}

	if len(paths) < 1 {
		glog.Warning("Found no templates")
	}

	for _, p := range paths {
		glog.V(10).Infof("Found template: %s", p)
		t, err := NewResource(p, config)
		if err != nil {
			lastError = err
			continue
		}
		templates = append(templates, t)
	}
	return templates, lastError
}
