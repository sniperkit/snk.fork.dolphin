package ps

import (
	"fmt"
	"sync"
	"time"

	"we.com/dolphin/types"
)

type diskIOInfo struct {
	Time  time.Time
	Count uint64
}

var (
	diskLock      sync.RWMutex
	lastDiskWrite = map[int]*diskIOInfo{}
)

// Check  check process resource usage
func (ps *ProcessState) Check(dc *types.DeployConfig) (conditions, events []*types.Condition) {
	if ps == nil {
		return nil, nil
	}

	resReq := dc.ResourceRequired

	resAct := &types.InstanceResUsage{
		Memory:         ps.MemInfo.RSS,
		CPUPercent:     ps.CPUPercent,
		Threads:        ps.NumThreads,
		DiskBytesWrite: ps.DiskIO.WriteBytes,
	}

	ratio := resAct.Memory * 100 / resReq.Memory
	switch {
	case ratio > 150 && resAct.Memory <= resReq.MaxAllowedMemory:
		events = append(events, &types.Condition{
			Type:    types.HighMem,
			Message: fmt.Sprintf("memory usage: %v, %v%%  of configed", resAct.Memory, ratio),
		})
	case resReq.Memory >= resReq.MaxAllowedMemory:
		conditions = append(conditions, &types.Condition{
			Type:    types.HighMem,
			Message: fmt.Sprintf("memory usage: %v, greater than configed: %v", resAct.Memory, resReq.MaxAllowedMemory),
		})
	}

	if resAct.Threads > resReq.MaxAllowdThreads {
		conditions = append(conditions, &types.Condition{
			Type:    types.HighThreads,
			Message: fmt.Sprintf("threads: %v, greater than  configed: %v", resAct.Threads, resReq.MaxAllowdThreads),
		})
	} else if resAct.Threads > resReq.MaxAllowdThreads*8/10 {
		events = append(events, &types.Condition{
			Type:    types.HighThreads,
			Message: fmt.Sprintf("threads: %v, greater than 80%% configed: %v", resAct.Threads, resReq.MaxAllowdThreads),
		})
	}

	g := 1024 * 1024 * 1024
	cpuUsage := uint64(resAct.CPUPercent * float64(g/100))
	if cpuUsage > resReq.MaxAllowedCPU {
		conditions = append(conditions, &types.Condition{
			Type:    types.HighCPU,
			Message: fmt.Sprintf("cpu: %.2f%% greater than max allowed: %.2f%%", resAct.CPUPercent, float64(resReq.MaxAllowedCPU*100)/float64(g)),
		})
	} else if cpuUsage > resReq.MaxAllowedCPU*8/10 || cpuUsage > resReq.CPU*5 {
		events = append(events, &types.Condition{
			Type:    types.HighCPU,
			Message: fmt.Sprintf("cpu: %.2f%% greater than configed: %.2f%%", resAct.CPUPercent, float64(resReq.CPU*100)/float64(g)),
		})
	}

	diskLock.RLock()
	diskInfo, ok := lastDiskWrite[ps.Pid]
	diskLock.RUnlock()

	renew := true
	if ok {
		day := uint64(12 * 24)
		d := time.Now().Sub(diskInfo.Time).Seconds()
		size := resAct.DiskBytesWrite - diskInfo.Count
		// in 5mins
		if d < 300 {
			if size*day > resReq.DiskSpace {
				events = append(events, &types.Condition{
					Type:    types.HighDiskIO,
					Message: fmt.Sprintf("diskIO: write %v", resAct.DiskBytesWrite),
				})
			} else {
				renew = false
			}
		}
	}

	if renew {
		di := &diskIOInfo{
			Time:  time.Now(),
			Count: resAct.DiskBytesWrite,
		}
		diskLock.Lock()
		lastDiskWrite[ps.Pid] = di
		diskLock.Unlock()
	}

	return
}
