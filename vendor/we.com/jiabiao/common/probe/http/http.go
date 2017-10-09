package http

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	utilnet "we.com/jiabiao/common/net"
	"we.com/jiabiao/common/probe"

	"github.com/golang/glog"
)

func New() HTTPProber {
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	transport := utilnet.SetTransportDefaults(&http.Transport{TLSClientConfig: tlsConfig, DisableKeepAlives: true})
	return httpProber{transport}
}

type HTTPProber interface {
	Probe(url *url.URL, headers http.Header, timeout time.Duration) (probe.Result, string, error)
}

type httpProber struct {
	transport *http.Transport
}

// Probe returns a ProbeRunner capable of running an http check.
func (pr httpProber) Probe(url *url.URL, headers http.Header, timeout time.Duration) (probe.Result, string, error) {
	return DoHTTPProbe(url, headers, &http.Client{Timeout: timeout, Transport: pr.transport})
}

type HTTPGetInterface interface {
	Do(req *http.Request) (*http.Response, error)
}

// DoHTTPProbe checks if a GET request to the url succeeds.
// If the HTTP response code is successful (i.e. 400 > code >= 200), it returns Success.
// If the HTTP response code is unsuccessful or HTTP communication fails, it returns Failure.
// This is exported because some other packages may want to do direct HTTP probes.
func DoHTTPProbe(url *url.URL, headers http.Header, client HTTPGetInterface) (probe.Result, string, error) {
	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		// Convert errors into failures to catch timeouts.
		return probe.Failure, err.Error(), nil
	}
	req.Header = headers
	if headers.Get("Host") != "" {
		req.Host = headers.Get("Host")
	}
	res, err := client.Do(req)
	if err != nil {
		// Convert errors into failures to catch timeouts.
		return probe.Failure, err.Error(), nil
	}
	defer res.Body.Close()
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return probe.Failure, "", err
	}
	body := string(b)
	if res.StatusCode >= http.StatusOK && res.StatusCode < http.StatusBadRequest {
		glog.V(4).Infof("Probe succeeded for %s, Response: %v", url.String(), *res)
		return probe.Success, body, nil
	}
	glog.V(4).Infof("Probe failed for %s with request headers %v, response body: %v", url.String(), headers, body)
	return probe.Failure, fmt.Sprintf("HTTP probe failed with statuscode: %d", res.StatusCode), nil
}