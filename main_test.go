package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "models-helper-bin-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binPath = filepath.Join(dir, "models-helper")
	build := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := build.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		fmt.Fprintf(os.Stderr, "build falhou: %v\n%s\n", err, out)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type TRunResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Env      map[string]any
}

func runHelper(t *testing.T, args ...string) TRunResult {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	exit := 0
	if ee, ok := runErr.(*exec.ExitError); ok {
		exit = ee.ExitCode()
	} else if runErr != nil {
		t.Fatalf("exec falhou: %v", runErr)
	}
	stdout := out.String()
	dec := json.NewDecoder(strings.NewReader(stdout))
	var env map[string]any
	if err := dec.Decode(&env); err != nil {
		t.Fatalf("stdout nao e um objeto JSON: %q (stderr %q)", stdout, errBuf.String())
	}
	if dec.More() {
		t.Fatalf("stdout tem mais de um objeto JSON: %q", stdout)
	}
	return TRunResult{ExitCode: exit, Stdout: stdout, Stderr: errBuf.String(), Env: env}
}

func dataMap(t *testing.T, r TRunResult) map[string]any {
	t.Helper()
	d, ok := r.Env["data"].(map[string]any)
	if !ok {
		t.Fatalf("data nao e objeto: %v", r.Env)
	}
	return d
}

func dataList(t *testing.T, r TRunResult) []any {
	t.Helper()
	d, ok := r.Env["data"].([]any)
	if !ok {
		t.Fatalf("data nao e lista: %v", r.Env)
	}
	return d
}

func errCode(t *testing.T, r TRunResult) string {
	t.Helper()
	e, ok := r.Env["error"].(map[string]any)
	if !ok {
		t.Fatalf("error ausente em %v", r.Env)
	}
	code, _ := e["code"].(string)
	return code
}

func setupRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("MODELS_HELPER_MODELS_ROOT", root)
	return root
}

func writeFakeStudio(t *testing.T, exitCode int, message string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "studio-fake")
	script := "#!/bin/sh\nprintf '%s\\n' '" + message + "' >&2\nexit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("err: %v", err)
	}
	t.Setenv("MODELS_HELPER_STUDIO_BIN", path)
	return path
}

func downloadHandler(total int, chunk int, delay time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(total))
		if r.Method == http.MethodHead {
			return
		}
		buf := make([]byte, chunk)
		written := 0
		for written < total {
			n := chunk
			if total-written < chunk {
				n = total - written
			}
			if _, err := w.Write(buf[:n]); err != nil {
				return
			}
			written += n
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			if delay > 0 {
				time.Sleep(delay)
			}
		}
	}
}

func newFakeHf(t *testing.T, listBody string, treeBody string, dl http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, listBody)
	})
	mux.HandleFunc("/api/models/org/tiny/tree/main", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, treeBody)
	})
	if dl != nil {
		mux.HandleFunc("/org/tiny/resolve/main/tiny.gguf", dl)
	}
	ts := httptest.NewServer(mux)
	t.Setenv("MODELS_HELPER_HF_API", ts.URL)
	t.Cleanup(ts.Close)
	return ts
}

func pollSidecar(t *testing.T, path string, timeout time.Duration, check func(map[string]any) bool) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			var m map[string]any
			if json.Unmarshal(data, &m) == nil && check(m) {
				return m
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("condicao nao alcancada em %s para %s", timeout, path)
	return nil
}

func TestMachineEnvelope(t *testing.T) {
	setupRoot(t)
	r := runHelper(t, "machine")
	if r.ExitCode != 0 || r.Env["ok"] != true || r.Env["schemaVersion"] != float64(1) {
		t.Fatalf("envelope invalido: %s", r.Stdout)
	}
	data := dataMap(t, r)
	for _, k := range []string{"ramTotalBytes", "ramAvailableBytes", "cpuCores", "hasGpu", "gpuName", "vramBytes", "hasVulkan", "vulkanUnavailableReason"} {
		if _, ok := data[k]; !ok {
			t.Fatalf("campo %s ausente: %s", k, r.Stdout)
		}
	}
}

