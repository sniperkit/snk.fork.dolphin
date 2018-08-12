/*
Sniperkit-Bot
- Status: analyzed
*/

package report

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strings"
	"time"

	"github.com/golang/glog"
	"we.com/dolphin/report/influx"
	"we.com/dolphin/report/metric"
	mytime "we.com/jiabiao/common/time"
)

var (
	// Quote Ident replacer.
	qiReplacer = strings.NewReplacer("\n", `\n`, `\`, `\\`, `"`, `\"`)
)

func getTLSConfig(
	SSLCert, SSLKey, SSLCA string,
	InsecureSkipVerify bool,
) (*tls.Config, error) {
	if SSLCert == "" && SSLKey == "" && SSLCA == "" && !InsecureSkipVerify {
		return nil, nil
	}

	t := &tls.Config{
		InsecureSkipVerify: InsecureSkipVerify,
	}

	if SSLCA != "" {
		caCert, err := ioutil.ReadFile(SSLCA)
		if err != nil {
			return nil, fmt.Errorf("Could not load TLS CA: %s",
				err)
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		t.RootCAs = caCertPool
	}

	if SSLCert != "" && SSLKey != "" {
		cert, err := tls.LoadX509KeyPair(SSLCert, SSLKey)
		if err != nil {
			return nil, fmt.Errorf(
				"Could not load TLS client key/certificate from %s:%s: %s",
				SSLKey, SSLCert, err)
		}

		t.Certificates = []tls.Certificate{cert}
		t.BuildNameToCertificate()
	}

	// will be nil by default if nothing is provided
	return t, nil
}

// InfluxDB struct is the primary data structure for the plugin
type InfluxDB struct {
	URLs             []string        `json:"urls,omitempty"`
	Username         string          `json:"username,omitempty"`
	Password         string          `json:"password,omitempty"`
	Database         string          `json:"database,omitempty"`
	UserAgent        string          `json:"userAgent,omitempty"`
	RetentionPolicy  string          `json:"retentionPolicy,omitempty"`
	WriteConsistency string          `json:"writeConsistency,omitempty"`
	Timeout          mytime.Duration `json:"timeout,omitempty"`
	UDPPayload       int             `json:"udpPayload,omitempty"`
	HTTPProxy        string          `json:"httpProxy,omitempty"`

	// Path to CA file
	SSLCA string `json:"sslca,omitempty"`
	// Path to host cert file
	SSLCert string `json:"sslCert,omitempty"`
	// Path to cert key file
	SSLKey string `json:"sslKey,omitempty"`
	// Use SSL but skip chain & host verification
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`

	// Precision is only here for legacy support. It will be ignored.
	Precision string `json:"precision,omitempty"`

	clients []influx.Client
}

// Connect initiates the primary connection to the range of provided URLs
func (i *InfluxDB) Connect() error {
	var urls []string
	urls = append(urls, i.URLs...)

	tlsConfig, err := getTLSConfig(
		i.SSLCert, i.SSLKey, i.SSLCA, i.InsecureSkipVerify)
	if err != nil {
		return err
	}

	for _, u := range urls {
		switch {

		default:
			// If URL doesn't start with "udp", assume HTTP client
			config := influx.HTTPConfig{
				URL:       u,
				Timeout:   time.Duration(i.Timeout),
				TLSConfig: tlsConfig,
				UserAgent: i.UserAgent,
				Username:  i.Username,
				Password:  i.Password,
				HTTPProxy: i.HTTPProxy,
			}
			wp := influx.WriteParams{
				Database:        i.Database,
				RetentionPolicy: i.RetentionPolicy,
				Consistency:     i.WriteConsistency,
			}
			c, err := influx.NewHTTP(config, wp)
			if err != nil {
				return fmt.Errorf("Error creating HTTP Client [%s]: %s", u, err)
			}
			i.clients = append(i.clients, c)

			err = c.Query(fmt.Sprintf(`CREATE DATABASE "%s"`, qiReplacer.Replace(i.Database)))
			if err != nil {
				if !strings.Contains(err.Error(), "Status Code [403]") {
					glog.Error("Database creation failed: " + err.Error())
				}
				continue
			}
		}
	}

	rand.Seed(time.Now().UnixNano())
	return nil
}

// Close will terminate the session to the backend, returning error if an issue arises
func (i *InfluxDB) Close() error {
	return nil
}

// Write will choose a random server in the cluster to write to until a successful write
// occurs, logging each unsuccessful. If all servers fail, return error.
func (i *InfluxDB) Write(metrics []metric.Metric) error {
	bufsize := 0
	for _, m := range metrics {
		bufsize += m.Len()
	}

	r := metric.NewReader(metrics)

	// This will get set to nil if a successful write occurs
	err := fmt.Errorf("Could not write to any InfluxDB server in cluster")

	p := rand.Perm(len(i.clients))
	for _, n := range p {
		if _, e := i.clients[n].WriteStream(r, bufsize); e != nil {
			// If the database was not found, try to recreate it:
			if strings.Contains(e.Error(), "database not found") {
				errc := i.clients[n].Query(fmt.Sprintf(`CREATE DATABASE "%s"`, qiReplacer.Replace(i.Database)))
				if errc != nil {
					glog.Errorf("Error: Database %s not found and failed to recreate",
						i.Database)
				}
			}
			if strings.Contains(e.Error(), "field type conflict") {
				glog.Errorf("Field type conflict, dropping conflicted points: %s", e)
				// setting err to nil, otherwise we will keep retrying and points
				// w/ conflicting types will get stuck in the buffer forever.
				err = nil
				break
			}
			// Log write failure
			glog.Errorf("InfluxDB Output Error: %s", e)
		} else {
			err = nil
			break
		}
	}

	return err
}
