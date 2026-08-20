//go:build windows

package jobs

import (
	"syscall"
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var procOpenProcess = kernel32.NewProc("OpenProcess")
var procCloseHandle = kernel32.NewProc("CloseHandle")

const processQueryLimitedInformation = 0x1000

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, _, callErr := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if handle != 0 {
		procCloseHandle.Call(handle)
		return true
	}
	if errno, ok := callErr.(syscall.Errno); ok && errno == 5 {
		return true
	}
	return false
}
