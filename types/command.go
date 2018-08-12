/*
Sniperkit-Bot
- Status: analyzed
*/

package types

import (
	"time"
)

// CommandType is a known command  type
type CommandType string

const (
	// CMDProbe  probe an instance, or service
	CMDProbe CommandType = "probe"
	// CMDStopInstance stop an given instances
	CMDStopInstance CommandType = "stop"
	// CMDStartInstance  start an cluster
	CMDStartInstance CommandType = "start"
	// CMDRestartInstance  restart an instance
	CMDRestartInstance CommandType = "restart"
)

// Command represent a command sent by master/manager, for this
// node to execute
type Command struct {
	ComandID       string            `json:"comandID,omitempty"`
	Type           CommandType       `json:"type,omitempty"`
	Args           interface{}       `json:"args,omitempty"`
	Envs           map[string]string `json:"envs,omitempty"`
	ExecuteTimeout time.Duration     `json:"executeTimeout,omitempty"`
	Needout        bool              `json:"needout,omitempty"`
	OutKeep        time.Duration     `json:"outKeep,omitempty"`
}

// CommandResult represent an execute result of and  command
type CommandResult struct {
	CommandID string        `json:"commandID,omitempty"`
	Success   bool          `json:"success,omitempty"`
	Took      time.Duration `json:"took,omitempty"`
	Output    []byte        `json:"output,omitempty"`
	Err       error         `json:"err,omitempty"`
}
