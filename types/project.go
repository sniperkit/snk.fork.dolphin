package types

// ProjectType  same with MonitorType
type ProjectType string

// UUID uuid(universal unique indetifior)
type UUID string

var (
	knownProjectTypes = []ProjectType{
		ProjectType("java"),
	}
)

// GetKnownProjectTypes return registerd ProjectTypes
func GetKnownProjectTypes() []ProjectType {
	ret := []ProjectType{}
	copy(ret, knownProjectTypes)
	return ret
}
