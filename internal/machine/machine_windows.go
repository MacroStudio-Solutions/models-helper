//go:build windows

package machine

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")

func readMemInfo() (uint64, uint64, bool) {
	var ms memoryStatusEx
	ms.Length = uint32(unsafe.Sizeof(ms))
	ret, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms)))
	if ret == 0 {
		return 0, 0, false
	}
	return ms.TotalPhys, ms.AvailPhys, true
}

func detectGpuFallback() bool {
	return false
}

func detectVulkan() (bool, string) {
	dir := os.Getenv("WINDIR")
	if dir == "" {
		dir = `C:\Windows`
	}
	if fileExists(filepath.Join(dir, "System32", "vulkan-1.dll")) {
		return true, ""
	}
	return false, "vulkan-1.dll ausente no sistema"
}
