package catalog

import (
	"context"
	"path/filepath"

	"github.com/MacroStudio-Solutions/models-helper/internal/contract"
	"github.com/MacroStudio-Solutions/models-helper/internal/fit"
	"github.com/MacroStudio-Solutions/models-helper/internal/format"
)

// O catalogo de transcricao e fixo.
//
// Ao contrario dos modelos de linguagem, aqui nao ha o que procurar: sao dois
// repositorios, os dois oficiais dos projetos, e um punhado de arquivos que nao
// mudam. Buscar num universo de sete opcoes seria cerimonia sem ganho.
//
// A recomendacao e medida, nao inferida do tamanho. Num audio real de 3,6s em
// portugues, nesta maquina: o whisper base errou palavras em 1,5s; o
// large-v3-turbo acertou tudo em 16s; o parakeet q8_0 acertou o mesmo em 0,9s.
// Por isso o padrao e o parakeet, e o whisper e apresentado pelo que o parakeet
// nao faz — marcacao de tempo, traducao para o ingles e deteccao de idioma.

const (
	WhisperRepo  = "ggerganov/whisper.cpp"
	ParakeetRepo = "ggml-org/parakeet-GGUF"

	EngineWhisper  = "whisper"
	EngineParakeet = "parakeet"

	RecommendedSpeechFile = "ggml-parakeet-tdt-0.6b-v3-q8_0.bin"
)

type TSpeechModel struct {
	RepoId string
	File   string
	Name   string
	Engine string
	// FallbackSize mantem a lista utilizavel quando a API publica esta fora do
	// ar: sao arquivos de release, imutaveis, e o tamanho medido hoje continua
	// valendo amanha. O tamanho real da arvore, quando chega, prevalece.
	FallbackSize uint64
	Quantization string
	Summary      string
}

var speechModels = []TSpeechModel{
	{ParakeetRepo, "ggml-parakeet-tdt-0.6b-v3-q8_0.bin", "Parakeet TDT 0.6B v3 Q8_0", EngineParakeet, 668757119, "Q8_0", "A recomendação padrão: mesma precisão do maior whisper em uma fração do tempo. Devolve texto puro, sem marcação de tempo."},
	{ParakeetRepo, "ggml-parakeet-tdt-0.6b-v3-q4_k.bin", "Parakeet TDT 0.6B v3 Q4_K", EngineParakeet, 415611879, "Q4_K", "Metade do peso do Q8_0, com alguma perda de precisão. Para máquina apertada."},
	{ParakeetRepo, "ggml-parakeet-tdt-0.6b-v3-q4_0.bin", "Parakeet TDT 0.6B v3 Q4_0", EngineParakeet, 355615679, "Q4_0", "O menor de todos. Escolha só quando a memória for o limite."},
	{ParakeetRepo, "ggml-parakeet-tdt-0.6b-v3-f16.bin", "Parakeet TDT 0.6B v3 F16", EngineParakeet, 1255897319, "F16", "Precisão cheia em meia palavra. Ganho pequeno sobre o Q8_0 e o dobro do peso."},
	{ParakeetRepo, "ggml-parakeet-tdt-0.6b-v3-f32.bin", "Parakeet TDT 0.6B v3 F32", EngineParakeet, 2508463079, "F32", "Referência sem quantização. Só faz sentido para comparar resultado."},
	{WhisperRepo, "ggml-tiny.bin", "Whisper tiny", EngineWhisper, 77704715, "", "O menor whisper. Rápido e impreciso; serve para teste, não para transcrição de verdade."},
	{WhisperRepo, "ggml-base.bin", "Whisper base", EngineWhisper, 147951465, "", "Erra palavras em português. Vale quando o que importa é a marcação de tempo e não a palavra exata."},
	{WhisperRepo, "ggml-small.bin", "Whisper small", EngineWhisper, 487601967, "", "O primeiro whisper com precisão aceitável em português."},
	{WhisperRepo, "ggml-medium.bin", "Whisper medium", EngineWhisper, 1533763059, "", "Um degrau acima do small, ainda longe do custo do large."},
	{WhisperRepo, "ggml-large-v3-turbo.bin", "Whisper large-v3-turbo", EngineWhisper, 1624555275, "", "O melhor whisper prático: acerta como o large-v3 numa fração do tempo. Escolha esta quando precisar de legenda com marcação de tempo."},
	{WhisperRepo, "ggml-large-v3.bin", "Whisper large-v3", EngineWhisper, 3095033483, "", "A referência de precisão do whisper, e a mais lenta. Só quando nada abaixo dela resolver."},
}

func SpeechModels() []TSpeechModel {
	return speechModels
}

// SpeechEntries monta o catalogo fixo com peso real, veredito e nome de
// exibicao. O erro de rede nao esvazia a lista: os pesos de referencia mantem
// a tela utilizavel, e quem chama decide se avisa sobre a falha.
func SpeechEntries(ctx context.Context, c *TClient, machineProfile contract.TMachineProfile, modelsDir string) ([]contract.TCatalogEntry, string) {
	sizes := map[string]uint64{}
	catalogErr := ""
	for _, repo := range []string{ParakeetRepo, WhisperRepo} {
		files, err := c.TreeOf(ctx, repo, ".bin")
		if err != nil {
			if catalogErr == "" {
				if he, ok := err.(*contract.THelperError); ok {
					catalogErr = he.Message
				} else {
					catalogErr = err.Error()
				}
			}
			continue
		}
		for _, f := range files {
			sizes[repo+"/"+f.Path] = f.Size
		}
	}

	entries := make([]contract.TCatalogEntry, 0, len(speechModels))
	for _, m := range speechModels {
		size := m.FallbackSize
		if live, ok := sizes[m.RepoId+"/"+m.File]; ok && live > 0 {
			size = live
		}
		entries = append(entries, contract.TCatalogEntry{
			TModelFit:    fit.Speech(machineProfile.RamTotalBytes, machineProfile.RamAvailableBytes, size),
			Name:         m.Name,
			RepoId:       m.RepoId,
			File:         m.File,
			Quantization: m.Quantization,
			SizeBytes:    size,
			SizeGb:       fit.SizeGb(size),
			SizeLabel:    format.Bytes(size),
			Installed:    fileExists(filepath.Join(modelsDir, m.File)),
			Engine:       m.Engine,
			EngineLabel:  EngineLabel(m.Engine),
			Summary:      m.Summary,
			Recommended:  m.File == RecommendedSpeechFile,
		})
	}
	return entries, catalogErr
}

// EngineLabel e o nome do motor como uma pessoa o le. Fica aqui, ao lado da
// tabela que decide o motor, para que o rotulo nao possa divergir dela.
func EngineLabel(engine string) string {
	switch engine {
	case EngineParakeet:
		return "Parakeet"
	case EngineWhisper:
		return "Whisper"
	default:
		return ""
	}
}
