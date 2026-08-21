package jobs

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/MacroStudio-Solutions/models-helper/internal/catalog"
	"github.com/MacroStudio-Solutions/models-helper/internal/contract"
	"github.com/MacroStudio-Solutions/models-helper/internal/paths"
)

func Start(repoId string, file string, destDir string) (*contract.TDownloadJob, *contract.THelperError) {
	root := paths.ModelsRoot()
	destReal, herr := paths.ResolveWithin(root, destDir)
	if herr != nil {
		return nil, herr
	}
	if herr := paths.SafeRelFile(file); herr != nil {
		return nil, herr
	}
	if herr := paths.SafeRepoId(repoId); herr != nil {
		return nil, herr
	}
	if err := os.MkdirAll(destReal, 0755); err != nil {
		return nil, contract.Errorf("START_FAILED", "falha ao criar o diretorio de destino: %v", err)
	}

	sc := SidecarPath(destReal, file)
	if existing, err := ReadSidecar(sc); err == nil {
		collected := TCollected{Job: existing, Path: sc}
		Reap(collected)
		if existing.State == contract.JobStateRunning {
			current := Snapshot(existing)
			return &current, contract.Errorf("DOWNLOAD_ALREADY_RUNNING", "ja existe trabalho em execucao para %s", file)
		}
	}

	client := catalog.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	fileUrl := client.ResolveURL(repoId, file)
	total, err := client.Size(ctx, fileUrl)
	if err != nil {
		return nil, contract.Errorf("DOWNLOAD_SIZE_UNKNOWN", "nao foi possivel determinar o tamanho de %s/%s: %v", repoId, file, err)
	}

	self, err := os.Executable()
	if err != nil {
		return nil, contract.Errorf("START_FAILED", "falha ao localizar o proprio binario: %v", err)
	}

	j := &TJobFile{}
	j.TDownloadJob = contract.TDownloadJob{
		JobId:       NewJobId(),
		RepoId:      repoId,
		File:        file,
		Destination: destReal,
		State:       contract.JobStateRunning,
		TotalBytes:  total,
		StartedAt:   nowIso(),
	}
	j.Url = fileUrl
	j.TempName = tempNameFor(file)
	if err := WriteSidecar(sc, j); err != nil {
		return nil, contract.Errorf("START_FAILED", "falha ao gravar o estado do trabalho: %v", err)
	}

	cmd := exec.Command(self, "__transfer", "--job-file", sc)
	cmd.Dir = destReal
	cmd.SysProcAttr = detachedAttr()
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if devNull != nil {
		cmd.Stdout = devNull
		cmd.Stderr = devNull
	}
	if err := cmd.Start(); err != nil {
		_ = os.Remove(sc)
		return nil, contract.Errorf("START_FAILED", "falha ao destacar o processo de transferencia: %v", err)
	}
	j.Pid = cmd.Process.Pid
	if err := WriteSidecar(sc, j); err != nil {
		return nil, contract.Errorf("START_FAILED", "falha ao registrar o pid do trabalho: %v", err)
	}
	_ = cmd.Process.Release()

	job := Snapshot(j)
	return &job, nil
}

func tempNameFor(file string) string {
	return filepath.Join(filepath.Dir(file), "."+filepath.Base(file)+".part")
}

func Status(jobId string, destDir string) ([]contract.TDownloadJob, *contract.THelperError) {
	root := paths.ModelsRoot()
	scanDir := root
	if destDir != "" {
		real, herr := paths.ResolveWithin(root, destDir)
		if herr != nil {
			return nil, herr
		}
		scanDir = real
	}
	collected := Collect(scanDir)
	jobs := []contract.TDownloadJob{}
	for _, c := range collected {
		Reap(c)
		RefreshReceived(c)
		if jobId != "" && c.Job.JobId != jobId {
			continue
		}
		jobs = append(jobs, Snapshot(c.Job))
	}
	if jobId != "" && len(jobs) == 0 {
		return nil, contract.Errorf("JOB_NOT_FOUND", "nenhum trabalho com identificador %s", jobId)
	}
	return jobs, nil
}

func Cancel(jobId string) (*contract.TDownloadJob, *contract.THelperError) {
	collected := Collect(paths.ModelsRoot())
	for _, c := range collected {
		if c.Job.JobId != jobId {
			continue
		}
		Reap(c)
		if c.Job.State == contract.JobStateRunning {
			if err := os.MkdirAll(filepath.Dir(c.Path), 0755); err == nil {
				_ = os.WriteFile(MarkerPath(c.Job.Destination, c.Job.File), []byte(nowIso()), 0644)
			}
		}
		job := Snapshot(c.Job)
		return &job, nil
	}
	return nil, contract.Errorf("JOB_NOT_FOUND", "nenhum trabalho com identificador %s", jobId)
}

func Remove(path string) (bool, *contract.THelperError) {
	root := paths.ModelsRoot()
	if !paths.IsStrictlyWithin(root, path) {
		return false, contract.Errorf("DEST_OUTSIDE_MODELS_DIR", "caminho %s esta fora do diretorio de modelos %s", path, root)
	}
	st, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return false, contract.Errorf("REMOVE_FAILED", "falha ao inspecionar %s: %v", path, err)
	}
	if st != nil && st.IsDir() {
		return false, contract.Errorf("REMOVE_REFUSED", "caminho %s e um diretorio", path)
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	name := base
	if suffix := ".download.json"; len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
		name = name[:len(name)-len(suffix)]
	}
	sc := SidecarPath(dir, name)
	if j, err := ReadSidecar(sc); err == nil && j.State == contract.JobStateRunning {
		return false, contract.Errorf("DOWNLOAD_RUNNING", "existe trabalho em execucao para %s; cancele-o antes de remover", name)
	}
	if st == nil {
		return false, nil
	}
	if err := os.Remove(path); err != nil {
		return false, contract.Errorf("REMOVE_FAILED", "falha ao remover %s: %v", path, err)
	}
	_ = os.Remove(sc)
	_ = os.Remove(MarkerPath(dir, name))
	_ = os.Remove(TempPath(dir, name))
	return true, nil
}
