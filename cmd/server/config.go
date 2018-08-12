/*
Sniperkit-Bot
- Status: analyzed
*/

package main

import (
	"bytes"
	"encoding/json"

	"github.com/coreos/etcd/clientv3"
	"we.com/dolphin/controllers/java/zk/types"
	"we.com/dolphin/registry/generic"
	"we.com/dolphin/report"
)

type config struct {
	Etcd     clientv3.Config  `json:"etcd,omitempty"`
	InfluxDB *report.InfluxDB `json:"influxDB,omitempty"`
	ZKs      types.Config     `json:"zKs,omitempty"`
}

func (c *config) UnmarshalJSON(dat []byte) error {
	type tmp struct {
		Etcd     interface{}      `json:"etcd,omitempty"`
		InfluxDB *report.InfluxDB `json:"influxDB,omitempty"`
		ZKs      types.Config     `json:"zKs,omitempty"`
	}

	var t tmp

	if err := json.Unmarshal(dat, &t); err != nil {
		return err
	}

	c.InfluxDB = t.InfluxDB
	c.ZKs = t.ZKs

	jstr, err := json.Marshal(t.Etcd)
	if err != nil {
		return err
	}

	cfg, err := generic.NewEtcdConfig(bytes.NewReader(jstr))
	if err != nil {
		return err
	}

	c.Etcd = cfg
	return nil
}