func TestUsageErrorsAreNamedEnvelopes(t *testing.T) {
	setupRoot(t)
	r := runHelper(t, "comando-inexistente")
	if r.ExitCode == 0 || errCode(t, r) != "INVALID_USAGE" {
		t.Fatalf("esperado INVALID_USAGE com saida nao zero: %s", r.Stdout)
	}
	r = runHelper(t, "catalog")
	if r.ExitCode == 0 || errCode(t, r) != "INVALID_USAGE" {
		t.Fatalf("catalog sem subcomando: %s", r.Stdout)
	}
	r = runHelper(t, "download", "start")
	if r.ExitCode == 0 || errCode(t, r) != "INVALID_USAGE" {
		t.Fatalf("download start sem flags: %s", r.Stdout)
	}
	r = runHelper(t, "remove")
	if r.ExitCode == 0 || errCode(t, r) != "INVALID_USAGE" {
		t.Fatalf("remove sem path: %s", r.Stdout)
	}
	r = runHelper(t, "status", "--profile", "outra-coisa")
	if r.ExitCode == 0 || errCode(t, r) != "UNSUPPORTED_PROFILE" {
		t.Fatalf("perfil desconhecido: %s", r.Stdout)
	}
}

func TestCatalogListEndToEnd(t *testing.T) {
	setupRoot(t)
	hits := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/api/models", func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"id":"org/tiny"}]`)
	})
	mux.HandleFunc("/api/models/org/tiny/tree/main", func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"path":"tiny-Q4_K_M.gguf","type":"file","size":1000},{"path":"tiny-Q8_0.gguf","type":"file","size":2000}]`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	t.Setenv("MODELS_HELPER_HF_API", ts.URL)

	r := runHelper(t, "catalog", "list", "--limit", "5")
	if r.ExitCode != 0 || r.Env["ok"] != true {
		t.Fatalf("catalog list falhou: %s / %s", r.Stdout, r.Stderr)
	}
	list := dataList(t, r)
	if len(list) != 1 {
		t.Fatalf("esperado 1 entrada: %s", r.Stdout)
	}
	entry := list[0].(map[string]any)
	for _, k := range []string{"name", "repoId", "file", "quantization", "sizeBytes", "sizeGb", "installed", "download", "fitOk", "fitTight", "fitGpu", "requiredBytes"} {
		if _, ok := entry[k]; !ok {
			t.Fatalf("campo %s ausente: %s", k, r.Stdout)
		}
	}
	if entry["file"] != "tiny-Q4_K_M.gguf" || entry["download"] != nil {
		t.Fatalf("entrada inesperada: %s", r.Stdout)
	}
	r = runHelper(t, "catalog", "list", "--limit", "5")
	if hits != 2 {
		t.Fatalf("segunda leitura deve usar cache, hits %d", hits)
	}
}

func TestCatalogSearchAndVersions(t *testing.T) {
	setupRoot(t)
	newFakeHf(t, `[{"id":"org/tiny"}]`, `[{"path":"tiny-Q4_K_M.gguf","type":"file","size":1000},{"path":"tiny-Q8_0.gguf","type":"file","size":2000}]`, nil)
	r := runHelper(t, "catalog", "search", "tiny")
	if r.ExitCode != 0 || len(dataList(t, r)) != 1 {
		t.Fatalf("search: %s", r.Stdout)
	}
	r = runHelper(t, "catalog", "versions", "org/tiny")
	if r.ExitCode != 0 {
		t.Fatalf("versions: %s", r.Stdout)
	}
	if len(dataList(t, r)) != 2 {
		t.Fatalf("versions deve listar todas as variantes: %s", r.Stdout)
	}
	r = runHelper(t, "catalog", "versions", "invalido")
	if r.ExitCode == 0 || errCode(t, r) != "INVALID_REPO_ID" {
		t.Fatalf("repo invalido: %s", r.Stdout)
	}
}

func TestInstalledCommand(t *testing.T) {
	setupRoot(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "modelo.gguf"), make([]byte, 4096), 0644)
	os.WriteFile(filepath.Join(dir, ".modelo.gguf.part"), make([]byte, 10), 0644)
	r := runHelper(t, "installed", "--dir", dir)
	if r.ExitCode != 0 {
		t.Fatalf("installed: %s", r.Stdout)
	}
	list := dataList(t, r)
	if len(list) != 1 {
		t.Fatalf("esperado apenas gguf: %s", r.Stdout)
	}
	item := list[0].(map[string]any)
	for _, k := range []string{"name", "path", "sizeBytes", "sizeGb", "fitOk", "fitTight", "fitGpu", "requiredBytes"} {
		if _, ok := item[k]; !ok {
			t.Fatalf("campo %s ausente: %s", k, r.Stdout)
		}
	}
}

func TestDownloadLifecycleComplete(t *testing.T) {
	root := setupRoot(t)
	newFakeHf(t, `[{"id":"org/tiny"}]`, `[{"path":"tiny.gguf","type":"file","size":1048576}]`, downloadHandler(1048576, 65536, 0))
	dest := filepath.Join(root, "llama-cpp")

	start := runHelper(t, "download", "start", "--repo", "org/tiny", "--file", "tiny.gguf", "--dest", dest)
	if start.ExitCode != 0 || start.Env["ok"] != true {
		t.Fatalf("start falhou: %s / %s", start.Stdout, start.Stderr)
	}
	job := dataMap(t, start)
	jobId, _ := job["jobId"].(string)
	if jobId == "" || job["state"] != "running" || job["totalBytes"] != float64(1048576) {
		t.Fatalf("job inicial invalido: %s", start.Stdout)
	}
	if job["pid"] == float64(0) {
		t.Fatalf("pid deve ser registrado no inicio")
	}

	sc := filepath.Join(dest, "tiny.gguf.download.json")
	final := pollSidecar(t, sc, 15*time.Second, func(m map[string]any) bool { return m["state"] == "completed" })
	if final["receivedBytes"] != float64(1048576) || final["percent"] != float64(100) {
		t.Fatalf("job concluido invalido: %v", final)
	}
	if st, err := os.Stat(filepath.Join(dest, "tiny.gguf")); err != nil || st.Size() != 1048576 {
		t.Fatalf("arquivo final ausente ou com tamanho errado")
	}
	if _, err := os.Stat(filepath.Join(dest, ".tiny.gguf.part")); !os.IsNotExist(err) {
		t.Fatalf("temporario deve sumir apos a conclusao")
	}

	status := runHelper(t, "download", "status", "--job", jobId)
	if status.ExitCode != 0 {
		t.Fatalf("status: %s", status.Stdout)
	}
	jobs := dataList(t, status)
	if len(jobs) != 1 || jobs[0].(map[string]any)["state"] != "completed" {
		t.Fatalf("status do job: %s", status.Stdout)
	}
}

func TestDownloadCompleteOverlapsAsInstalled(t *testing.T) {
	root := setupRoot(t)
	newFakeHf(t, `[{"id":"org/tiny"}]`, `[{"path":"tiny.gguf","type":"file","size":262144}]`, downloadHandler(262144, 65536, 0))
	dest := filepath.Join(root, "llama-cpp")

	start := runHelper(t, "download", "start", "--repo", "org/tiny", "--file", "tiny.gguf", "--dest", dest)
	if start.ExitCode != 0 {
		t.Fatalf("start: %s", start.Stdout)
	}
	sc := filepath.Join(dest, "tiny.gguf.download.json")
	pollSidecar(t, sc, 15*time.Second, func(m map[string]any) bool { return m["state"] == "completed" })

	r := runHelper(t, "catalog", "list")
	list := dataList(t, r)
	if len(list) != 1 {
		t.Fatalf("catalog: %s", r.Stdout)
	}
	entry := list[0].(map[string]any)
	if entry["installed"] != true {
		t.Fatalf("entrada deveria estar instalada: %s", r.Stdout)
	}
	if entry["download"] != nil {
		t.Fatalf("trabalho concluido e ja instalado deve ficar invisivel (download null): %s", r.Stdout)
	}
}

func TestStatusOverlapsCompletedDownloadAsInstalled(t *testing.T) {
	root := setupRoot(t)
	writeFakeStudio(t, 0, "")
	newFakeHf(t, `[{"id":"org/tiny"}]`, `[{"path":"tiny.gguf","type":"file","size":262144}]`, downloadHandler(262144, 65536, 0))
	dest := filepath.Join(root, "llama-cpp")

	start := runHelper(t, "download", "start", "--repo", "org/tiny", "--file", "tiny.gguf", "--dest", dest)
	if start.ExitCode != 0 {
		t.Fatalf("start: %s", start.Stdout)
	}
	sc := filepath.Join(dest, "tiny.gguf.download.json")
	pollSidecar(t, sc, 15*time.Second, func(m map[string]any) bool { return m["state"] == "completed" })

	r := runHelper(t, "status", "--profile", "local-models")
	if r.ExitCode != 0 {
		t.Fatalf("status: %s / %s", r.Stdout, r.Stderr)
	}
	data := dataMap(t, r)
	catalog := data["catalog"].([]any)
	if len(catalog) != 1 {
		t.Fatalf("catalog: %s", r.Stdout)
	}
	entry := catalog[0].(map[string]any)
	if entry["installed"] != true {
		t.Fatalf("entrada deveria estar instalada na leitura composta: %s", r.Stdout)
	}
	if entry["download"] != nil {
		t.Fatalf("trabalho concluido e ja instalado deve ficar invisivel na leitura composta: %s", r.Stdout)
	}
	if len(data["installed"].([]any)) != 1 || data["hasInstalled"] != true {
		t.Fatalf("inventario da leitura composta: %s", r.Stdout)
	}
}

func TestDownloadStartRefusesDuplicate(t *testing.T) {
	root := setupRoot(t)
	newFakeHf(t, `[{"id":"org/tiny"}]`, `[{"path":"tiny.gguf","type":"file","size":50000000}]`, downloadHandler(50000000, 65536, 20*time.Millisecond))
	dest := filepath.Join(root, "llama-cpp")

	first := runHelper(t, "download", "start", "--repo", "org/tiny", "--file", "tiny.gguf", "--dest", dest)
	if first.ExitCode != 0 {
		t.Fatalf("start: %s", first.Stdout)
	}
	firstJob := dataMap(t, first)

	second := runHelper(t, "download", "start", "--repo", "org/tiny", "--file", "tiny.gguf", "--dest", dest)
	if second.ExitCode == 0 || errCode(t, second) != "DOWNLOAD_ALREADY_RUNNING" {
		t.Fatalf("segundo start deve recusar: %s", second.Stdout)
	}
	current := dataMap(t, second)
	if current["jobId"] != firstJob["jobId"] {
		t.Fatalf("recusa deve devolver o trabalho corrente: %s vs %s", second.Stdout, first.Stdout)
	}

	cancel := runHelper(t, "download", "cancel", "--job", firstJob["jobId"].(string))
	if cancel.ExitCode != 0 {
		t.Fatalf("cancel: %s", cancel.Stdout)
	}
}

func TestDownloadCancelRemovesTempAndMarksCancelled(t *testing.T) {
	root := setupRoot(t)
	newFakeHf(t, `[{"id":"org/tiny"}]`, `[{"path":"tiny.gguf","type":"file","size":50000000}]`, downloadHandler(50000000, 65536, 20*time.Millisecond))
	dest := filepath.Join(root, "llama-cpp")

	start := runHelper(t, "download", "start", "--repo", "org/tiny", "--file", "tiny.gguf", "--dest", dest)
	jobId := dataMap(t, start)["jobId"].(string)
	sc := filepath.Join(dest, "tiny.gguf.download.json")
	pollSidecar(t, sc, 15*time.Second, func(m map[string]any) bool { return m["receivedBytes"].(float64) > 0 })

	cancel := runHelper(t, "download", "cancel", "--job", jobId)
	if cancel.ExitCode != 0 || cancel.Env["ok"] != true {
		t.Fatalf("cancel falhou: %s", cancel.Stdout)
	}

	final := pollSidecar(t, sc, 15*time.Second, func(m map[string]any) bool { return m["state"] == "cancelled" })
	if _, has := final["cancelling"]; has {
		t.Fatalf("estado observavel nunca pode ser cancelling")
	}
	if _, err := os.Stat(filepath.Join(dest, ".tiny.gguf.part")); !os.IsNotExist(err) {
		t.Fatalf("temporario deve ser removido no cancelamento")
	}
	if _, err := os.Stat(filepath.Join(dest, "tiny.gguf")); !os.IsNotExist(err) {
		t.Fatalf("arquivo final jamais pode existir apos cancelamento")
	}
}

func TestDownloadKilledPidBecomesFailed(t *testing.T) {
	root := setupRoot(t)
	newFakeHf(t, `[{"id":"org/tiny"}]`, `[{"path":"tiny.gguf","type":"file","size":50000000}]`, downloadHandler(50000000, 65536, 20*time.Millisecond))
	dest := filepath.Join(root, "llama-cpp")

	start := runHelper(t, "download", "start", "--repo", "org/tiny", "--file", "tiny.gguf", "--dest", dest)
	jobId := dataMap(t, start)["jobId"].(string)
	sc := filepath.Join(dest, "tiny.gguf.download.json")

	var pid int
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(sc)
		if err == nil {
			var m struct {
				Pid int `json:"pid"`
			}
			if json.Unmarshal(data, &m) == nil && m.Pid > 0 {
				pid = m.Pid
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	if pid <= 0 {
		t.Fatalf("pid nunca apareceu no arquivo lateral")
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		t.Fatalf("kill: %v", err)
	}

	status := runHelper(t, "download", "status", "--job", jobId)
	if status.ExitCode != 0 {
		t.Fatalf("status: %s", status.Stdout)
	}
	job := dataList(t, status)[0].(map[string]any)
	if job["state"] != "failed" {
		t.Fatalf("pid morto com running deve virar failed: %s", status.Stdout)
	}
	if msg, _ := job["error"].(string); !strings.Contains(msg, "pid") {
		t.Fatalf("mensagem de falha deve nomear o pid morto: %v", job["error"])
	}
}

func TestDownloadStartRefusesOutsideDest(t *testing.T) {
	setupRoot(t)
	newFakeHf(t, `[{"id":"org/tiny"}]`, `[{"path":"tiny.gguf","type":"file","size":100}]`, downloadHandler(100, 64, 0))
	r := runHelper(t, "download", "start", "--repo", "org/tiny", "--file", "tiny.gguf", "--dest", t.TempDir())
	if r.ExitCode == 0 || errCode(t, r) != "DEST_OUTSIDE_MODELS_DIR" {
		t.Fatalf("destino externo deve ser recusado: %s", r.Stdout)
	}
	r = runHelper(t, "download", "start", "--repo", "org/tiny", "--file", "../escape.gguf", "--dest", filepath.Join(setupRoot(t), "llama-cpp"))
	if r.ExitCode == 0 || errCode(t, r) != "INVALID_FILE_NAME" {
		t.Fatalf("arquivo com travessia deve ser recusado: %s", r.Stdout)
	}
	r = runHelper(t, "download", "start", "--repo", "sem-barra", "--file", "tiny.gguf", "--dest", filepath.Join(setupRoot(t), "llama-cpp"))
	if r.ExitCode == 0 || errCode(t, r) != "INVALID_REPO_ID" {
		t.Fatalf("repo invalido deve ser recusado: %s", r.Stdout)
	}
}

func TestRemoveSemantics(t *testing.T) {
	root := setupRoot(t)
	dir := filepath.Join(root, "llama-cpp")
	os.MkdirAll(dir, 0755)
	model := filepath.Join(dir, "velho.gguf")
	os.WriteFile(model, make([]byte, 128), 0644)

	r := runHelper(t, "remove", "--path", filepath.Join(t.TempDir(), "fora.gguf"))
	if r.ExitCode == 0 || errCode(t, r) != "DEST_OUTSIDE_MODELS_DIR" {
		t.Fatalf("remocao fora do diretorio de modelos deve ser recusada: %s", r.Stdout)
	}

	r = runHelper(t, "remove", "--path", model)
	if r.ExitCode != 0 || dataMap(t, r)["removed"] != true {
		t.Fatalf("remocao valida: %s", r.Stdout)
	}
	if _, err := os.Stat(model); !os.IsNotExist(err) {
		t.Fatalf("arquivo deveria ter sido removido")
	}

	r = runHelper(t, "remove", "--path", model)
	if r.ExitCode != 0 || dataMap(t, r)["removed"] != false {
		t.Fatalf("remocao de arquivo inexistente devolve removed false: %s", r.Stdout)
	}
}

func TestRemoveRefusedWhileRunning(t *testing.T) {
	root := setupRoot(t)
	newFakeHf(t, `[{"id":"org/tiny"}]`, `[{"path":"tiny.gguf","type":"file","size":50000000}]`, downloadHandler(50000000, 65536, 20*time.Millisecond))
	dest := filepath.Join(root, "llama-cpp")
	start := runHelper(t, "download", "start", "--repo", "org/tiny", "--file", "tiny.gguf", "--dest", dest)
	jobId := dataMap(t, start)["jobId"].(string)

	r := runHelper(t, "remove", "--path", filepath.Join(dest, "tiny.gguf"))
	if r.ExitCode == 0 || errCode(t, r) != "DOWNLOAD_RUNNING" {
		t.Fatalf("remocao com trabalho em execucao deve ser recusada: %s", r.Stdout)
	}
	runHelper(t, "download", "cancel", "--job", jobId)
}

func TestStatusCompositeHealthy(t *testing.T) {
	root := setupRoot(t)
	writeFakeStudio(t, 0, "")
	newFakeHf(t, `[{"id":"org/tiny"}]`, `[{"path":"tiny-Q4_K_M.gguf","type":"file","size":1000}]`, nil)
	dir := filepath.Join(root, "llama-cpp")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "modelo.gguf"), make([]byte, 2048), 0644)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"id":"modelo.gguf"}]}`)
	}))
	defer srv.Close()
	t.Setenv("MODELS_HELPER_SERVER_URL", srv.URL)

	r := runHelper(t, "status", "--profile", "local-models")
	if r.ExitCode != 0 || r.Env["ok"] != true || r.Env["schemaVersion"] != float64(1) {
		t.Fatalf("status: %s / %s", r.Stdout, r.Stderr)
	}
	data := dataMap(t, r)
	runtimeHealth := data["runtime"].(map[string]any)
	if runtimeHealth["ok"] != true || runtimeHealth["error"] != "" {
		t.Fatalf("runtime saudavel esperado: %s", r.Stdout)
	}
	if runtimeHealth["recommendedRuntimeId"] == "" || runtimeHealth["recommendationReason"] == "" {
		t.Fatalf("recomendacao obrigatoria: %s", r.Stdout)
	}
	server := data["server"].(map[string]any)
	if server["online"] != true || server["modelId"] != "modelo.gguf" {
		t.Fatalf("server: %s", r.Stdout)
	}
	if !strings.HasSuffix(server["baseUrl"].(string), "/v1") {
		t.Fatalf("baseUrl %v", server["baseUrl"])
	}
	if data["hasInstalled"] != true {
		t.Fatalf("hasInstalled: %s", r.Stdout)
	}
	if len(data["installed"].([]any)) != 1 {
		t.Fatalf("installed: %s", r.Stdout)
	}
	catalog := data["catalog"].([]any)
	if len(catalog) != 1 {
		t.Fatalf("catalog: %s", r.Stdout)
	}
	entry := catalog[0].(map[string]any)
	for _, k := range []string{"fitOk", "fitTight", "fitGpu", "requiredBytes"} {
		if _, ok := entry[k]; !ok {
			t.Fatalf("veredito %s ausente no catalogo: %s", k, r.Stdout)
		}
	}
}

