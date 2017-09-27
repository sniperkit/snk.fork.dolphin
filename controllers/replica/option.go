package replica

import "time"

type option struct {
	cocurrency          int
	legacyVerionTimeout time.Duration
	dryMode             bool
}
