/*
Sniperkit-Bot
- Status: analyzed
*/

package project

type Info struct {
	Desc        string `json:"desc,omitempty"`
	APIVersion  string `json:"apiVersion,omitempty"`
	ServiceName string `json:"serviceName,omitempty"`
	Project     string `json:"project,omitempty"`
	ZKRoute     string `json:"zkRoute,omitempty"`
	ZKInstance  string `json:"zkInstance,omitempty"`
}
