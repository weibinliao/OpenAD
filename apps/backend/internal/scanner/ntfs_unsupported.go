//go:build !windows

package scanner

import "fmt"

func (scanner *NTFSScanner) readPermissions(path string) ([]Permission, error) {
	return nil, fmt.Errorf("NTFS scanning is only supported on Windows: %s", path)
}
