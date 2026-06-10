//go:build unix
// +build unix

package fs

import (
	"fmt"
	"syscall"
)

func getUidGid(sysinfo any) (uint32, uint32, error) {
	if st, ok := sysinfo.(*syscall.Stat_t); ok && st != nil {
		return uint32(st.Uid), uint32(st.Gid), nil
	}
	return 0, 0, fmt.Errorf("invalid sysinfo")
}
