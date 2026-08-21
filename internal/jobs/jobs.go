package jobs

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/MacroStudio-Solutions/models-helper/internal/contract"
	"github.com/MacroStudio-Solutions/models-helper/internal/format"
)

type TJobFile struct {
	contract.TDownloadJob
	Url      string `json:"url"`
	TempName string `json:"tempName"`
}

func SidecarPath(destDir string, file string) string {
	return filepath.Join(destDir, file+".download.json")
}

func TempPath(destDir string, file string) string {
	return filepath.Join(destDir, filepath.Join(filepath.Dir(file), "."+filepath.Base(file)+".part"))
}

func MarkerPath(destDir string, file string) string {
	return SidecarPath(destDir, file) + ".cancel"
}

func NewJobId() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return time.Now().UTC().Format("20060102150405") + "-fallback"
	}
	return hex.EncodeToString(buf)
}

func nowIso() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func WriteSidecar(path string, j *TJobFile) error {
	j.UpdatedAt = nowIso()
	data, err := json.Marshal(j)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func ReadSidecar(path string) (*TJobFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var j TJobFile
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, err
	}
	return &j, nil
}

type TCollected struct {
	Job  *TJobFile
	Path string
}

func Collect(dir string) []TCollected {
	collected := []TCollected{}
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".models-helper" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".download.json") {
			return nil
		}
		if j, rerr := ReadSidecar(p); rerr == nil && j.JobId != "" {
			collected = append(collected, TCollected{Job: j, Path: p})
		}
		return nil
	})
	sort.Slice(collected, func(i, j int) bool {
		if collected[i].Job.StartedAt != collected[j].Job.StartedAt {
			return collected[i].Job.StartedAt < collected[j].Job.StartedAt
		}
		return collected[i].Path < collected[j].Path
	})
	return collected
}

func Reap(c TCollected) {
	j := c.Job
	if j.State != contract.JobStateRunning || j.Pid == 0 {
		return
	}
	if pidAlive(j.Pid) {
		return
	}
	j.State = contract.JobStateFailed
	j.Error = "processo de transferencia encerrado inesperadamente (pid morto)"
	j.UpdatedAt = nowIso()
	_ = WriteSidecar(c.Path, j)
}

func RefreshReceived(c TCollected) {
	j := c.Job
	if j.State != contract.JobStateRunning {
		return
	}
	temp := TempPath(j.Destination, j.File)
	if st, err := os.Stat(temp); err == nil && st.Mode().IsRegular() {
		j.ReceivedBytes = uint64(st.Size())
		if j.TotalBytes > 0 {
			pct := int(uint64(j.ReceivedBytes) * 100 / j.TotalBytes)
			if pct > 100 {
				pct = 100
			}
			j.Percent = pct
		}
	}
}

func LatestByFile(collected []TCollected, destDir string) map[string]*TJobFile {
	byFile := map[string]*TJobFile{}
	for i := range collected {
		j := collected[i].Job
		if j.Destination != destDir {
			continue
		}
		if cur, exists := byFile[j.File]; !exists || j.StartedAt >= cur.StartedAt {
			byFile[j.File] = j
		}
	}
	return byFile
}

func (j *TJobFile) RecomputePercent() {
	if j.TotalBytes > 0 {
		pct := int(uint64(j.ReceivedBytes) * 100 / j.TotalBytes)
		if pct > 100 {
			pct = 100
		}
		j.Percent = pct
	}
}

func CancelMarkerExists(destDir string, file string) bool {
	if _, err := os.Stat(MarkerPath(destDir, file)); err == nil {
		return true
	}
	return false
}

// Snapshot e a unica forma de tirar um TDownloadJob de um arquivo lateral.
// Existe para que nenhum ponto de leitura possa esquecer os rotulos: quem
// copiasse a struct direto devolveria bytes crus para a tela imprimir.
func Snapshot(j *TJobFile) contract.TDownloadJob {
	job := j.TDownloadJob
	job.ReceivedLabel = format.Bytes(job.ReceivedBytes)
	job.TotalLabel = format.Bytes(job.TotalBytes)
	job.ProgressLabel = format.Transfer(job.ReceivedBytes, job.TotalBytes)
	return job
}
