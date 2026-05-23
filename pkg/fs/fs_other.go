//go:build !unix
// +build !unix

package fs

func getUidGid(sysinfo any) (uint32, uint32, error) {
	return 0, 0, nil
}
