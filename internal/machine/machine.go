package machine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/MacroStudio-Solutions/models-helper/internal/contract"
	"github.com/MacroStudio-Solutions/models-helper/internal/format"
)

func Profile() contract.TMachineProfile {
	p := contract.TMachineProfile{CpuCores: runtime.NumCPU()}
	if total, avail, ok := readMemInfo(); ok {
		p.RamTotalBytes = total
		p.RamAvailableBytes = avail
	}
	queryNvidia(&p)
	if !p.HasGpu {
		p.HasGpu = detectGpuFallback()
	}
	p.HasVulkan, p.VulkanUnavailableReason = detectVulkan()
	return describe(p)
}

// describe preenche os rotulos legiveis. Fica no fim do perfil, e nao em cada
// leitor, para que os campos brutos e os formatados nunca discordem.
func describe(p contract.TMachineProfile) contract.TMachineProfile {
	p.RamTotalLabel = format.Bytes(p.RamTotalBytes)
	p.RamAvailableLabel = format.Bytes(p.RamAvailableBytes)
	p.CpuLabel = format.Cores(p.CpuCores)
	if p.VramBytes > 0 {
		p.VramLabel = format.Bytes(p.VramBytes)
	} else {
		p.VramLabel = "sem memória de vídeo detectada"
	}
	switch {
	case p.HasGpu && p.GpuName != "" && p.VramBytes > 0:
		p.GpuLabel = p.GpuName + " · " + format.Bytes(p.VramBytes)
	case p.HasGpu && p.GpuName != "":
		p.GpuLabel = p.GpuName
	case p.HasGpu:
		p.GpuLabel = "GPU detectada"
	default:
		p.GpuLabel = "sem GPU dedicada"
	}
	return p
}

func queryNvidia(p *contract.TMachineProfile) {
	bin, err := exec.LookPath("nvidia-smi")
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "--query-gpu=memory.total,name", "--format=csv,noheader,nounits").Output()
	if err != nil {
		return
	}
	line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	if line == "" {
		return
	}
	fields := strings.SplitN(line, ",", 2)
	mib, err := strconv.Atoi(strings.TrimSpace(fields[0]))
	if err != nil || mib <= 0 {
		return
	}
	p.HasGpu = true
	p.VramBytes = uint64(mib) * 1048576
	if len(fields) > 1 {
		p.GpuName = strings.TrimSpace(fields[1])
	}
}

func RecommendVariant(p contract.TMachineProfile) (string, string) {
	if runtime.GOOS == "windows" && runtime.GOARCH == "arm64" {
		return "llama-cpp", "plataforma sem artefato de GPU declarado; recomendada a variante de processador"
	}
	if !p.HasVulkan {
		reason := p.VulkanUnavailableReason
		if reason == "" {
			reason = "caminho Vulkan indisponivel"
		}
		return "llama-cpp", "sem driver Vulkan adequado (" + reason + "); recomendada a variante de processador"
	}
	if p.VramBytes == 0 {
		return "llama-cpp", "Vulkan disponivel, mas nenhuma memoria de video detectada; recomendada a variante de processador"
	}
	return "llama-cpp-vulkan", fmt.Sprintf("Vulkan disponivel com %.1f GiB de memoria de video", float64(p.VramBytes)/1073741824)
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
