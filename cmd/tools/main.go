/*
Sniperkit-Bot
- Status: analyzed
*/

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/pkg/errors"
	"we.com/dolphin/controllers/java/project"
	"we.com/dolphin/registry/etcdkey"
	"we.com/dolphin/registry/generic"
	"we.com/jiabiao/common/yaml"
)

/*
desc: "crm service"
service:  "com.crm"
git_repo: "git@repo.we.com:java/crm-server.git"
owner: "jason.yu"
bins:
    - 1
    - 7
    - jobs
# 注意要使用yaml的格式， 不能使用tab, 要用空格
# 至少要有三个拨测接口
interfaces:
    listSourceDetail:
        desc:  "listSourceDetail"
        data: '{"service":"crm.utils","interface":"listSourceDetail","args":[]}'
        matches: success
    listRecord:
        desc: listRecord lalalala
        data: '{"service":"crm.bid","interface":"listRecord","args":[1,"57716"]}'
    listBidRecord:
        desc: list  bid record
        data: '{"service":"crm.query","interface":"listBidRecord","args":[{"page_info":{"curr_page":1,"page_size":20}}]}'

*/

type diCfg struct {
	Project string   `json:"project,omitempty"`
	Desc    string   `json:"desc,omitempty"`
	Service string   `json:"service,omitempty"`
	Bins    []string `json:"bins,omitempty"`
}

func (di *diCfg) UnmarshalJSON(dat []byte) error {
	type tmp struct {
		Project string        `json:"project,omitempty"`
		Desc    string        `json:"desc,omitempty"`
		Service string        `json:"service,omitempty"`
		Bins    []interface{} `json:"bins,omitempty"`
	}

	v := tmp{}

	if err := json.Unmarshal(dat, &v); err != nil {
		return err
	}

	di.Project = v.Project
	di.Desc = v.Desc
	di.Service = v.Service

	for _, b := range v.Bins {
		switch t := b.(type) {
		case float64:
			di.Bins = append(di.Bins, fmt.Sprintf("%d", int(t)))
		case string:
			di.Bins = append(di.Bins, t)
		default:
			return errors.Errorf("unknown bin types:(%T, %v)", t, t)
		}
	}

	return nil
}

func (di diCfg) ToInfo() project.Info {

	tmp := project.Info{
		Project:     di.Project,
		Desc:        di.Desc,
		ServiceName: di.Service,
	}

	if di.Project == di.Service {
		tmp.APIVersion = "4.0"
	} else {
		tmp.APIVersion = "2.0"
	}
	return tmp
}

var (
	diPath  = flag.String("diPath", "", "dialinterface path")
	dstPath = flag.String("dst", "", "path where to store infos")
)

func main() {
	flag.Parse()

	if *diPath == "" {
		fmt.Fprintln(os.Stderr, "dialinterface path cannot be empty")
		return
	}

	if *dstPath == "" {
		fmt.Fprintln(os.Stderr, "dst path cannot be empty")
		return
	}

	glob := fmt.Sprintf("%v/*.yml", *diPath)

	files, err := filepath.Glob(glob)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list config file err: %v\n", err)
		return
	}

	var infos []project.Info

	for _, v := range files {
		content, err := ioutil.ReadFile(v)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v: %v\n", v, err)
			continue
		}

		reader := bytes.NewReader(content)

		decode := yaml.NewYAMLOrJSONDecoder(reader, 4)
		cfg := diCfg{}

		if err := decode.Decode(&cfg); err != nil {
			fmt.Fprintf(os.Stderr, "decode err: %v, %v\n", v, err)
		}

		f := filepath.Base(v)
		cfg.Project = strings.TrimSuffix(f, ".yml")

		infos = append(infos, cfg.ToInfo())
	}

	/*
			if err := os.MkdirAll(*dstPath, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "create dst dir: %v\n", err)
			return
		}
			for _, v := range infos {
				name := strings.Replace(string(v.Name), ":", "-", -1)
				fileName := fmt.Sprintf("%v/%v.yml", *dstPath, name)

				dat, _ := json.Marshal(v)
				dat, _ = yaml.ToYAML(dat)

				ioutil.WriteFile(fileName, dat, 0644)
			}
	*/

	generic.SetEtcdConfig(clientv3.Config{
		Endpoints:   []string{"192.168.1.68:2379"},
		DialTimeout: 5 * time.Second,
	})

	base := etcdkey.JavaProjectDir()
	store, err := generic.GetStoreInstance(base, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}

	for _, v := range infos {
		if v.APIVersion != "2.0" {
			continue
		}
		store.Update(context.Background(), v.Project, v, nil, 0)
	}

	fmt.Printf("success\n")
}
