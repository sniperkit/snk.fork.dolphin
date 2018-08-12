/*
Sniperkit-Bot
- Status: analyzed
*/

package java

import (
	"we.com/dolphin/types"
	"we.com/dolphin/types/ins/registry"
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
		DiskSpace:        uint64(5 * g),
		MaxAllowedMemory: uint64(320 * m),
		MaxAllowdThreads: 1 * k,
		MaxAllowedCPU:    uint64(g),
	}

	medium = types.DeployResource{
		Memory:           uint64(256 * m),
		CPU:              uint64(128 * m),
		NetworkIn:        1,
		NetworkOut:       1,
		DiskSpace:        uint64(10 * g),
		MaxAllowedMemory: uint64(650 * m),
		MaxAllowdThreads: 2 * k,
		MaxAllowedCPU:    uint64(2 * g),
	}

	large = types.DeployResource{
		Memory:           uint64(625 * m),
		CPU:              uint64(256 * m),
		NetworkIn:        1,
		NetworkOut:       1,
		DiskSpace:        uint64(15 * g),
		MaxAllowedMemory: uint64(1024 * m),
		MaxAllowdThreads: 2 * k,
		MaxAllowedCPU:    uint64(2 * g),
	}

	high = types.DeployResource{
		Memory:           uint64(2048 * m),
		CPU:              uint64(512 * m),
		NetworkIn:        1,
		NetworkOut:       1,
		DiskSpace:        uint64(15 * g),
		MaxAllowedMemory: uint64(4 * g),
		MaxAllowdThreads: 2 * k,
		MaxAllowedCPU:    uint64(4 * g),
	}
	// CPUResourcePerCPU resouce providered by a single cpu
	CPUResourcePerCPU = 1 * g
)

func init() {
	dev := registry.StageType{
		Stage: types.Dev,
		Type:  Type,
	}
	devSpec := registry.ResouceSpec{
		Small:  low,
		Medium: medium,
		Large:  medium,
	}

	test := registry.StageType{
		Stage: types.Test,
		Type:  Type,
	}
	testSpec := registry.ResouceSpec{
		Small:  low,
		Medium: medium,
		Large:  medium,
	}

	uat := registry.StageType{
		Stage: types.UAT,
		Type:  Type,
	}
	uatSpec := registry.ResouceSpec{
		Small:  low,
		Medium: medium,
		Large:  medium,
	}

	prd := registry.StageType{
		Stage: types.Production,
		Type:  Type,
	}
	prdSpec := registry.ResouceSpec{
		Small:  medium,
		Medium: large,
		Large:  high,
	}

	registry.UpdateDefaultDeployResource(dev, devSpec)
	registry.UpdateDefaultDeployResource(test, testSpec)
	registry.UpdateDefaultDeployResource(uat, uatSpec)
	registry.UpdateDefaultDeployResource(prd, prdSpec)
}
