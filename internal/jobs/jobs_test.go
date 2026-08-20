package jobs

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/MacroStudio-Solutions/models-helper/internal/contract"
)

func TestSidecarRoundtrip(t *testing.T) {
	dir := t.TempDir()
	j := &TJobFile{}
	j.TDownloadJob = contract.TDownloadJob{
		JobId: "abc", RepoId: "org/r", File: "m.gguf", Destination: dir,
		State: contract.JobStateRunning, TotalBytes: 42, Pid: 7,
		StartedAt: "2026-01-01T00:00:00Z",
	}
	j.Url = "http://x/y"
	j.TempName = ".m.gguf.part"
	sc := SidecarPath(dir, "m.gguf")
	if err := WriteSidecar(sc, j); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := os.Stat(sc + ".tmp"); err == nil {
		t.Fatalf("temporario de escrita nao pode sobrar")
	}
	got, err := ReadSidecar(sc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.JobId != "abc" || got.Url != "http://x/y" || got.TempName != ".m.gguf.part" || got.Pid != 7 {
		t.Fatalf("roundtrip %+v", got)
	}
	if got.UpdatedAt == "" {
		t.Fatalf("updatedAt deve ser preenchido na escrita")
	}
}

func TestPathNaming(t *testing.T) {
	if SidecarPath("/d", "m.gguf") != "/d/m.gguf.download.json" {
		t.Fatalf("sidecar %s", SidecarPath("/d", "m.gguf"))
	}
	if TempPath("/d", "m.gguf") != "/d/.m.gguf.part" {
		t.Fatalf("temp %s", TempPath("/d", "m.gguf"))
	}
	if TempPath("/d", "sub/m.gguf") != "/d/sub/.m.gguf.part" {
		t.Fatalf("temp subdir %s", TempPath("/d", "sub/m.gguf"))
	}
	if MarkerPath("/d", "m.gguf") != "/d/m.gguf.download.json.cancel" {
		t.Fatalf("marker %s", MarkerPath("/d", "m.gguf"))
	}
}

func deadPid(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Skipf("sem processo: %v", err)
	}
	_ = cmd.Wait()
	return cmd.Process.Pid
}

func alivePid(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("sleep", "5")
	if err := cmd.Start(); err != nil {
		t.Skipf("sem processo: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	return cmd.Process.Pid
}

func TestReapDeadPidBecomesFailed(t *testing.T) {
	dir := t.TempDir()
	j := &TJobFile{}
	j.TDownloadJob = contract.TDownloadJob{
		JobId: "j1", File: "m.gguf", Destination: dir,
		State: contract.JobStateRunning, Pid: deadPid(t), StartedAt: time.Now().UTC().Format(time.RFC3339),
	}
	sc := SidecarPath(dir, "m.gguf")
	if err := WriteSidecar(sc, j); err != nil {
		t.Fatalf("err: %v", err)
	}
	Reap(TCollected{Job: j, Path: sc})
	if j.State != contract.JobStateFailed || j.Error == "" {
		t.Fatalf("pid morto com running devia virar failed com mensagem: %+v", j.TDownloadJob)
	}
}

func TestReapAlivePidStaysRunning(t *testing.T) {
	dir := t.TempDir()
	j := &TJobFile{}
	j.TDownloadJob = contract.TDownloadJob{
		JobId: "j2", File: "m.gguf", Destination: dir,
		State: contract.JobStateRunning, Pid: alivePid(t), StartedAt: time.Now().UTC().Format(time.RFC3339),
	}
	Reap(TCollected{Job: j, Path: SidecarPath(dir, "m.gguf")})
	if j.State != contract.JobStateRunning {
		t.Fatalf("pid vivo deve seguir running: %+v", j.TDownloadJob)
	}
}

func TestReapIgnoresTerminalStates(t *testing.T) {
	dir := t.TempDir()
	j := &TJobFile{}
	j.TDownloadJob = contract.TDownloadJob{
		JobId: "j3", File: "m.gguf", Destination: dir,
		State: contract.JobStateCompleted, Pid: deadPid(t),
	}
	Reap(TCollected{Job: j, Path: SidecarPath(dir, "m.gguf")})
	if j.State != contract.JobStateCompleted {
		t.Fatalf("estado terminal nao deve ser reapado: %+v", j.TDownloadJob)
	}
}

func TestCollectSkipsCacheDirAndNoise(t *testing.T) {
	root := t.TempDir()
	modelsDir := filepath.Join(root, "llama-cpp")
	os.MkdirAll(modelsDir, 0755)
	os.MkdirAll(filepath.Join(root, ".models-helper", "cache"), 0755)

	j := &TJobFile{}
	j.TDownloadJob = contract.TDownloadJob{
		JobId: "keep", File: "m.gguf", Destination: modelsDir,
		State: contract.JobStateRunning, Pid: alivePid(t), StartedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := WriteSidecar(SidecarPath(modelsDir, "m.gguf"), j); err != nil {
		t.Fatalf("err: %v", err)
	}
	os.WriteFile(filepath.Join(root, ".models-helper", "cache", "x.download.json"), []byte(`{"jobId":"nope"}`), 0644)
	os.WriteFile(filepath.Join(modelsDir, "lixo.download.json"), []byte(`nao-json`), 0644)

	collected := Collect(root)
	if len(collected) != 1 || collected[0].Job.JobId != "keep" {
		t.Fatalf("collect %+v", collected)
	}
}

func TestRefreshReceivedFromTempFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(TempPath(dir, "m.gguf"), make([]byte, 777), 0644)
	j := &TJobFile{}
	j.TDownloadJob = contract.TDownloadJob{
		JobId: "j4", File: "m.gguf", Destination: dir,
		State: contract.JobStateRunning, TotalBytes: 2000,
	}
	RefreshReceived(TCollected{Job: j, Path: SidecarPath(dir, "m.gguf")})
	if j.ReceivedBytes != 777 || j.Percent != 38 {
		t.Fatalf("refresh %+v", j.TDownloadJob)
	}
}

func TestLatestByFilePrefersNewest(t *testing.T) {
	dir := t.TempDir()
	older := &TJobFile{}
	older.TDownloadJob = contract.TDownloadJob{JobId: "old", File: "m.gguf", Destination: dir, StartedAt: "2026-01-01T00:00:00Z"}
	newer := &TJobFile{}
	newer.TDownloadJob = contract.TDownloadJob{JobId: "new", File: "m.gguf", Destination: dir, StartedAt: "2026-01-02T00:00:00Z"}
	other := &TJobFile{}
	other.TDownloadJob = contract.TDownloadJob{JobId: "other", File: "z.gguf", Destination: dir, StartedAt: "2026-01-03T00:00:00Z"}
	byFile := LatestByFile([]TCollected{{Job: older, Path: "a"}, {Job: newer, Path: "b"}, {Job: other, Path: "c"}}, dir)
	if byFile["m.gguf"].JobId != "new" || byFile["z.gguf"].JobId != "other" {
		t.Fatalf("byFile %+v", byFile)
	}
}

func TestJobFileMarshalsContractFields(t *testing.T) {
	j := &TJobFile{}
	j.TDownloadJob = contract.TDownloadJob{JobId: "x", File: "m.gguf", Destination: "/d", State: contract.JobStateRunning}
	j.Url = "u"
	j.TempName = "t"
	data, err := json.Marshal(j)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("err: %v", err)
	}
	for _, k := range []string{"jobId", "file", "destination", "state", "url", "tempName", "receivedBytes", "updatedAt", "error"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("campo %s ausente em %s", k, data)
		}
	}
}
