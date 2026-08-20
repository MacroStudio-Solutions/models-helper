package inventory

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MacroStudio-Solutions/models-helper/internal/contract"
	"github.com/MacroStudio-Solutions/models-helper/internal/fit"
)

func List(dir string, machineProfile contract.TMachineProfile) ([]contract.TInstalledModel, error) {
	entries := []contract.TInstalledModel{}
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return entries, nil
		}
		return nil, err
	}
	for _, e := range files {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".gguf") {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		full := filepath.Join(dir, e.Name())
		entries = append(entries, contract.TInstalledModel{
			TModelFit: fit.Compute(machineProfile.RamTotalBytes, machineProfile.RamAvailableBytes, machineProfile.VramBytes, uint64(info.Size())),
			Name:      e.Name(),
			Path:      full,
			SizeBytes: uint64(info.Size()),
			SizeGb:    fit.SizeGb(uint64(info.Size())),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}
