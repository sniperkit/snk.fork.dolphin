package vo

import (
	"time"
)

// Instance vo
type Instance struct {
	UUID         string    `json:"id"`
	Node         string    `json:"node"`
	User         string    `json:"user"`
	Host         string    `json:"hostID"`
	HostName     string    `json:"hostname"`
	IP           string    `json:"address"`
	Port         int       `json:"port"`
	Pid          int       `json:"pid"`
	Version      string    `json:"version"`
	StartTime    time.Time `json:"startTime"`
	InstanceType int       `json:"type"`
	Status       int       `json:"status"`
	RouteStatus  string    `json:"routeStatus"`
	APIVersion   string    `json:"apiVersion"`
}
