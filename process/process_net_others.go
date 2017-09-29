// +build !linux

package ps

import "we.com/dolphin/types"

// ListenPortsOfPid listening port of pid  tcp/ipv4
func ListenPortsOfPid(pid int) ([]types.Addr, error) {
	return []types.Addr{}, nil
}
