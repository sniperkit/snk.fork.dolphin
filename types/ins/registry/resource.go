/*
Sniperkit-Bot
- Status: analyzed
*/

package registry

import (
	"bytes"
	"fmt"
	"time"

	"github.com/golang/glog"
	"we.com/dolphin/types"
)

var (
	defaultResource  = map[StageType]ResouceSpec{}
	unknownStageType = map[StageType]struct{}{}
	unknownChan      = make(chan StageType, 3)
)

type StageType struct {
	Stage types.Stage
	Type  types.ProjectType
}

type ResouceSpec struct {
	Small  types.DeployResource
	Medium types.DeployResource
	Large  types.DeployResource
}

// UpdateDefaultDeployResource update default resource usage
func UpdateDefaultDeployResource(st StageType, rs ResouceSpec) {
	defaultResource[st] = rs
	delete(unknownStageType, st)
}

// GetDefaultDeployResource get default deploy resource
func GetDefaultDeployResource(st StageType) *ResouceSpec {
	r, ok := defaultResource[st]
	if ok {
		return &r
	}
	unknownChan <- st
	return nil
}

type ResourceInfor interface {
	GetDeployResouce(key types.DeployKey) *types.DeployResource
}

func init() {
	go func() {
		d := 2 * time.Hour
		timer := time.NewTimer(d)

		for {
			select {
			case <-timer.C:
				var b bytes.Buffer
				fmt.Fprintf(&b, "ps: %v unkown stageTypes:", len(unknownStageType))
				for k := range unknownStageType {
					fmt.Fprintf(&b, "%v:%v,", k.Stage, k.Type)
				}

				glog.Error(b.String())
				timer.Reset(d)
			case st := <-unknownChan:
				if _, ok := unknownStageType[st]; !ok {
					unknownStageType[st] = struct{}{}
					timer.Reset(2 * time.Second)
				}
			}
		}
	}()
}