func TestStatusRuntimeFailureKeepsExitZero(t *testing.T) {
	setupRoot(t)
	writeFakeStudio(t, 1, "✗ runtime nao instalado para esta plataforma")
	newFakeHf(t, `[]`, `[]`, nil)

	r := runHelper(t, "status", "--profile", "local-models")
	if r.ExitCode != 0 {
		t.Fatalf("falha de resolucao do runtime nao pode derrubar o status: %s", r.Stdout)
	}
	runtimeHealth := dataMap(t, r)["runtime"].(map[string]any)
	if runtimeHealth["ok"] != false {
		t.Fatalf("runtime.ok esperado false: %s", r.Stdout)
	}
	msg, _ := runtimeHealth["error"].(string)
	if msg == "" || strings.ContainsAny(msg, "\n\r") {
		t.Fatalf("erro de runtime deve ser texto simples sanitizado: %q", msg)
	}
	if !strings.Contains(msg, "runtime nao instalado") {
		t.Fatalf("mensagem preservada: %q", msg)
	}
}

func TestStatusServesInventoryWhenCatalogFails(t *testing.T) {
	root := setupRoot(t)
	writeFakeStudio(t, 0, "")
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := dead.URL
	dead.Close()
	t.Setenv("MODELS_HELPER_HF_API", url)
	dir := filepath.Join(root, "llama-cpp")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "off.gguf"), make([]byte, 512), 0644)

	r := runHelper(t, "status", "--profile", "local-models")
	if r.ExitCode != 0 || r.Env["ok"] != true {
		t.Fatalf("inventario local nunca depende da rede: %s / %s", r.Stdout, r.Stderr)
	}
	data := dataMap(t, r)
	if data["hasInstalled"] != true || len(data["installed"].([]any)) != 1 {
		t.Fatalf("inventario ausente: %s", r.Stdout)
	}
	if len(data["catalog"].([]any)) != 0 {
		t.Fatalf("catalogo deve ser vazio: %s", r.Stdout)
	}
	if data["catalogError"] == "" {
		t.Fatalf("falha de catalogo deve ser nomeada em campo proprio: %s", r.Stdout)
	}
}

