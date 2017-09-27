package files

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/golang/glog"
	multierror "github.com/hashicorp/go-multierror"
	"we.com/dolphin/config"

	_ "we.com/dolphin/config/host"
)

/*
	monitor a list of directory file contend change,
	and notify config manger to update their config
*/

var (
	configDir = flag.String("cfg.dir", "/etc/monitor", "directory to find find config files")
)

type monitor struct {
	configMangerMap map[string]config.ConfigManager
}

var (
	once sync.Once
	m    = monitor{
		configMangerMap: map[string]config.ConfigManager{},
	}
)

func registry() {
	a := map[string]struct{}{}
	for typ, mng := range config.GetConfigManagers() {
		dir := mng.MonitorDir()
		dir = filepath.Join(*configDir, dir)
		if _, ok := a[dir]; ok {
			log.Fatalf("more than on manager watch %v for update, %v", dir, typ)
		}
		a[dir] = struct{}{}
		m.configMangerMap[dir] = mng
	}

	for d, mng := range m.configMangerMap {
		_, err := os.Stat(d)
		// if dir not exist,
		if os.IsNotExist(err) {
			log.Fatalf("config dir %v not exits", d)
		}

		if err != nil {
			log.Fatalf("stat dir %v, err: %v", d, err)
		}

		// fmt.Printf("monitor dir %v for %v\n", d, mng.MonitorDir())

		filepath.Walk(d, func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				m.configMangerMap[path] = mng
			}
			return nil
		})
	}
}

// Load project config from disk
func Load() error {
	once.Do(registry)
	var merr *multierror.Error
	for d, mng := range m.configMangerMap {
		glob := filepath.Join(d, "*.json")

		files, err := filepath.Glob(glob)
		if err != nil {
			glog.Warningf("find config file to load: %v", err)
		}
		for _, f := range files {
			glog.V(10).Infof("start to load config: %v", f)
			if err := mng.LoadConfig(f); err != nil {
				merr = multierror.Append(merr, err)
			}
		}

		glob = filepath.Join(d, "*.yml")

		files, err = filepath.Glob(glob)
		if err != nil {
			glog.Warningf("find config file to load: %v", err)
		}
		for _, f := range files {
			glog.V(10).Infof("start to load config: %v", f)
			if err := mng.LoadConfig(f); err != nil {
				merr = multierror.Append(merr, err)
			}
		}

		if strings.HasSuffix(d, ".yml") || strings.HasSuffix(d, ".json") {
			glog.V(10).Infof("start to load config: %v", d)
			if err := mng.LoadConfig(d); err != nil {
				merr = multierror.Append(merr, err)
			}
		}
	}

	return merr.ErrorOrNil()
}

// Watch configuration for change
func Watch(stopC chan struct{}) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		glog.Fatal(err)
	}

	go func() {
		defer watcher.Close()
		for {
			select {
			case event := <-watcher.Events:
				glog.V(10).Infof("event: %v", event)
				if err := handlerEvent(watcher, event); err != nil {
					glog.Errorf("handle file event err: %v", err)
				}

			case err := <-watcher.Errors:
				glog.Warningf("error: %v", err)

			case <-stopC:
				return
			}
		}
	}()

	for d := range m.configMangerMap {
		glog.V(10).Infof("add dir %v to watch", d)
		err := watcher.Add(d)
		if err != nil {
			glog.Fatal(err)
		}
	}
}

//  handerEvent handler file event
func handlerEvent(watcher *fsnotify.Watcher, event fsnotify.Event) error {
	glog.Infof("new file event: %v", event)
	switch {
	case event.Op&fsnotify.Write == fsnotify.Write:
		if !strings.HasSuffix(event.Name, ".json") && !strings.HasSuffix(event.Name, ".yml") {
			glog.Infof("file name ends with '.json' or '.yml', got %v, ignored", event.Name)
			return nil
		}

		f := filepath.Dir(event.Name)
		mng, ok := m.configMangerMap[f]
		if !ok {
			mng, ok = m.configMangerMap[event.Name]
			if !ok {
				glog.Errorf("cannot find config manager from file :%v", event.Name)
				return nil
			}
		}

		mng.DeleteConfig(event.Name)

		// then reload config
		return mng.LoadConfig(event.Name)

	case event.Op&fsnotify.Rename == fsnotify.Rename, event.Op&fsnotify.Remove == fsnotify.Remove:
		f := filepath.Dir(event.Name)
		mng, ok := m.configMangerMap[f]
		if !ok {
			glog.Errorf("cannot find config manager from file: %v", event.Name)
			return nil
		}

		if !strings.HasSuffix(event.Name, ".json") && !strings.HasSuffix(event.Name, ".yml") {
			glog.Infof("expect file name ends with '.json' or '.yml', got %v, ignored", event.Name)
			return nil
		}

		mng.DeleteConfig(event.Name)
		return nil

	case event.Op&fsnotify.Create == fsnotify.Create:
		f := filepath.Dir(event.Name)
		mng, ok := m.configMangerMap[f]
		if !ok {
			glog.Errorf("cannot find config manager from file :%v", event.Name)
			return nil
		}

		s, err := os.Stat(event.Name)
		if err != nil {
			return err
		}

		// if created a new directory
		if s.IsDir() {
			m.configMangerMap[event.Name] = mng
			err = watcher.Add(event.Name)
			glog.V(4).Infof("add a new folder to watch: %v", event.Name)
			if err != nil {
				glog.Warningf("add watch failed: %v", err)
			}
			return err
		}

		if !strings.HasSuffix(event.Name, ".json") && !strings.HasSuffix(event.Name, ".yml") {
			glog.Infof("expect file name ends with '.json' or '.yml', got %v, ignored", event.Name)
			return nil
		}

		// then reload config
		return mng.LoadConfig(event.Name)
	}

	return nil
}
