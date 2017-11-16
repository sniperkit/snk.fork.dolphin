package main

import (
	"github.com/coreos/etcd/clientv3"
	"we.com/dolphin/controllers/java/zk/types"
	"we.com/dolphin/report"
)

type config struct {
	Etcd     clientv3.Client  `json:"etcd,omitempty"`
	InfluxDB *report.InfluxDB `json:"influxDB,omitempty"`
	ZKs      types.Config     `json:"zKs,omitempty"`
}
