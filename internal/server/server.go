// Leitura do estado dos dois servidores locais.
package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MacroStudio-Solutions/models-helper/internal/contract"
)

const probeTimeout = 2 * time.Second

// DefaultAlias e o nome pelo qual um agente ja configurado encontra o modelo
// padrao. Ele sobrevive a troca para o modo roteador porque continua sendo um
// preset — o servidor passa a expor varios modelos, e este continua sendo um
// deles.
const DefaultAlias = "studio-local"

type tLlamaModel struct {
	Id     string `json:"id"`
	Source string `json:"source"`
	Status struct {
		Value string `json:"value"`
	} `json:"status"`
}

type tLlamaModels struct {
	Data []tLlamaModel `json:"data"`
}

// ProbeLlama le /v1/models.
//
// Em modo roteador a pergunta "qual modelo esta em execucao" deixa de ter uma
// resposta so: o servidor conhece varios e carrega sob demanda o que a
// requisicao nomear. Entao o estado devolve a lista inteira, e mantem modelId
// preenchido com o que estiver carregado para nao quebrar quem ja lia esse
// campo.
func ProbeLlama(baseUrl string) contract.TServerState {
	state := contract.TServerState{BaseUrl: baseUrl + "/v1", Models: []contract.TServerModel{}}
	client := &http.Client{Timeout: probeTimeout}
	resp, err := client.Get(baseUrl + "/v1/models")
	if err != nil {
		return state
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return state
	}
	state.Online = true

	var payload tLlamaModels
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return state
	}

	fromPreset := 0
	for _, m := range payload.Data {
		loaded := m.Status.Value != "" && m.Status.Value != "unloaded"
		entry := contract.TServerModel{
			Id:         m.Id,
			Loaded:     loaded,
			StateLabel: stateLabel(m.Status.Value),
			IsDefault:  m.Id == DefaultAlias,
			FromPreset: m.Source == "preset",
		}
		if entry.FromPreset {
			fromPreset++
		}
		if entry.IsDefault {
			state.HasDefault = true
			state.DefaultName = m.Id
		}
		if loaded {
			state.LoadedCount++
			if state.ModelId == "" {
				state.ModelId = m.Id
			}
		}
		state.Models = append(state.Models, entry)
	}
	state.ModelCount = len(state.Models)

	// Um servidor de modelo fixo expoe exatamente um modelo, e ele nunca vem de
	// diretorio nem de preset. Mais de um, ou um vindo do diretorio, so acontece
	// em modo roteador.
	if state.ModelCount > 1 || fromPreset > 0 || hasDirSource(payload.Data) {
		state.Mode = "router"
	} else if state.ModelCount == 1 {
		state.Mode = "single"
		if state.ModelId == "" {
			state.ModelId = payload.Data[0].Id
		}
	}
	return state
}

func hasDirSource(models []tLlamaModel) bool {
	for _, m := range models {
		if m.Source == "models_dir" {
			return true
		}
	}
	return false
}

func stateLabel(value string) string {
	switch value {
	case "", "unloaded":
		return "em disco"
	case "loading":
		return "carregando"
	default:
		return "carregado"
	}
}

type tWhisperRecord struct {
	Model string `json:"model"`
	Path  string `json:"path"`
}

// WhisperStatePath e o registro que o roteiro de ligar o servidor grava.
// O whisper-server nao expoe qual modelo carregou, e perguntar isso a lista de
// processos nao atravessa as plataformas declaradas.
func WhisperStatePath(modelsDir string) string {
	return filepath.Join(modelsDir, ".server.json")
}

// ProbeWhisper pergunta ao servidor de transcricao se ele esta no ar. O
// whisper-server nao tem endpoint de saude nem de modelos: a raiz serve a
// pagina de exemplo e responde 200 quando o processo esta pronto.
func ProbeWhisper(baseUrl string, modelsDir string) contract.TTranscriptionServer {
	state := contract.TTranscriptionServer{
		BaseUrl:      baseUrl,
		InferenceUrl: baseUrl + "/inference",
	}
	client := &http.Client{Timeout: probeTimeout}
	resp, err := client.Get(baseUrl + "/")
	if err != nil {
		return state
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return state
	}
	state.Online = true

	data, err := os.ReadFile(WhisperStatePath(modelsDir))
	if err != nil {
		return state
	}
	var record tWhisperRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return state
	}
	name := strings.TrimSpace(record.Model)
	if name == "" && record.Path != "" {
		name = filepath.Base(record.Path)
	}
	if name == "" {
		return state
	}
	state.ModelName = name
	state.ModelPath = record.Path
	state.HasModelName = true
	return state
}
