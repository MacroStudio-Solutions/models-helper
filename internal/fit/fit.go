package fit

import (
	"fmt"

	"github.com/MacroStudio-Solutions/models-helper/internal/contract"
)

const kvCacheMarginBytes = 1610612736

func Compute(ramTotalBytes uint64, ramAvailableBytes uint64, vramBytes uint64, modelBytes uint64) contract.TModelFit {
	need := float64(modelBytes)*1.2 + kvCacheMarginBytes
	available := float64(ramAvailableBytes)
	total := float64(ramTotalBytes)
	vram := float64(vramBytes)
	capacity := available + vram
	overall := total + vram
	f := contract.TModelFit{
		FitGpu:        vram >= float64(modelBytes)*1.15,
		RequiredBytes: uint64(need),
	}
	if need <= capacity*0.85 {
		f.FitOk = true
		f.FitTight = false
	} else if need <= overall*0.92 {
		f.FitOk = false
		f.FitTight = true
	} else {
		f.FitOk = false
		f.FitTight = false
	}
	return f
}

func SizeGb(bytes uint64) string {
	return fmt.Sprintf("%.1f", float64(bytes)/1073741824)
}
