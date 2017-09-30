package service

type ServiceManager interface {
	List()
	Restart()
	Reload()
	Start()
}
