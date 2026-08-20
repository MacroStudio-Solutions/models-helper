package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWithinAcceptsInside(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "llama-cpp")
	if _, herr := ResolveWithin(root, inner); herr != nil {
		t.Fatalf("caminho interno recusado: %v", herr)
	}
}

func TestResolveWithinRefusesOutside(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if _, herr := ResolveWithin(root, filepath.Join(outside, "x.gguf")); herr == nil {
		t.Fatalf("caminho externo aceito")
	}
}

func TestResolveWithinRefusesRootItself(t *testing.T) {
	root := t.TempDir()
	if _, herr := ResolveWithin(root, root); herr == nil {
		t.Fatalf("propria raiz aceita")
	}
}

func TestResolveWithinRefusesTraversal(t *testing.T) {
	root := t.TempDir()
	if _, herr := ResolveWithin(root, filepath.Join(root, "..", "escape.gguf")); herr == nil {
		t.Fatalf("travessia aceita")
	}
}

func TestResolveWithinRefusesSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink indisponivel: %v", err)
	}
	if _, herr := ResolveWithin(root, filepath.Join(link, "x.gguf")); herr == nil {
		t.Fatalf("escape por link simbolico aceito")
	}
}

func TestSafeRelFile(t *testing.T) {
	bad := []string{"", "/abs.gguf", "../up.gguf", "..", "."}
	for _, f := range bad {
		if herr := SafeRelFile(f); herr == nil {
			t.Fatalf("SafeRelFile(%q) aceito", f)
		}
	}
	good := []string{"model.gguf", "sub/model.gguf"}
	for _, f := range good {
		if herr := SafeRelFile(f); herr != nil {
			t.Fatalf("SafeRelFile(%q) recusado: %v", f, herr)
		}
	}
}

func TestSafeRepoId(t *testing.T) {
	bad := []string{"", "sem-barra", "a/b/c", "../x", "a/..", "a\\b"}
	for _, r := range bad {
		if herr := SafeRepoId(r); herr == nil {
			t.Fatalf("SafeRepoId(%q) aceito", r)
		}
	}
	if herr := SafeRepoId("unsloth/Qwen3-4B-Instruct-2507-GGUF"); herr != nil {
		t.Fatalf("repo valido recusado: %v", herr)
	}
}
