package preset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withModelsRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("MODELS_HELPER_MODELS_ROOT", root)
	dir := filepath.Join(root, "llama-cpp")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSetWritesTheAliasSection(t *testing.T) {
	dir := withModelsRoot(t)
	model := filepath.Join(dir, "modelo.gguf")
	if err := os.WriteFile(model, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	state, herr := Set(model)
	if herr != nil {
		t.Fatalf("erro inesperado: %v", herr)
	}
	if !state.HasModel || state.Alias != "studio-local" {
		t.Fatalf("estado do preset errado: %+v", state)
	}

	body, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "[studio-local]") || !strings.Contains(string(body), "modelo.gguf") {
		t.Fatalf("preset gravado errado: %s", body)
	}
	if Read().ModelPath != state.ModelPath {
		t.Fatalf("releitura divergiu da escrita")
	}
}

// O preset vira linha de comando de um processo servidor: um caminho fora do
// diretorio de modelos e recusado pelo mesmo guarda que protege download e
// remocao.
func TestSetRefusesAPathOutsideTheModelsDirectory(t *testing.T) {
	withModelsRoot(t)
	outside := filepath.Join(t.TempDir(), "alheio.gguf")
	if err := os.WriteFile(outside, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, herr := Set(outside); herr == nil {
		t.Fatalf("caminho externo deveria ser recusado")
	}
}

func TestSetRefusesAMissingModel(t *testing.T) {
	dir := withModelsRoot(t)
	if _, herr := Set(filepath.Join(dir, "inexistente.gguf")); herr == nil || herr.Code != "MODEL_NOT_FOUND" {
		t.Fatalf("modelo ausente deveria ter erro nomeado, obtido %v", herr)
	}
}

func TestEnsureCreatesAnEmptyPresetAndClearEmptiesIt(t *testing.T) {
	dir := withModelsRoot(t)
	if _, herr := Ensure(); herr != nil {
		t.Fatalf("ensure falhou: %v", herr)
	}
	if _, err := os.Stat(Path()); err != nil {
		t.Fatalf("preset nao criado: %v", err)
	}
	if Read().HasModel {
		t.Fatalf("preset vazio nao deve declarar modelo")
	}

	model := filepath.Join(dir, "modelo.gguf")
	if err := os.WriteFile(model, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, herr := Set(model); herr != nil {
		t.Fatal(herr)
	}
	state, herr := Clear()
	if herr != nil {
		t.Fatal(herr)
	}
	if state.HasModel {
		t.Fatalf("clear deveria remover o padrao: %+v", state)
	}
}
