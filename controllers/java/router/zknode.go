package router

import (
	"encoding/json"
	"time"
)

type ServiceNode struct {
	Host        string    `json:"address"`
	Type        int       `json:"type"`
	Port        int       `json:"port"`
	StartTime   time.Time `json:"startTime"`
	MainClass   string    `json:"mainclass"`
	Pid         int       `json:"pid"`
	ReconnectZK int       `json:"reconnectZK"`
	Version     string    `json:"version"`
	Methods     []string  `json:"method"`
}

// UnmarshalJSON implements json interface
func (sn *ServiceNode) UnmarshalJSON(b []byte) error {
	var o struct {
		Host        string   `json:"address"`
		IP          string   `json:"report_ip"`
		Type        int      `json:"type"`
		Port        int      `json:"port"`
		StartTime   string   `json:"startTime"`
		Starttime   string   `json:"start_time"`
		Time        string   `json:"time"`
		MainClass   string   `json:"mainclass"`
		Pid         int      `json:"pid"`
		ReconnectZK int      `json:"reconnectZK"`
		Version     string   `json:"version"`
		Methods     []string `json:"method"`
		// Catches all undefined fields and must be empty after parsing.
		XXX map[string]interface{} `json:",inline"`
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
	sn.Methods = o.Methods

	return nil
}
