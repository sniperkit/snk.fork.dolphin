package replica

import "time"

type option struct {
	// maxtries time, when deploy a config  fails
	maxTries            int
	cocurrency          int
	legacyVerionTimeout time.Duration
	dryMode             bool
}
