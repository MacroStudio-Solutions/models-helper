package catalog

import "testing"

func TestQuantToken(t *testing.T) {
	cases := map[string]string{
		"Qwen3-4B-Instruct-2507-Q4_K_M.gguf": "Q4_K_M",
		"model.IQ4_XS.gguf":                  "IQ4_XS",
		"gemma-3-12b-it-q8_0.gguf":           "Q8_0",
		"llama-2-7b.Q5_K_M.gguf":             "Q5_K_M",
		"model.gguf":                         "",
		"Qwen3-30B-A3B.gguf":                 "",
		"mistral-BF16.gguf":                  "BF16",
	}
	for file, want := range cases {
		if got := QuantToken(file); got != want {
			t.Fatalf("QuantToken(%s) = %q, queria %q", file, got, want)
		}
	}
}

func TestPickDefaultFilePrefersQ4KM(t *testing.T) {
	files := []TTreeFile{
		{Path: "m-Q8_0.gguf", Size: 8000},
		{Path: "m-Q4_K_M.gguf", Size: 4000},
		{Path: "m-Q2_K.gguf", Size: 2000},
	}
	got, ok := PickDefaultFile(files)
	if !ok || got.Path != "m-Q4_K_M.gguf" {
		t.Fatalf("esperado Q4_K_M, obtido %+v", got)
	}
}

func TestPickDefaultFileFallsBackToSmallest(t *testing.T) {
	files := []TTreeFile{
		{Path: "m-a.gguf", Size: 14000},
		{Path: "m-b.gguf", Size: 3000},
	}
	got, ok := PickDefaultFile(files)
	if !ok || got.Path != "m-b.gguf" {
		t.Fatalf("esperado menor arquivo sem quantizacao conhecida, obtido %+v", got)
	}
}

func TestPickDefaultFileEmpty(t *testing.T) {
	if _, ok := PickDefaultFile(nil); ok {
		t.Fatalf("lista vazia nao deve ter padrao")
	}
}

func TestDisplayName(t *testing.T) {
	if got := DisplayName("unsloth/Qwen3-4B-Instruct-2507-GGUF", "Q4_K_M"); got != "Qwen3-4B-Instruct-2507 Q4_K_M" {
		t.Fatalf("DisplayName = %q", got)
	}
	if got := DisplayName("org/plain-model", ""); got != "plain-model" {
		t.Fatalf("DisplayName sem token = %q", got)
	}
}
