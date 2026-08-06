//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
	"syscall"
	"unsafe"

	"github.com/gin-gonic/gin"
	"golang.org/x/sys/windows"
)

var listUNCServerShares = listUNCServerSharesWindows

type shareInfo1 struct {
	NetName *uint16
	Type    uint32
	Remark  *uint16
}

var netShareEnumProc = windows.NewLazySystemDLL("netapi32.dll").NewProc("NetShareEnum")

func listUNCServerSharesWindows(serverRoot string) ([]gin.H, error) {
	normalizedRoot := normalizeUNCPath(serverRoot)
	if !isUNCServerRootPath(normalizedRoot) {
		return nil, fmt.Errorf("UNC server share discovery requires a server root path")
	}

	serverName, err := windows.UTF16PtrFromString(normalizedRoot)
	if err != nil {
		return nil, err
	}

	var resumeHandle uint32
	shareEntries := make([]gin.H, 0, 8)
	const maxPreferredLength = uint32(0xFFFFFFFF)
	for {
		var buffer *byte
		var entriesRead uint32
		var totalEntries uint32

		status, _, callErr := netShareEnumProc.Call(
			uintptr(unsafe.Pointer(serverName)),
			uintptr(1),
			uintptr(unsafe.Pointer(&buffer)),
			uintptr(maxPreferredLength),
			uintptr(unsafe.Pointer(&entriesRead)),
			uintptr(unsafe.Pointer(&totalEntries)),
			uintptr(unsafe.Pointer(&resumeHandle)),
		)
		if status != 0 && status != uintptr(syscall.ERROR_MORE_DATA) {
			if callErr != nil && callErr != syscall.Errno(0) {
				return nil, callErr
			}
			return nil, syscall.Errno(status)
		}

		if buffer != nil {
			func() {
				defer func() {
					_ = windows.NetApiBufferFree(buffer)
				}()

				entrySize := unsafe.Sizeof(shareInfo1{})
				for index := uint32(0); index < entriesRead; index++ {
					entry := (*shareInfo1)(unsafe.Pointer(uintptr(unsafe.Pointer(buffer)) + uintptr(index)*entrySize))
					name := strings.TrimSpace(windows.UTF16PtrToString(entry.NetName))
					if shouldSkipUNCShare(name, entry.Type) {
						continue
					}

					shareEntries = append(shareEntries, gin.H{
						"name": name,
						"path": joinUNCSharePath(normalizedRoot, name),
					})
				}
			}()
		}

		if status != uintptr(syscall.ERROR_MORE_DATA) {
			break
		}
	}

	sort.Slice(shareEntries, func(i, j int) bool {
		left := strings.ToLower(shareEntries[i]["name"].(string))
		right := strings.ToLower(shareEntries[j]["name"].(string))
		return left < right
	})

	return shareEntries, nil
}

func shouldSkipUNCShare(name string, shareType uint32) bool {
	if name == "" {
		return true
	}

	if shareType&0x3 != 0 {
		return true
	}

	upper := strings.ToUpper(name)
	if upper == "ADMIN$" || upper == "IPC$" {
		return true
	}

	return strings.HasSuffix(upper, "$")
}

func joinUNCSharePath(serverRoot, shareName string) string {
	root := strings.TrimRight(serverRoot, `\`)
	return root + `\` + shareName
}
