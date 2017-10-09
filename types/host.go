package types

import (
	"fmt"
	"regexp"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

var (
	hostrex = regexp.MustCompile(`^[a-z0-9A-Z][-a-z0-9A-Z.]*`)
	iprex   = regexp.MustCompile(`^([0-9]{1,3}.){3}[0-9]{1,3}`)

	ErrInvalidHostName = errors.New("types: invalid host name")
)

// HostConfig  host config infos
type HostConfig struct {
	HostName string `json:"hostName,omitempty"`

	Stage           Stage             `json:"stage,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"` // labels are used as selectors
	ReportTags      map[string]string `json:"reportTags,omitempty"`
	Resourcereserve DeployResource    `json:"resourcereserve,omitempty"`
}

// HostCondition  host condition happing
type HostCondition string

const (
	// HostMemoryShortage  host short of memory
	HostMemoryShortage HostCondition = "memoryShortage"
	// HostLoadHigh  load high
	HostLoadHigh HostCondition = "loadHigh"
	// HostNetWidthHigh bandwidth used to much
	HostNetWidthHigh HostCondition = "netWidthHigh"
	// HostThreadHigh host short of pid resource
	HostThreadHigh HostCondition = "threadsToMuch"
	// HostDiskShortage  disk  space
	HostDiskShortage HostCondition = "diskShortage"
	// HostMetaInfoChanged  host meta info changed
	HostMetaInfoChanged HostCondition = "metaInfoChanged"
	// HostHealthy  host is healthy
	HostHealthy HostCondition = "healthy"
)

type HostID UUID

// HostEvent host status changes
type HostEvent struct {
	HostID     HostID
	Conditions []HostCondition
	Msg        string
}

// HostInfo static info of this host, which do not update  frequently
type HostInfo struct {
	HostID   HostID   `json:"hostID,omitempty"`
	HostName HostName `json:"hostname,omitempty"`
	Stage    Stage    `json:"stage,omitempty"`

	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"properties,omitempty"`

	NumOfCPUs int                  `json:"numOfCPUs,omitempty"`
	Memory    uint64               `json:"memory,omitempty"`
	SwapSize  uint64               `json:"swap_size,omitempty"`
	Disk      map[string]*DiskStat `json:"disk,omitempty"`
	LocalIP   string               `json:"localIP,omitempty"`
	OuterIPs  map[string]string    `json:"outerIPs,omitempty"`
	Services  []string             `json:"services,omitempty"`

	UpdateTime time.Time `json:"updateTime,omitempty"`
}

// Validate  check if the hi is valid,  if not err is not nil
func (hi HostInfo) Validate() error {
	var err *multierror.Error

	if terr := hi.HostName.Validate(); err != nil {
		err = multierror.Append(err, terr)
	}

	if hi.Stage > Production || hi.Stage == UnknownStage {
		err = multierror.Append(err, fmt.Errorf("unknown stage: %v", hi.Stage))
	}

	return err.ErrorOrNil()
}

const (
	cpuResource = uint64(1024 * 1024 * 1024)
	gb          = 1024 * 1024 * 1024 * 8
)

// GetResource get total resource in the form of DeployResource
func (hi HostInfo) GetResource() DeployResource {
	diskSpace := uint64(0)
	for _, v := range hi.Disk {
		diskSpace += v.Totoal
	}
	ret := DeployResource{
		Memory:     hi.Memory + hi.SwapSize/2,
		CPU:        uint64(hi.NumOfCPUs) * cpuResource,
		DiskSpace:  diskSpace,
		NetworkIn:  gb,
		NetworkOut: gb,
	}

	return ret
}

// HostName host name
type HostName string

// Validate checks is hostname is valid
func (hn HostName) Validate() error {
	if hostrex.MatchString(string(hn)) {
		return nil
	}

	return ErrInvalidHostName
}

// HostStatus runtime host status
// Tags are set by users, annotations are used internally
type HostStatus struct {
	HostInfo   *HostInfo `json:"hostInfo,omitempty"`
	UpdateTime time.Time `json:"updateTime,omitempty"`

	Load1    float64 `json:"load1,omitempty"`
	Load5    float64 `json:"load5,omitempty"`
	Load15   float64 `json:"load15,omitempty"`
	CPUUsage float64 `json:"cpuUsage,omitempty"`

	FreeMemory   uint64 `json:"freeMemory,omitempty"`
	CachedMemory uint64 `json:"cachedMemory,omitempty"`
	FreeSwap     uint64 `json:"freeSwap,omitempty"`
	Sin          uint64 `json:"sin,omitempty"`
	Sout         uint64 `json:"out,omitempty"`

	BandWidthUsage map[string]NetIOStat `json:"netStat,omitempty"`

	DiskStat map[string]DiskStat `json:"diskStat,omitempty"`

	NumOfThreads   int `json:"numOfThreads,omitempty"`
	NumofProcesses int `json:"numOfProcesses,omitempty"`
}

// DiskStat  disk partition stat
type DiskStat struct {
	Devices    string `json:"devices,omitempty"`
	Mountpoint string `json:"mountpoint,omitempty"`
	Totoal     uint64 `json:"totoal,omitempty"`
	Free       uint64 `json:"free,omitempty"`
	ReadSpeed  uint64 `json:"readSpeed,omitempty"`
	WriteSpeed uint64 `json:"writeSpeed,omitempty"`
	ReadCount  uint64 `json:"readCount,omitempty"`
	WriteCount uint64 `json:"writeCount,omitempty"`
}

// NetIOStat nic net stat
type NetIOStat struct {
	Name        string `json:"name"`        // interface name
	BytesSent   uint64 `json:"bytesSent"`   // number of bytes sent  per second
	BytesRecv   uint64 `json:"bytesRecv"`   // number of bytes received  per second
	PacketsSent uint64 `json:"packetsSent"` // number of packets sent  per second
	PacketsRecv uint64 `json:"packetsRecv"` // number of packets received  per second
}
