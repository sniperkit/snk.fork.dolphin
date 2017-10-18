package hostinfo

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/hashicorp/go-multierror"
	"github.com/pborman/uuid"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"we.com/dolphin/types"
)

const (
	machineIDFile = "/etc/machine-id"
)

var hostInfo = HostInfo{
	Annotations: map[string]string{},
	Labels:      map[string]string{},
}

// HostInfo static info of this host, which do not update  frequently
type HostInfo struct {
	lock        sync.RWMutex
	HostID      types.HostID
	HostName    string
	Annotations map[string]string
	Labels      map[string]string
	IPs         map[string]string
	NumOfCPUs   int
	Memory      uint64
	SwapSize    uint64
	Stage       types.Stage
	DiskStat    map[string]*types.DiskStat
}

// GetIPs return ips
func GetIPs() map[string]string {
	ret := map[string]string{}
	for k, v := range hostInfo.IPs {
		ret[k] = v
	}
	return ret
}

// GetMemory returns total memory size
func GetMemory() uint64 {
	return hostInfo.Memory
}

// GetNumOfCPUs get num of cpus
func GetNumOfCPUs() int {
	return hostInfo.NumOfCPUs
}

// GetSwapSize return size of swap
func GetSwapSize() uint64 {
	return hostInfo.SwapSize
}

// GetLabels return labels of this host
func GetLabels() map[string]string {
	ret := map[string]string{}
	hostInfo.lock.RLock()
	defer hostInfo.lock.RUnlock()
	for k, v := range hostInfo.Labels {
		ret[k] = v
	}
	return ret
}

// GetAnnotations get annotations of this host
func GetAnnotations() map[string]string {
	ret := map[string]string{}
	hostInfo.lock.RLock()
	defer hostInfo.lock.RUnlock()
	for k, v := range hostInfo.Annotations {
		ret[k] = v
	}

	return ret
}

// AddAnnotation  add or update an annotation
func AddAnnotation(k, v string) {
	hostInfo.lock.Lock()
	defer hostInfo.lock.Unlock()
	hostInfo.Annotations[k] = v
}

// DelAnnotatioin remotes an annotation
func DelAnnotatioin(k string) {
	hostInfo.lock.Lock()
	defer hostInfo.lock.Unlock()
	delete(hostInfo.Annotations, k)
}

// GetHostName return hostname
func GetHostName() string {
	return hostInfo.HostName
}

// GetHostID return hostID
func GetHostID() types.HostID {
	return hostInfo.HostID
}

// UpdateStage update stage of this host
// this may be usful when a new agent is install on an host which has no ENV set
func UpdateStage(stage types.Stage) {
	// first try to write
	hostInfo.lock.Lock()
	defer hostInfo.lock.Unlock()

	err := overWriteStage(stage)
	if err != nil {
		glog.Fatal(err)
	}

	hostInfo.Stage = stage
}

// GetStage return stage of current host
func GetStage() types.Stage {
	return hostInfo.Stage
}

// GetDiskStat return mounted disk partations, and there total size
// other fields of DiskStat is left empty
func GetDiskStat() map[string]*types.DiskStat {
	ret := map[string]*types.DiskStat{}
	hostInfo.lock.RLock()
	defer hostInfo.lock.RUnlock()

	for k, v := range hostInfo.DiskStat {
		ds := *v
		ret[k] = &ds
	}

	return ret
}

// overWriteStage info store in machineID file
// caller should hold the lock
func overWriteStage(stage types.Stage) error {
	prefix := "stage="
	// first try read stage info from machineID file
	content, err := ioutil.ReadFile(machineIDFile)
	if err != nil {
		glog.Errorf("over write stage: %v", err)
		return err
	}

	newLines := []string{}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			continue
		}
		newLines = append(newLines, line)
	}

	newLines = append(newLines, fmt.Sprintf("%v%v", prefix, stage))

	contentStr := strings.Join(newLines, "\n")

	// write new content to file
	if err = ioutil.WriteFile(machineIDFile, ([]byte)(contentStr), 0644); err != nil {
		glog.Errorf("err: %v", err)
	}

	return err
}

