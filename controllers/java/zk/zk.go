/*
Sniperkit-Bot
- Status: analyzed
*/

package zk

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/golang/glog"
	zk "github.com/samuel/go-zookeeper/zk"
)

// Client provides a wrapper around the zookeeper client
type Client struct {
	client *zk.Conn
}

func (c *Client) Close() {
	if c != nil && c.client != nil {
		c.client.Close()
	}
}

func NewClient(machines []string) (*Client, error) {
	c, _, err := zk.Connect(machines, time.Second) //*10)
	if err != nil {
		panic(err)
	}
	b, s, err := c.Get("/service/com.dc-item/1_5")
	if err != nil {
		panic(err)
	}
	glog.Infof("%v: %v", b, s)
	return &Client{c}, nil
}

func nodeWalk(prefix string, c *Client, pathMatcher *regexp.Regexp,
	keysOnly bool, vars map[string][]byte) error {
	l, stat, err := c.client.Children(prefix)
	if err != nil {
		return err
	}

	if stat.NumChildren == 0 {
		if pathMatcher != nil && !pathMatcher.MatchString(prefix) {
			return nil
		}
		var d []byte
		if !keysOnly {
			b, _, err := c.client.Get(prefix)
			if err != nil {
				return err
			}
			d = b
		}
		vars[prefix] = d
	}

	if stat.NumChildren > 0 {
		var d []byte
		if !keysOnly {
			b, _, err := c.client.Get(prefix)
			if err != nil {
				return err
			}
			d = b
		}
		vars[prefix] = d
		for _, key := range l {
			var s string
			if strings.HasSuffix(prefix, "/") {
				s = prefix + key
			} else {
				s = prefix + "/" + key
			}

			nodeWalk(s, c, pathMatcher, keysOnly, vars)
		}
	}
	return nil
}

// GetValues return subtree of these nodes
func (c *Client) GetValues(keys []string, re *regexp.Regexp, keysOnly bool) (map[string][]byte, error) {
	vars := make(map[string][]byte)
	for _, key := range keys {
		err := nodeWalk(key, c, re, keysOnly, vars)
		if err != nil {
			return vars, err
		}
	}
	return vars, nil
}

// SetNodeValue set node value to value
func (c *Client) SetNodeValue(key string, value string) error {
	var err error
	key = strings.Replace(key, "/*", "", -1)
	_, _, err = c.client.Exists(key)
	if err != nil {
		return err
	}

	_, err = c.client.Set(key, ([]byte)(value), -1)

	if err != nil {
		return err
	}

	return nil
}

// GetNodeValue get node value
func (c *Client) GetNodeValue(key string) (string, error) {
	var err error
	var b []byte
	_, _, err = c.client.Exists(key)
	if err != nil {
		return "", err
	}

	b, _, err = c.client.Get(key)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// GetNodesValues get nodes values
func (c *Client) GetNodesValues(keys []string) (map[string]string, error) {
	vars := make(map[string]string)
	for _, v := range keys {
		nv := strings.Replace(v, "/*", "", -1)
		_, _, err := c.client.Exists(nv)
		if err != nil {
			return vars, err
		}

		b, _, err := c.client.Get(nv)
		if err != nil {
			return vars, err
		}
		vars[v] = string(b)
	}
	return vars, nil
}

// WatchPrefix watches  path and subpath match pathMatcher for changes
// if a node a does not match pathMatcher when we start to watch, then we add a new node b which matches pathmatcher,
// node b will not watched
// a nil pathMatcher will match anything
func (c *Client) WatchPrefix(ctx context.Context, path string, pathMatcher *regexp.Regexp, ech chan<- zk.Event) {
	if c == nil {
		glog.Info("zk: client is nil")
		return
	}

	c.nodeWalkW(ctx, path, pathMatcher, ech)
}

// https://zookeeper.apache.org/doc/trunk/zookeeperProgrammers.html
// We can set watches with the three calls that read the state of ZooKeeper: exists, getData,
// and getChildren. The following list details the events that a watch can trigger and the calls that enable them:
// Created event:
//     Enabled with a call to exists.
// Deleted event:
//     Enabled with a call to exists, getData, and getChildren.
// Changed event:
//     Enabled with a call to exists and getData.
// Child event:
//     Enabled with a call to getChildren.

func (c *Client) nodeWalkW(ctx context.Context, prefix string, pathMatcher *regexp.Regexp, ech chan<- zk.Event) {
	var err error
	defer func() {
		if err != nil {
			ech <- zk.Event{
				Path: prefix,
				Err:  err,
			}
		}
	}()

	l, stat, childech, err := c.client.ChildrenW(prefix)
	if err != nil {
		return
	}

	var dataC <-chan zk.Event
	if pathMatcher == nil || pathMatcher.MatchString(prefix) {
		_, _, datac, err := c.client.GetW(prefix)
		if err != nil {
			glog.Errorf("zk: getdate %v: %v", prefix, err)
		}

		if datac != nil {
			dataC = datac
		}
	}

	if stat.NumChildren > 0 {
		for _, key := range l {
			var s string
			if strings.HasSuffix(prefix, "/") {
				s = prefix + key
			} else {
				s = prefix + "/" + key
			}

			c.nodeWalkW(ctx, s, pathMatcher, ech)
		}
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-dataC:
				ech <- event

				if event.Type == zk.EventNodeDeleted {
					return
				}

				if _, _, dataC, err = c.client.GetW(prefix); err != nil {
					return
				}

			case event := <-childech:
				ech <- event
				if event.Type == zk.EventNodeDeleted {
					return
				}

				if event.Type == zk.EventNodeChildrenChanged {
					c.nodeWalkW(ctx, prefix, pathMatcher, ech)
					return
				}

				if _, _, childech, err = c.client.ChildrenW(prefix); err != nil {
					return
				}
			}
		}
	}()
}
