package fit

import (
	"fmt"

	"github.com/MacroStudio-Solutions/models-helper/internal/contract"
	"github.com/MacroStudio-Solutions/models-helper/internal/format"
)

// Margem de KV-cache de um modelo de linguagem servido pelo llama.cpp.
const kvCacheMarginBytes = 1610612736

// Um modelo de transcricao nao mantem contexto de conversa: o que ele reserva
// alem dos pesos e o buffer de audio e de decodificacao, uma ordem de grandeza
// menor. Aplicar a margem do llama.cpp aqui reprovaria modelos que rodam bem.
const speechMarginBytes = 536870912

func Compute(ramTotalBytes uint64, ramAvailableBytes uint64, vramBytes uint64, modelBytes uint64) contract.TModelFit {
	return computeWith(ramTotalBytes, ramAvailableBytes, vramBytes, modelBytes, kvCacheMarginBytes)
}

// Speech avalia um modelo de transcricao. A memoria de video e deliberadamente
// ignorada: o artefato de whisper.cpp declarado pela extensao e o de
// processador, entao prometer GPU seria promessa que o binario nao cumpre.
func Speech(ramTotalBytes uint64, ramAvailableBytes uint64, modelBytes uint64) contract.TModelFit {
	return computeWith(ramTotalBytes, ramAvailableBytes, 0, modelBytes, speechMarginBytes)
}

func computeWith(ramTotalBytes uint64, ramAvailableBytes uint64, vramBytes uint64, modelBytes uint64, marginBytes float64) contract.TModelFit {
	need := float64(modelBytes)*1.2 + marginBytes
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
	return Describe(f)
}

// Describe deriva a ordenacao e o rotulo a partir do veredito ja calculado.
// Fica separado do calculo porque um consumidor que monte um TModelFit a mao —
// um catalogo fixo, um teste — precisa exatamente da mesma tabela.
func Describe(f contract.TModelFit) contract.TModelFit {
	f.RequiredLabel = format.Bytes(f.RequiredBytes)
	switch {
	case f.FitGpu:
		f.FitRank = contract.FitRankGpu
		f.FitLabel = "roda na GPU"
	case f.FitOk:
		f.FitRank = contract.FitRankOk
		f.FitLabel = "roda bem"
	case f.FitTight:
		f.FitRank = contract.FitRankTight
		f.FitLabel = "no limite"
	default:
		f.FitRank = contract.FitRankNo
		f.FitLabel = "não recomendado"
	}
	return f
}

func SizeGb(bytes uint64) string {
	return fmt.Sprintf("%.1f", float64(bytes)/1073741824)
}
