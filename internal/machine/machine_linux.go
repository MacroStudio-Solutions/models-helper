//go:build linux

package machine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func readMemInfo() (uint64, uint64, bool) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, false
	}
	var total, avail uint64
	var haveTotal, haveAvail bool
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		kb, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			total = kb * 1024
			haveTotal = true
		case "MemAvailable:":
			avail = kb * 1024
			haveAvail = true
		}
	}
	return total, avail, haveTotal && haveAvail
}

func detectGpuFallback() bool {
	entries, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "card") && !strings.Contains(name, "-") {
			return true
		}
	}
	return false
}

var vulkanLoaderCandidates = []string{
	"/usr/lib/x86_64-linux-gnu/libvulkan.so.1",
	"/usr/lib/aarch64-linux-gnu/libvulkan.so.1",
	"/usr/lib64/libvulkan.so.1",
	"/usr/lib/libvulkan.so.1",
	"/lib/x86_64-linux-gnu/libvulkan.so.1",
	"/lib/aarch64-linux-gnu/libvulkan.so.1",
	"/usr/local/lib/libvulkan.so.1",
}

func vulkanLoaderPresent() bool {
	if out, err := exec.Command("ldconfig", "-p").Output(); err == nil {
		if strings.Contains(string(out), "libvulkan.so.1") {
			return true
		}
	}
	for _, p := range vulkanLoaderCandidates {
		if fileExists(p) {
			return true
		}
	}
	return false
}

func detectVulkan() (bool, string) {
	if !vulkanLoaderPresent() {
		return false, "carregador Vulkan (libvulkan.so.1) ausente no sistema"
	}
	for _, pattern := range []string{"/usr/share/vulkan/icd.d/*.json", "/etc/vulkan/icd.d/*.json"} {
		if matches, err := filepath.Glob(pattern); err == nil && len(matches) > 0 {
			return true, ""
		}
	}
	return false, "nenhum driver Vulkan (ICD) instalado em /usr/share/vulkan/icd.d"
}
