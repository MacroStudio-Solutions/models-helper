package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MacroStudio-Solutions/models-helper/internal/contract"
)

func entry(name string, rank int, size uint64, installed bool) contract.TCatalogEntry {
	e := contract.TCatalogEntry{Name: name, SizeBytes: size, Installed: installed}
	e.FitRank = rank
	e.FitGpu = rank == contract.FitRankGpu
	e.FitOk = rank == contract.FitRankOk
	e.FitTight = rank == contract.FitRankTight
	return e
}

func names(entries []contract.TCatalogEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name)
	}
	return out
}

// A ordenacao por viabilidade nao pode perder a popularidade: dentro de um
// mesmo veredito, a ordem de chegada da API e o unico sinal de qualidade que
// existe, e um sort instavel a embaralharia.
func TestSortByFitIsStableWithinARank(t *testing.T) {
	entries := []contract.TCatalogEntry{
		entry("grande", contract.FitRankNo, 90, false),
		entry("primeiro-ok", contract.FitRankOk, 10, false),
		entry("apertado", contract.FitRankTight, 40, false),
		entry("segundo-ok", contract.FitRankOk, 20, false),
		entry("gpu", contract.FitRankGpu, 30, false),
	}
	SortEntries(entries, SortFit)
	got := strings.Join(names(entries), ",")
	if got != "gpu,primeiro-ok,segundo-ok,apertado,grande" {
		t.Fatalf("ordem por viabilidade errada: %s", got)
	}
}

func TestSortByPopularityLeavesTheApiOrderAlone(t *testing.T) {
	entries := []contract.TCatalogEntry{
		entry("grande", contract.FitRankNo, 90, false),
		entry("pequeno", contract.FitRankOk, 10, false),
	}
	SortEntries(entries, SortPopularity)
	if names(entries)[0] != "grande" {
		t.Fatalf("popularidade nao deveria reordenar: %v", names(entries))
	}
}

func TestFilterDropsWhatTheMachineCannotRun(t *testing.T) {
	entries := []contract.TCatalogEntry{
		entry("cabe", contract.FitRankOk, 10, false),
		entry("nao-cabe", contract.FitRankNo, 90, false),
		entry("apertado", contract.FitRankTight, 40, false),
	}
	got := names(FilterByFit(entries, FitFits))
	if strings.Join(got, ",") != "cabe,apertado" {
		t.Fatalf("filtro de viabilidade errado: %v", got)
	}
	if len(FilterByFit(entries, FitAny)) != 3 {
		t.Fatalf("o modo any nao filtra nada")
	}
}

// Um modelo ja baixado nunca some da lista: escondê-lo tiraria do operador o
// unico lugar onde ele ve que o arquivo esta ocupando disco.
func TestFilterKeepsInstalledEvenWhenItDoesNotFit(t *testing.T) {
	entries := []contract.TCatalogEntry{entry("baixado-grande", contract.FitRankNo, 90, true)}
	if len(FilterByFit(entries, FitFits)) != 1 {
		t.Fatalf("modelo instalado foi escondido pelo filtro")
	}
	if len(FilterByFit(entries, FitGpu)) != 1 {
		t.Fatalf("modelo instalado foi escondido pelo filtro de GPU")
	}
}

func TestLimitNeverGrowsTheList(t *testing.T) {
	entries := []contract.TCatalogEntry{entry("a", 1, 1, false), entry("b", 1, 2, false)}
	if len(Limit(entries, 5)) != 2 {
		t.Fatalf("limite maior que a lista nao deve alterar nada")
	}
	if len(Limit(entries, 1)) != 1 {
		t.Fatalf("limite nao aplicado")
	}
	if len(Limit(entries, 0)) != 2 {
		t.Fatalf("limite zero significa sem limite")
	}
}

func TestModeValidation(t *testing.T) {
	if IsSortMode("aleatorio") || IsFitMode("talvez") {
		t.Fatalf("modo invalido aceito")
	}
	for _, m := range []string{SortFit, SortPopularity, SortSize} {
		if !IsSortMode(m) {
			t.Fatalf("modo de ordem rejeitado: %s", m)
		}
	}
	for _, m := range []string{FitAny, FitFits, FitGpu} {
		if !IsFitMode(m) {
			t.Fatalf("modo de viabilidade rejeitado: %s", m)
		}
	}
}

func speechServer(t *testing.T, fail bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !strings.Contains(r.URL.Path, "/tree/main") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		files := []map[string]any{
			{"path": "ggml-parakeet-tdt-0.6b-v3-q8_0.bin", "type": "file", "size": 999},
			{"path": "README.md", "type": "file", "size": 10},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(files)
	}))
}

func TestSpeechEntriesPreferLiveSizesAndKeepTheEditorialOrder(t *testing.T) {
	srv := speechServer(t, false)
	defer srv.Close()
	client := &TClient{BaseURL: srv.URL, HTTP: srv.Client(), CacheDir: t.TempDir()}

	entries, catalogErr := SpeechEntries(context.Background(), client, contract.TMachineProfile{RamTotalBytes: 1 << 34, RamAvailableBytes: 1 << 33}, t.TempDir())
	if catalogErr != "" {
		t.Fatalf("erro inesperado: %s", catalogErr)
	}
	if len(entries) != len(SpeechModels()) {
		t.Fatalf("catalogo fixo mudou de tamanho: %d", len(entries))
	}
	if entries[0].File != RecommendedSpeechFile || !entries[0].Recommended {
		t.Fatalf("a recomendacao deve abrir a lista: %+v", entries[0])
	}
	if entries[0].SizeBytes != 999 {
		t.Fatalf("tamanho da arvore deveria prevalecer: %d", entries[0].SizeBytes)
	}
	if entries[1].SizeBytes == 0 {
		t.Fatalf("um arquivo ausente da arvore deve cair no tamanho de referencia")
	}
	for _, e := range entries {
		if e.Summary == "" || e.Engine == "" || e.SizeLabel == "" {
			t.Fatalf("entrada sem texto editorial ou rotulo: %+v", e)
		}
	}
}

// A tela nao pode ficar vazia porque a API publica caiu: os pesos sao arquivos
// de release imutaveis, e o tamanho medido continua valendo.
func TestSpeechEntriesSurviveARemoteFailure(t *testing.T) {
	srv := speechServer(t, true)
	defer srv.Close()
	client := &TClient{BaseURL: srv.URL, HTTP: srv.Client(), CacheDir: t.TempDir()}

	entries, catalogErr := SpeechEntries(context.Background(), client, contract.TMachineProfile{RamTotalBytes: 1 << 34, RamAvailableBytes: 1 << 33}, t.TempDir())
	if catalogErr == "" {
		t.Fatalf("a falha remota deve ser reportada")
	}
	if len(entries) != len(SpeechModels()) {
		t.Fatalf("o catalogo fixo deve sobreviver a falha remota: %d", len(entries))
	}
	for _, e := range entries {
		if e.SizeBytes == 0 {
			t.Fatalf("entrada sem tamanho de referencia: %+v", e)
		}
	}
}

func TestCuratedListIsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range CuratedRepos() {
		if seen[m.RepoId] {
			t.Fatalf("repositorio repetido na vitrine: %s", m.RepoId)
		}
		seen[m.RepoId] = true
		if strings.Count(m.RepoId, "/") != 1 {
			t.Fatalf("identificador fora da forma org/nome: %s", m.RepoId)
		}
		if m.Summary == "" {
			t.Fatalf("modelo da vitrine sem justificativa editorial: %s", m.RepoId)
		}
	}
}
