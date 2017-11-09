package java

import (
	"we.com/dolphin/types"
)

/*

 */

// deployGroup in order to simplify deploy configes
// which share many attributes, we try to group this
// config together, so the same config only need to  specify once
type deployGroup struct {
	NumOfInstance int         `json:"numOfInstance,omitempty"`
	Stage         types.Stage `json:"env,omitempty"`

	Image        string                 `json:"image,omitempty"`
	DeployDir    string                 `json:"deployDir,omitempty"`
	Values       map[string]interface{} `json:"values,omitempty"`
	DeployPolicy types.DeployPolicy     `json:"deployPolicy,omitempty"`

	// these fields used to select which hosts can start this project
	Selector         types.Selector       `json:"selector,omitempty"`
	ResourceRequired types.DeployResource `json:"resourceRequired,omitempty"`

	RestartPolicy *types.RestartPolicy `json:"restartPolicy,omitempty"`
	UpdatePolicy  *types.UpdateOption  `json:"updatePolicy,omitempty"`
	Deploys       map[string]types.DeployConfig
}

const (
	path = "/etc/dolphin/deploy/uat/"
)

func (dg *deployGroup) validate() error {
	if dg == nil {
		return nil
	}

	return nil
}
