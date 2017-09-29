package ps

import (
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/net"
	"github.com/shirou/gopsutil/process"
	"we.com/dolphin/types"
)

// ProcessState common process state info
type ProcessState struct {
	Type types.ProjectType

	Pid        int
	NumFDs     int
	NumThreads int
	CPUPercent float64
	UpdateTime time.Time

	CtxSwitch process.NumCtxSwitchesStat
	MemInfo   process.MemoryInfoStat
	NetIO     []net.IOCountersStat // currently cannot get per process netio info, just leave this empty
	DiskIO    process.IOCountersStat
	CPUInfo   cpu.TimesStat
}

// CalProcessState get common state and metric of a process
func CalProcessState(proc *process.Process) (ps *ProcessState) {
	if proc == nil {
		return
	}
	var merr *multierror.Error
	numFDs, err := proc.NumFDs()
	if err != nil {
		merr = multierror.Append(merr, err)
	}

	numThread, err := proc.NumThreads()
	if err != nil {
		merr = multierror.Append(merr, err)
	}

	cpuPercent, err := proc.Percent(0)
	if err != nil {
		merr = multierror.Append(merr, err)
	}

	ps = &ProcessState{
		Pid:        int(proc.Pid),
		NumFDs:     int(numFDs),
		NumThreads: int(numThread),
		CPUPercent: cpuPercent,
		UpdateTime: time.Now(),
	}

	cpuInfo, err := proc.Times()
	if err != nil {
		merr = multierror.Append(merr, err)
	} else {
		ps.CPUInfo = *cpuInfo
	}

	ctxSwitch, err := proc.NumCtxSwitches()
	if err != nil {
		merr = multierror.Append(merr, err)
	} else {
		ps.CtxSwitch = *ctxSwitch
	}

	memInfo, err := proc.MemoryInfo()
	if err != nil {
		merr = multierror.Append(merr, err)
	} else {
		ps.MemInfo = *memInfo
	}

	diskIO, err := proc.IOCounters()
	if err != nil {
		merr = multierror.Append(merr, err)
	} else {
		ps.DiskIO = *diskIO
	}

	return
}

// GetMetric return metric for current ProcessState
func (ps *ProcessState) GetMetric() map[string]interface{} {
	metric := map[string]interface{}{}
	metric = map[string]interface{}{
		"numFDs":     ps.NumFDs,
		"numThreads": ps.NumThreads,
		"cpuPercent": ps.CPUPercent,
		"memRss":     ps.MemInfo.RSS,
		"memSwap":    ps.MemInfo.Swap,
		"memVms":     ps.MemInfo.VMS,

		"diskReadBytes":  ps.DiskIO.ReadBytes,
		"diskReadCount":  ps.DiskIO.ReadCount,
		"diskWriteBytes": ps.DiskIO.WriteBytes,
		"diskWriteCount": ps.DiskIO.WriteCount,

		"cpuUser":   ps.CPUInfo.User,
		"cpuSys":    ps.CPUInfo.System,
		"cpuIOWait": ps.CPUInfo.Iowait,
	}

	return metric
}
