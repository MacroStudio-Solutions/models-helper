package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func llamaServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

func TestRouterModeReportsEveryModel(t *testing.T) {
	srv := llamaServer(t, `{"data":[
		{"id":"Qwen3-4B","source":"models_dir","status":{"value":"unloaded"}},
		{"id":"studio-local","source":"preset","status":{"value":"loaded"}}
	]}`)
	defer srv.Close()

	state := ProbeLlama(srv.URL)
	if !state.Online || state.Mode != "router" {
		t.Fatalf("modo roteador nao reconhecido: %+v", state)
	}
	if state.ModelCount != 2 || state.LoadedCount != 1 {
		t.Fatalf("contagem errada: %+v", state)
	}
	if state.ModelId != "studio-local" {
		t.Fatalf("modelId deve nomear o que esta carregado: %+v", state)
	}
	if !state.HasDefault || state.DefaultName != "studio-local" {
		t.Fatalf("padrao nao reconhecido: %+v", state)
	}
	if state.Models[0].StateLabel != "em disco" || state.Models[1].StateLabel != "carregado" {
		t.Fatalf("rotulos de estado errados: %+v", state.Models)
	}
}

// O servidor de modelo fixo continua sendo lido: a extensao ainda pode estar
// servindo um servidor iniciado por uma versao anterior da tela.
func TestSingleModelServerIsStillReadable(t *testing.T) {
	srv := llamaServer(t, `{"data":[{"id":"studio-local","status":{"value":""}}]}`)
	defer srv.Close()

	state := ProbeLlama(srv.URL)
	if state.Mode != "single" || state.ModelId != "studio-local" {
		t.Fatalf("servidor de modelo fixo mal lido: %+v", state)
	}
}

func TestOfflineServerIsNotAFailure(t *testing.T) {
	state := ProbeLlama("http://127.0.0.1:1")
	if state.Online || state.Mode != "" {
		t.Fatalf("servidor fora do ar deveria ser estado, nao erro: %+v", state)
	}
	if state.Models == nil {
		t.Fatalf("a lista de modelos nunca deve ser nula, para a tela poder iterar")
	}
}

func TestWhisperProbeReadsTheStartRecord(t *testing.T) {
	dir := t.TempDir()
	record, _ := json.Marshal(map[string]string{"model": "ggml-large-v3-turbo.bin", "path": filepath.Join(dir, "ggml-large-v3-turbo.bin")})
	if err := os.WriteFile(WhisperStatePath(dir), record, 0644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	state := ProbeWhisper(srv.URL, dir)
	if !state.Online || !state.HasModelName || state.ModelName != "ggml-large-v3-turbo.bin" {
		t.Fatalf("estado do servidor de transcricao errado: %+v", state)
	}
	if state.InferenceUrl != srv.URL+"/inference" {
		t.Fatalf("endereco de inferencia errado: %s", state.InferenceUrl)
	}
}

// Sem o registro, o servidor continua sendo reportado como no ar: o que se
// perde e o nome do modelo, e afirmar um nome errado seria pior.
func TestWhisperProbeWithoutRecordStaysOnline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	state := ProbeWhisper(srv.URL, t.TempDir())
	if !state.Online || state.HasModelName || state.ModelName != "" {
		t.Fatalf("sem registro o nome deve ficar vazio: %+v", state)
	}
}
