//go:build !linux && !windows

package machine

func readMemInfo() (uint64, uint64, bool) {
	return 0, 0, false
}

func detectGpuFallback() bool {
	return false
}

func detectVulkan() (bool, string) {
	return false, "deteccao de Vulkan nao suportada nesta plataforma"
}
