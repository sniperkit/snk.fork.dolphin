/*
Sniperkit-Bot
- Status: analyzed
*/

package deploy

type errorCode int

const (
	unknown errorCode = iota
	imageNotExist
	imageVersionNotExist
	imageCanntEmpty
	worktreeNotClean
)

var (
	errMsg = map[errorCode]string{
		unknown:              "unknown",
		imageCanntEmpty:      "image name cannt be empty",
		imageNotExist:        "image not found",
		imageVersionNotExist: "image verion does not exist",
		worktreeNotClean:     "work tree not clean",
	}
)

type terror struct {
	code errorCode
	msg  string
}

func (e *terror) Error() string {
	if e == nil {
		return ""
	}

	s, ok := errMsg[e.code]
	if !ok {
		s = "unknown err"
	}
	if len(e.msg) > 0 {
		return s + ": " + e.msg
	}
	return s
}
