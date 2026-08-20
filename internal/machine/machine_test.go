package machine

import (
	"runtime"
	"testing"

	"github.com/MacroStudio-Solutions/models-helper/internal/contract"
)

func TestRecommendVariantNoVulkanFallsToCpu(t *testing.T) {
	if runtime.GOOS == "windows" && runtime.GOARCH == "arm64" {
		t.Skip("plataforma sem variante de GPU")
	}
	p := contract.TMachineProfile{HasVulkan: false, VulkanUnavailableReason: "sem ICD"}
	id, reason := RecommendVariant(p)
	if id != "llama-cpp" {
		t.Fatalf("esperado llama-cpp, obtido %s", id)
	}
	if reason == "" {
		t.Fatalf("motivo vazio")
	}
}

func TestRecommendVariantWindowsArmNeverGpu(t *testing.T) {
	if !(runtime.GOOS == "windows" && runtime.GOARCH == "arm64") {
		t.Skip("teste exclusivo de windows arm64")
	}
	id, _ := RecommendVariant(contract.TMachineProfile{HasVulkan: true, VramBytes: 8 << 30})
	if id != "llama-cpp" {
		t.Fatalf("windows arm64 nunca recebe GPU, obtido %s", id)
	}
}

func TestRecommendVariantGpuWhenVulkanAndVram(t *testing.T) {
	if runtime.GOOS == "windows" && runtime.GOARCH == "arm64" {
		t.Skip("plataforma sem variante de GPU")
	}
	id, reason := RecommendVariant(contract.TMachineProfile{HasVulkan: true, VramBytes: 8 << 30})
	if id != "llama-cpp-vulkan" {
		t.Fatalf("esperado llama-cpp-vulkan, obtido %s", id)
	}
	if reason == "" {
		t.Fatalf("motivo vazio")
	}
}

func TestRecommendVariantVulkanWithoutVram(t *testing.T) {
	if runtime.GOOS == "windows" && runtime.GOARCH == "arm64" {
		t.Skip("plataforma sem variante de GPU")
	}
	id, reason := RecommendVariant(contract.TMachineProfile{HasVulkan: true, VramBytes: 0})
	if id != "llama-cpp" || reason == "" {
		t.Fatalf("esperado queda para processador com motivo, obtido %s / %q", id, reason)
	}
}

func TestProfileRuns(t *testing.T) {
	p := Profile()
	if p.CpuCores <= 0 {
		t.Fatalf("cpuCores %d", p.CpuCores)
	}
	if p.HasVulkan && p.VulkanUnavailableReason != "" {
		t.Fatalf("vulkan disponivel nao pode ter motivo de ausencia")
	}
	if !p.HasVulkan && p.VulkanUnavailableReason == "" {
		t.Fatalf("vulkan ausente exige motivo declarado")
	}
}
