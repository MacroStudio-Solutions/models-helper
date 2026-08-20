package inventory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MacroStudio-Solutions/models-helper/internal/contract"
)

func TestListOnlyGgufFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "model.gguf"), make([]byte, 2048), 0644)
	os.WriteFile(filepath.Join(dir, ".model.gguf.part"), make([]byte, 1024), 0644)
	os.WriteFile(filepath.Join(dir, "model.gguf.download.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	items, err := List(dir, contract.TMachineProfile{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("esperado 1 item, obtido %d", len(items))
	}
	if items[0].Name != "model.gguf" || items[0].SizeBytes != 2048 {
		t.Fatalf("item inesperado: %+v", items[0])
	}
	if items[0].SizeGb != "0.0" {
		t.Fatalf("sizeGb %s", items[0].SizeGb)
	}
	if items[0].Path != filepath.Join(dir, "model.gguf") {
		t.Fatalf("path %s", items[0].Path)
	}
}

func TestListMissingDirIsEmpty(t *testing.T) {
	items, err := List(filepath.Join(t.TempDir(), "nao-existe"), contract.TMachineProfile{})
	if err != nil || len(items) != 0 {
		t.Fatalf("esperado vazio sem erro, obtido %v %+v", err, items)
	}
}