func getDiskStat() (map[string]*types.DiskStat, error) {
	partitions, err := disk.Partitions(true)
	if err != nil {
		err := fmt.Errorf("get disk partitions lisk failed: %v", err)
		glog.Warningf("%v", err)
		return nil, err
	}
	valid := map[string]string{}
	for _, p := range partitions {

		realPath, err := filepath.EvalSymlinks(p.Device)
		if err != nil {
			realPath = p.Device
		}

		parts := strings.Split(realPath, "/")
		if len(parts) > 2 && parts[1] == "dev" {
			valid[strings.TrimPrefix(realPath, "/dev/")] = p.Mountpoint
		}
	}

	diskStats := map[string]*types.DiskStat{}
	var merr *multierror.Error
	for dev, mnt := range valid {
		usage, err := disk.Usage(mnt)
		if err != nil {
			merr = multierror.Append(merr, err)
			continue
		}
		diskStat := types.DiskStat{
			Totoal:     usage.Total,
			Mountpoint: mnt,
			Devices:    dev,
		}

		diskStats[dev] = &diskStat
	}

	return diskStats, merr.ErrorOrNil()
}

// getEnv assumes machineID file already exists on disk
func getEnv() (types.Stage, error) {
	hostInfo.lock.Lock()
	defer hostInfo.lock.Unlock()

	prefix := "stage="
	envStr := ""
	// first try read stage info from machineID file
	content, err := ioutil.ReadFile(machineIDFile)
	if err != nil {
		glog.Errorf("get stage: %v", err)
		return types.UnknownStage, err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			envStr = strings.TrimPrefix(line, prefix)
			return types.ParseStage(envStr)
		}
	}

	// did not find stage from file, try get stage from ENV
	envStr = os.Getenv("ENV")
	if envStr != "" {
		contentStr := strings.TrimSpace(string(content))
		//
		if err = ioutil.WriteFile(machineIDFile, ([]byte)(fmt.Sprintf("%v\n%v%v", contentStr, prefix, envStr)), 0644); err != nil {
			glog.Errorf("err: %v", err)
		}

		return types.ParseStage(envStr)
	}

	return types.UnknownStage, nil
}

// getHostID generate host is not exist, return hostid and err if have one
func getHostID(generate bool) (string, error) {
	hostInfo.lock.Lock()
	defer hostInfo.lock.Unlock()
	hostID := ""
	prefix := "hostID="
	content, err := ioutil.ReadFile(machineIDFile)

	if err != nil && generate {
		tmphostID := uuid.New()
		if err = ioutil.WriteFile(machineIDFile, ([]byte)(fmt.Sprintf("%v%v", prefix, tmphostID)), 0644); err == nil {
			hostID = tmphostID
		}
	} else {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, prefix) {
				hostID = strings.TrimPrefix(line, prefix)
				break
			}
		}

		// for compatible with already exist files which only contains hostid
		if hostID == "" && len(content) > 0 {
			hostID = strings.TrimSpace(string(content))
			fmt.Printf("change content of machineID file from %v, to %v%v", hostID, prefix, hostID)
			if err = ioutil.WriteFile(machineIDFile, ([]byte)(fmt.Sprintf("%v%v", prefix, hostID)), 0644); err != nil {
				fmt.Printf("err: %v", err)
			}
		}

		vuid := uuid.Parse(hostID)
		if vuid != nil {
			hostID = vuid.String()
		} else {
			err = fmt.Errorf("invalid hostid %s read from %s", string(hostID), machineIDFile)
		}
	}

	return hostID, err
}

func getHostIPs() (map[string]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	ips := map[string]string{}

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		// handle err
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP.To4()
				if ip != nil {
					ips[i.Name] = ip.String()
				}
			default:
				continue
			}
			// process IP address
		}
	}

	return ips, nil
}

func ipExists(ip string, ips map[string]string) bool {
	for _, v := range ips {
		if ip == v {
			return true
		}
	}
	return false
}

// if ipRegx is not empty and return the ip matched, if no match return the reason, if more than one matched, return error
// search nic name sequentially, return the first match, if no match return reson

func getHostIPConfig(ips map[string]string, ipRegexp string, nics []string) (string, error) {
	// if ipregexp set, match  against the found ips
	var ip string
	if ipRegexp != "" {
		ipregexp := regexp.MustCompile(ipRegexp)
		for _, v := range ips {
			if ipregexp.MatchString(v) {
				if ip == "" {
					ip = v
				} else {
					return "", fmt.Errorf("more than one ip matched the provided ipregexp '%s': '%s','%s'", ipRegexp, ip, v)
				}
			}
		}
		if ip == "" {
			return "", fmt.Errorf("no ip match the configed ipregexp: '%s'", ipRegexp)
		}

		return ip, nil
	}

	// find ip according the the nic names provided
	for _, nic := range nics {
		if ip, ok := ips[nic]; ok && len(ip) > 0 {
			return ip, nil
		}
	}

	return ip, fmt.Errorf("could not find an ip matche the configuration:\n ips: %+v, ipRegexp: %v, nics: %v", ips, ipRegexp, nics)
}

