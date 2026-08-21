package inventory

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MacroStudio-Solutions/models-helper/internal/contract"
	"github.com/MacroStudio-Solutions/models-helper/internal/fit"
	"github.com/MacroStudio-Solutions/models-helper/internal/format"
)

// ExtGguf e ExtWhisper sao as duas extensoes de peso que o Studio inventaria:
// o llama.cpp le .gguf, o whisper.cpp le .bin. O inventario recebe a extensao
// em vez de aceitar as duas sempre, para que um diretorio nunca liste peso de
// um motor que nao o carrega.
const (
	ExtGguf    = ".gguf"
	ExtWhisper = ".bin"
)

type TOptions struct {
	Ext string
	// Speech troca a formula de viabilidade: um modelo de transcricao nao
	// reserva KV-cache de conversa.
	Speech bool
	Engine string
}

func List(dir string, machineProfile contract.TMachineProfile) ([]contract.TInstalledModel, error) {
	return ListWith(dir, machineProfile, TOptions{Ext: ExtGguf})
}

func ListWith(dir string, machineProfile contract.TMachineProfile, options TOptions) ([]contract.TInstalledModel, error) {
	ext := options.Ext
	if ext == "" {
		ext = ExtGguf
	}
	entries := []contract.TInstalledModel{}
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return entries, nil
		}
		return nil, err
	}
	for _, e := range files {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ext) {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		size := uint64(info.Size())
		full := filepath.Join(dir, e.Name())
		verdict := fit.Compute(machineProfile.RamTotalBytes, machineProfile.RamAvailableBytes, machineProfile.VramBytes, size)
		if options.Speech {
			verdict = fit.Speech(machineProfile.RamTotalBytes, machineProfile.RamAvailableBytes, size)
		}
		entries = append(entries, contract.TInstalledModel{
			TModelFit: verdict,
			Name:      e.Name(),
			Path:      full,
			SizeBytes: size,
			SizeGb:    fit.SizeGb(size),
			SizeLabel: format.Bytes(size),
			ApiName:   strings.TrimSuffix(e.Name(), ext),
			Engine:    options.Engine,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}
