/*
Sniperkit-Bot
- Status: analyzed
*/

package service

type ServiceManager interface {
	List()
	Restart()
	Reload()
	Start()
}
