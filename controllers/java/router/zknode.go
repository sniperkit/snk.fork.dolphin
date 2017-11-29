package router

import (
	"encoding/json"
	"time"
)

/*
{"address":"10.10.10.59","type":1,"port":40341,"startTime":"2017-11-13 18:03:19","mainclass":"com.to8to.weixin.server.WeixinServer","pid":27417,"reconnectZK":0,"version":"97"}

{"address":"10.10.10.30","type":1,"port":40364,"time":"2017-06-22 00:21:26"}

{"type":3,"port":0,"time":"2017-07-17 09:24:42"}

{"pid":13943,"version":"7","bind_ip":"0.0.0.0","report_ip":"10.10.10.82","port":40080,"start_time":"2017-09-22 19:07:05","type":1,"method":["views.contractBill.generate","contractBill.query","accountItem.findById","views.contractItem.queryPage","accountItem.findByIds","contractBill.update","views.accountItem.getAccountItem","contractBill.findById","contractBill.create","contractBill.deleteByIds","views.contractBill.queryPage","contractBill.findByIds","views.contractBill.getContractAndItem","contractBill.deleteById","views.contractBill.getDetail","contractItem.query","accountItem.query","views.accountItem.queryPage","contractItem.findByIds","contractItem.findById"]}

{"pid":43024,"version":"7","report_ip":"10.10.10.51","start_time":"2017-10-28 15:35:53","type":0}
*/

type ServiceNode struct {
	APIVersion  string    `json:"apiVersion,omitempty"`
	NodeName    string    `json:"nodeName,omitempty"`
	Host        string    `json:"address,omitempty"`
	Type        int       `json:"type,omitempty"`
	Port        int       `json:"port,omitempty"`
	StartTime   time.Time `json:"startTime,omitempty"`
	MainClass   string    `json:"mainclass,omitempty"`
	Pid         int       `json:"pid,omitempty"`
	ReconnectZK int       `json:"reconnectZK,omitempty"`
	Version     string    `json:"version,omitempty"`
}

// UnmarshalJSON implements json interface
func (sn *ServiceNode) UnmarshalJSON(b []byte) error {
	var o struct {
		APIVersion  string `json:"apiVersion,omitempty"`
		Host        string `json:"address"`
		IP          string `json:"report_ip"`
		Type        int    `json:"type"`
		Port        int    `json:"port"`
		StartTime   string `json:"startTime"`
		Starttime   string `json:"start_time"`
		Time        string `json:"time"`
		MainClass   string `json:"mainclass"`
		Pid         int    `json:"pid"`
		ReconnectZK int    `json:"reconnectZK"`
		Version     string `json:"version"`
	}

	err := json.Unmarshal(b, &o)
	if err != nil {
		return err
	}

	if o.StartTime == "" && o.Time != "" {
		o.StartTime = o.Time
	}

	if o.StartTime == "" && o.Starttime != "" {
		o.StartTime = o.Starttime
	}

	if o.StartTime != "" {
		t, err := time.Parse("2006-01-02 15:04:05", o.StartTime)
		if err != nil {
			return err
		}

		sn.StartTime = t.Add(-8 * time.Hour)
	}

	if o.Host != "" {
		sn.Host = o.Host
	} else {
		sn.Host = o.IP
	}
	sn.Type = o.Type
	sn.Port = o.Port
	sn.MainClass = o.MainClass
	sn.Pid = o.Pid
	sn.Version = o.Version
	sn.ReconnectZK = o.ReconnectZK

	return nil
}
