package replica

import "errors"

var (
	ErrNoHostMeetCondition = errors.New("replica: cannot find host match condition")
	ErrHostShortOfResource = errors.New("replica: host short of resource")
	ErrCocurrencyFull      = errors.New("repllica: cocurrency full, please try again 2 mins later")
	ErrUnknown             = errors.New("replica: unknown error")
)
