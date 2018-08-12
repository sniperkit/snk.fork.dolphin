/*
Sniperkit-Bot
- Status: analyzed
*/

package utils

// User is simple user
type User struct {
	Name  string
	Roles []string
	Token string
}

// ProjectPerm is a group permission of a project
type ProjectPerm map[Role]struct{}

// Group permission of a group by projects
type Group struct {
	Name  string
	Roles map[string]ProjectPerm
}

// Role is a role in an project
type Role string

const (
	// RolePM admin of a project, have full control of a project except make online changes
	RolePM Role = "pm"
	// RoleDeveloper developer is a project, have permission to issue a ticket
	RoleDeveloper Role = "developer"
	// RoleOperator have permission to make online  changes
	RoleOperator Role = "operator"
	// RoleQA qa
	RoleQA Role = "QA"
	// RoleObserver observer
	RoleObserver Role = "observer"
)

var (
	// UserAdmin have all permissions
	UserAdmin = User{"admin", nil, ""}
	// GroupAdmin have all permissions
	GroupAdmin = Group{Name: "admin"}
)
