/*
Sniperkit-Bot
- Status: analyzed
*/

package alert

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
)

var (
	alertEndpoint = "http://alarm.we.com/api/v1/alerts"
)

// Message alert entity
type Message struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

var (
	tr   *http.Transport
	lock sync.RWMutex
)

func getTR() *http.Transport {
	if tr != nil {
		return tr
	}
	lock.Lock()
	defer lock.Unlock()
	if tr != nil {
		return tr
	}

	tr = &http.Transport{DisableKeepAlives: false, MaxIdleConns: 2, ResponseHeaderTimeout: 2 * time.Second}
	return tr
}

// SendAlerts send alerts
func SendAlerts(messages ...Message) error {
	glog.Infof("send %v alerts: %v", len(messages), messages)

	buf, err := json.Marshal(messages)
	reader := bytes.NewReader(buf)
	req, err := http.NewRequest(http.MethodPost, alertEndpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Add("Content-type", "application/json")
	req.Header.Add("User-Agent", "dolphin-agent")

	client := &http.Client{Transport: getTR()}

	resp, err := client.Do(req)
	if err != nil {
		glog.Warningf("send alert failed: %s", err)
	}

	if resp != nil {
		content, _ := ioutil.ReadAll(resp.Body)
		glog.Infof("send alert status code: %s, response content: %q", resp.Status, content)
		resp.Body.Close()
		if bytes.Contains(content, []byte("success")) {
			return nil
		}
		err = errors.Errorf("status: %v, msg: %s", resp.Status, string(content))
	}

	return err
}