const (
	defaultEtcdConfigFile = "/etc/telegraf/etcd.yml"
)

var (
	etcdConfigFile = ""
)

var (
	internalIPRegexp = regexp.MustCompile(`^(10|192\.168|172\.(1[6-9]|2[0-9]|3[01]))\.`)
)

// GetInternalIP returns an internal ip, if has no internal ip except loopback, return empty
// if has more then one internal ip, return the first meet, except loopback
func GetInternalIP() string {
	hostInfo.lock.RLock()
	defer hostInfo.lock.RUnlock()
	ips := hostInfo.IPs

	for _, ip := range ips {
		if internalIPRegexp.MatchString(ip) {
			return ip
		}
	}
	return ""
}

// GetExternalIPS return ExternalIPS
func GetExternalIPS() []string {
	ret := []string{}
	hostInfo.lock.RLock()
	defer hostInfo.lock.RUnlock()
	ips := hostInfo.IPs

	for _, ip := range ips {
		if internalIPRegexp.MatchString(ip) {
			continue
		}
		if strings.HasPrefix(ip, "127.0.0") {
			continue
		}

		ret = append(ret, ip)
	}
	return ret
}

// Init gether host info, can cal host annotations, tags, host status
// these should block, so after Init, others can call get hostname of this hosts
func gatherHostInfo() (*HostInfo, error) {
	var merr *multierror.Error
	hostid, err := getHostID(true)
	if err != nil {
		err := fmt.Errorf("get hostID error %v", err)
		merr = multierror.Append(merr, err)
	}

	ips, err := getHostIPs()
	if err != nil {
		err := fmt.Errorf("get host ip list error: %v", err)
		merr = multierror.Append(merr, err)
	}

	cpus, err := cpu.Counts(true)
	if err != nil {
		err := fmt.Errorf("error get num of cpus: %v", err)
		merr = multierror.Append(merr, err)
	}

	var memSize, swap uint64
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		err := fmt.Errorf("error get mem stat: %v", err)
		merr = multierror.Append(merr, err)
	} else {
		memSize = memInfo.Total
	}

	swapInfo, err := mem.SwapMemory()
	if err != nil {
		err := fmt.Errorf("error get swap stat: %v", err)
		merr = multierror.Append(merr, err)
	} else {
		swap = swapInfo.Total
	}

	hostname, err := os.Hostname()
	if err != nil {
		err := fmt.Errorf("get hostname err: %v", err)
		merr = multierror.Append(merr, err)
	}

	stage, err := getEnv()
	if err != nil {
		err = fmt.Errorf("get stage info: %v", err)
		merr = multierror.Append(merr, err)
	}

	diskStat, err := getDiskStat()
	if err != nil {
		err = fmt.Errorf("get disk stat: %v", err)
		merr = multierror.Append(merr, err)
	}
	hostinfo := HostInfo{
		HostID:    types.HostID(hostid),
		HostName:  hostname,
		IPs:       ips,
		NumOfCPUs: cpus,
		Memory:    memSize,
		SwapSize:  swap,
		Stage:     stage,
		DiskStat:  diskStat,
	}

	return &hostinfo, merr.ErrorOrNil()
}

func init() {
	hi, err := gatherHostInfo()

	if err != nil {
		log.Fatalf("get hostinfo error: %v", err)
	}
	copyHostInfo(&hostInfo, hi)
}

// copyHostInfos from b to a
func copyHostInfo(dst, src *HostInfo) {
	if dst == nil || src == nil {
		return
	}
	dst.lock.Lock()
	defer dst.lock.Unlock()

	src.lock.Lock()
	defer src.lock.Unlock()

	dst.Stage = src.Stage
	dst.HostID = src.HostID
	dst.HostName = src.HostName
	dst.Memory = src.Memory
	dst.NumOfCPUs = src.NumOfCPUs
	dst.SwapSize = src.SwapSize
	dst.IPs = src.IPs
	dst.DiskStat = src.DiskStat
}
