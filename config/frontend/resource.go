package frontend

import (
	"we.com/dolphin/types"
)

/*
	predefined resource need  for give give types and stages
	every type has three different shemes
*/

var (
	javaType  = types.ProjectType("java")
	esType    = types.ProjectType("es")
	redisType = types.ProjectType("redis")
	mysqlType = types.ProjectType("mysql")
	mqType    = types.ProjectType("mq")
)

var (
	k = 1024
	m = 1024 * k
	g = 1024 * m

	low = types.DeployResource{
		Memory:           uint64(128 * m),
		CPU:              uint64(128 * m),
		NetworkIn:        1,
		NetworkOut:       1,
		DiskSpace:        0,
		MaxAllowedMemory: uint64(320 * m),
		MaxAllowdThreads: 1 * k,
		MaxAllowedCPU:    uint64(g),
	}

	medium = types.DeployResource{
		Memory:           uint64(256 * m),
		CPU:              uint64(128 * m),
		NetworkIn:        1,
		NetworkOut:       1,
		DiskSpace:        1,
		MaxAllowedMemory: uint64(650 * m),
		MaxAllowdThreads: 2 * k,
		MaxAllowedCPU:    uint64(2 * g),
	}

	high = types.DeployResource{
		Memory:           uint64(2048 * m),
		CPU:              uint64(512 * m),
		NetworkIn:        1,
		NetworkOut:       1,
		DiskSpace:        1,
		MaxAllowedMemory: uint64(4 * g),
		MaxAllowdThreads: 2 * k,
		MaxAllowedCPU:    uint64(4 * g),
	}

	// defaultResUsage default resource uage of an instance of type of env
	defaultResUsage = map[types.ProjectType]map[types.Stage]types.DeployResource{
		javaType: map[types.Stage]types.DeployResource{
			types.Test:         low,
			types.Dev:          low,
			types.Intergration: medium,
			types.Production:   high,
		},
	}

	// CPUResourcePerCPU resouce providered by a single cpu
	CPUResourcePerCPU = 1 * g
)
