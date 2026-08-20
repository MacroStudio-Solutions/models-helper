package contract

import (
	"encoding/json"
	"testing"
)

func keys(t *testing.T, v any) map[string]any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", data, err)
	}
	return m
}

func assertKeys(t *testing.T, got map[string]any, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("campoCount %d != %d: %v", len(got), len(want), got)
	}
	for _, k := range want {
		if _, ok := got[k]; !ok {
			t.Fatalf("campo %s ausente em %v", k, got)
		}
	}
}

func TestEnvelopeShape(t *testing.T) {
	ok := &THelperEnvelope[map[string]any]{SchemaVersion: 1, Ok: true}
	data := map[string]any{"x": 1}
	ok.Data = &data
	m := keys(t, ok)
	assertKeys(t, m, []string{"schemaVersion", "ok", "data"})

	fail := &THelperEnvelope[map[string]any]{SchemaVersion: 1, Ok: false, Error: &THelperError{Code: "X", Message: "y"}}
	m = keys(t, fail)
	assertKeys(t, m, []string{"schemaVersion", "ok", "error"})

	err := keys(t, fail.Error)
	assertKeys(t, err, []string{"code", "message"})
}

func TestMachineProfileShape(t *testing.T) {
	assertKeys(t, keys(t, TMachineProfile{}), []string{
		"ramTotalBytes", "ramAvailableBytes", "cpuCores", "hasGpu", "gpuName",
		"vramBytes", "hasVulkan", "vulkanUnavailableReason",
	})
}

func TestModelFitShape(t *testing.T) {
	assertKeys(t, keys(t, TModelFit{}), []string{"fitOk", "fitTight", "fitGpu", "requiredBytes"})
}

func TestCatalogEntryShape(t *testing.T) {
	entry := TCatalogEntry{Name: "n", RepoId: "o/r", File: "f.gguf", Quantization: "Q4_K_M", SizeBytes: 1, SizeGb: "0.0", Installed: false}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != `{"fitOk":false,"fitTight":false,"fitGpu":false,"requiredBytes":0,"name":"n","repoId":"o/r","file":"f.gguf","quantization":"Q4_K_M","sizeBytes":1,"sizeGb":"0.0","installed":false,"download":null}` {
		t.Fatalf("json inesperado: %s", data)
	}
}

func TestInstalledModelShape(t *testing.T) {
	assertKeys(t, keys(t, TInstalledModel{}), []string{
		"fitOk", "fitTight", "fitGpu", "requiredBytes", "name", "path", "sizeBytes", "sizeGb",
	})
}

func TestDownloadJobShape(t *testing.T) {
	assertKeys(t, keys(t, TDownloadJob{}), []string{
		"jobId", "repoId", "file", "destination", "state", "receivedBytes",
		"totalBytes", "percent", "pid", "startedAt", "updatedAt", "error",
	})
}

func TestLocalModelsStatusShape(t *testing.T) {
	st := TLocalModelsStatus{Installed: []TInstalledModel{}, Catalog: []TCatalogEntry{}}
	m := keys(t, st)
	assertKeys(t, m, []string{
		"runtime", "server", "machine", "installed", "hasInstalled", "catalog", "catalogError",
	})
	runtimeM, ok := m["runtime"].(map[string]any)
	if !ok {
		t.Fatalf("runtime nao e objeto: %v", m["runtime"])
	}
	assertKeys(t, runtimeM, []string{"ok", "error", "recommendedRuntimeId", "recommendationReason"})
	serverM, ok := m["server"].(map[string]any)
	if !ok {
		t.Fatalf("server nao e objeto: %v", m["server"])
	}
	assertKeys(t, serverM, []string{"online", "modelId", "baseUrl"})
}

func TestDownloadJobPointerMarshalsNull(t *testing.T) {
	type wrapper struct {
		Download *TDownloadJob `json:"download"`
	}
	data, _ := json.Marshal(wrapper{})
	if string(data) != `{"download":null}` {
		t.Fatalf("esperado null, obtido %s", data)
	}
}
