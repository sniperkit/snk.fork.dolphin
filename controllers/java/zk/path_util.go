package zk

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

const (
	zooKeeperBase = "/mnt/zk"
)

// 当前仅同步： 路由，实例， 和版本 相关的信息
// 其他的先不考虑， 路由和实例有两种不同实况，分别对就原来的paltform和现在的rpc两种不同的框架
// 这里，我们应当对他们有所区分
// getZKpath是一个一一映射，与 getEtcdPath互逆， 即 a = getZKPath(getEtcdPath(a))
func getEtcdPath(env string, zkPath string) (string, error) {
	if len(zkPath) == 0 {
		return "", errors.New("zkpath not allow empty")
	}

	var path string
	var err error
	if strings.HasPrefix(zkPath, "/biz/") {
		path, err = parseZKPathv4(zkPath)
	} else {
		path, err = parseZKPathv2(zkPath)
	}

	if err != nil {
		return "", err
	}

	return filepath.ToSlash(filepath.Join(zooKeeperBase, string(env), path)), nil
}

func getZKPath(etcdPath string) (string, error) {
	prefix := zooKeeperBase
	if !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	if !strings.HasPrefix(etcdPath, prefix) {
		return "", errors.Errorf("invalid mapped zk  etcdPath: %v", etcdPath)
	}

	path := strings.TrimPrefix(etcdPath, prefix)

	// strip env parts
	idx := strings.Index(path, "/")
	if idx > 0 && idx+1 < len(path) {
		path = path[idx+1:]
	}

	return getZKPath0(path)
}

// {typ}/{version}/{cluster}/....
func getZKPath0(strippedEtcdPath string) (string, error) {
	path := strippedEtcdPath

	parts := strings.Split(path, "/")

	if len(parts) < 3 {
		return "", errors.Errorf("invalid etcdPath")
	}

	typ := parts[0]
	version := parts[1]
	cluster := parts[2]

	other := ""

	if len(parts) > 3 {
		other = strings.Join(parts[3:], "/")
	}

	if typ == "instance" {
		return "", fmt.Errorf("currently, service is not allowed to sync to zk")
	}

	if typ != "config" && typ != "route" {
		return "", errors.Errorf("unknow etcdPath type: %v", strippedEtcdPath)
	}

	if version == "2" {
		if typ == "route" {
			if other != "route" {
				return "", errors.Errorf("invalid etcdPath of service type of v2, %v", strippedEtcdPath)
			}

			return filepath.ToSlash(filepath.Join("/", "service", cluster)), nil
		}

		ret := filepath.ToSlash(filepath.Join("/", typ, cluster, other))
		return ret, nil
	}

	if version == "4" {
		idx := strings.LastIndex(cluster, ".")
		if idx < 0 || idx+1 == len(cluster) {
			return "", errors.Errorf("invalid cluster name: %v", strippedEtcdPath)
		}
		biz := cluster[:idx]
		app := cluster[idx+1:]

		if typ == "route" {
			return filepath.ToSlash(filepath.Join("/biz", biz, app, "policy/", other)), nil
		}

		if typ == "config" {
			return filepath.ToSlash(filepath.Join("/biz", biz, app, "config/", other)), nil
		}
	}

	return "", errors.Errorf("unknow version: %v", strippedEtcdPath)
}

func parseZKPathv2(zkPath string) (string, error) {
	parts := strings.Split(zkPath, "/")

	if len(parts) < 3 {
		err := errors.New("zkpath is not valid")
		return "", err
	}

	typ := parts[1]

	if typ != "config" && typ != "service" {
		return "", errors.Errorf("unknown zkpath: %v", zkPath)
	}

	if typ == "service" {
		if len(parts) == 3 {
			typ = "route"
			parts = append(parts, "route")
		} else {
			typ = "instance"
		}
	}

	tmp := make([]string, 0, len(parts)+2)
	tmp = append(tmp, typ, "2")
	others := parts[2:]
	tmp = append(tmp, others...)

	return filepath.ToSlash(filepath.Join(tmp...)), nil
}

func parseZKPathv4(zkPath string) (string, error) {
	if !strings.HasPrefix(zkPath, "/biz/") {
		return "", errors.Errorf("invalid it4.0 zkpath, %v", zkPath)
	}

	path := strings.TrimPrefix(zkPath, "/biz/")

	parts := strings.Split(path, "/")

	if len(parts) < 3 {
		return "", errors.Errorf("uncomplete  it4.0 zkPath, %v", zkPath)
	}

	typ := parts[2]
	if typ == "daemon" {
		typ = "instance"
	} else if typ == "policy" {
		typ = "route"
	}

	if typ != "route" && typ != "config" && typ != "instance" {
		return "", errors.Errorf("cannot parse it4.0 zkpath: %v", zkPath)
	}

	service := strings.Join(parts[0:2], ".")

	tmp := make([]string, 0, len(parts)+2)
	tmp = append(tmp, typ, "4", service)
	tmp = append(tmp, parts[3:]...)

	return filepath.ToSlash(filepath.Join(tmp...)), nil
}