func TestStatusServerProbeLimitedToTwoSeconds(t *testing.T) {
	setupRoot(t)
	writeFakeStudio(t, 0, "")
	newFakeHf(t, `[]`, `[]`, nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(6 * time.Second)
	}))
	defer srv.Close()
	t.Setenv("MODELS_HELPER_SERVER_URL", srv.URL)

	begin := time.Now()
	r := runHelper(t, "status", "--profile", "local-models")
	elapsed := time.Since(begin)
	if r.ExitCode != 0 {
		t.Fatalf("status: %s", r.Stdout)
	}
	if dataMap(t, r)["server"].(map[string]any)["online"] != false {
		t.Fatalf("sonda sem resposta deve ficar offline: %s", r.Stdout)
	}
	if elapsed > 4*time.Second {
		t.Fatalf("sonda limitada a 2s deveria manter o status rapido: %s", elapsed)
	}
}

func TestDownloadUnknownSizeIsNamedError(t *testing.T) {
	setupRoot(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/org/tiny/resolve/main/tiny.gguf", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	t.Setenv("MODELS_HELPER_HF_API", ts.URL)
	root, _ := os.LookupEnv("MODELS_HELPER_MODELS_ROOT")

	r := runHelper(t, "download", "start", "--repo", "org/tiny", "--file", "tiny.gguf", "--dest", filepath.Join(root, "llama-cpp"))
	if r.ExitCode == 0 || errCode(t, r) != "DOWNLOAD_SIZE_UNKNOWN" {
		t.Fatalf("tamanho desconhecido deve ser erro nomeado: %s", r.Stdout)
	}
}
