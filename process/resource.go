package ps

import "we.com/dolphin/types"

var (
	defaultResource = map[StageType]ResouceSpec{}
)

type StageType struct {
	Stage types.Stage
	Type  types.ProjectType
}

type ResouceSpecType string

var (
	Small  ResouceSpecType = "small"
	Medium ResouceSpecType = "medium"
	Large  ResouceSpecType = "large"
)

type ResouceSpec struct {
	Small  types.DeployResource
	Medium types.DeployResource
	Large  types.DeployResource
}

// UpdateDefaultDeployResource update default resource usage
func UpdateDefaultDeployResource(st StageType, rs ResouceSpec) {
	defaultResource[st] = rs
}

// GetDefaultDeployResource get default deploy resource
func GetDefaultDeployResource(st StageType) *ResouceSpec {
	r, ok := defaultResource[st]
	if ok {
		return &r
	}
	return nil
}

type ResourceInfor interface {
	GetDeployResouce(key types.DeployKey) *types.DeployResource
}
