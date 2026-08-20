package jobs

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/MacroStudio-Solutions/models-helper/internal/contract"
)

const (
	bufSize          = 256 << 10
	updateBytesEvery = 512 << 10
	updateEvery      = 400 * time.Millisecond
)

func RunTransfer(jobFilePath string) error {
	j, err := ReadSidecar(jobFilePath)
	if err != nil {
		return err
	}
	finalPath := filepath.Join(j.Destination, j.File)
	tempPath := filepath.Join(j.Destination, j.TempName)
	if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
		return fail(jobFilePath, j, tempPath, fmt.Sprintf("falha ao preparar o diretorio de destino: %v", err))
	}

	update := func(mutate func(*TJobFile)) {
		mutate(j)
		j.Pid = os.Getpid()
		_ = WriteSidecar(jobFilePath, j)
	}

	resp, err := http.Get(j.Url)
	if err != nil {
		return fail(jobFilePath, j, tempPath, fmt.Sprintf("falha ao iniciar a transferencia: %v", err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fail(jobFilePath, j, tempPath, fmt.Sprintf("o servidor respondeu HTTP %d", resp.StatusCode))
	}
	if resp.ContentLength > 0 {
		j.TotalBytes = uint64(resp.ContentLength)
	}

	out, err := os.OpenFile(tempPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return fail(jobFilePath, j, tempPath, fmt.Sprintf("falha ao criar o arquivo temporario: %v", err))
	}

	buf := make([]byte, bufSize)
	var received uint64
	lastWrite := time.Now()
	var lastWritten uint64
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				return fail(jobFilePath, j, tempPath, fmt.Sprintf("falha ao gravar o arquivo temporario: %v", werr))
			}
			received += uint64(n)
		}
		progress := time.Since(lastWrite) >= updateEvery || received-lastWritten >= updateBytesEvery
		if progress {
			if CancelMarkerExists(j.Destination, j.File) {
				out.Close()
				_ = os.Remove(tempPath)
				update(func(x *TJobFile) {
					x.State = contract.JobStateCancelled
					x.ReceivedBytes = received
					x.RecomputePercent()
				})
				_ = os.Remove(MarkerPath(j.Destination, j.File))
				return nil
			}
			lastWrite = time.Now()
			lastWritten = received
			update(func(x *TJobFile) {
				x.ReceivedBytes = received
				x.RecomputePercent()
			})
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			out.Close()
			return fail(jobFilePath, j, tempPath, fmt.Sprintf("leitura interrompida: %v", rerr))
		}
	}

	if err := out.Sync(); err != nil {
		out.Close()
		return fail(jobFilePath, j, tempPath, fmt.Sprintf("falha ao sincronizar o arquivo temporario: %v", err))
	}
	if err := out.Close(); err != nil {
		return fail(jobFilePath, j, tempPath, fmt.Sprintf("falha ao fechar o arquivo temporario: %v", err))
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return fail(jobFilePath, j, tempPath, fmt.Sprintf("falha ao promover o arquivo final: %v", err))
	}
	if j.TotalBytes == 0 {
		if st, serr := os.Stat(finalPath); serr == nil {
			j.TotalBytes = uint64(st.Size())
		}
	}
	update(func(x *TJobFile) {
		x.State = contract.JobStateCompleted
		x.ReceivedBytes = received
		if x.TotalBytes > 0 && received >= x.TotalBytes {
			x.ReceivedBytes = x.TotalBytes
		}
		x.Percent = 100
		x.Error = ""
	})
	_ = os.Remove(MarkerPath(j.Destination, j.File))
	return nil
}

func fail(jobFilePath string, j *TJobFile, tempPath string, message string) error {
	_ = os.Remove(tempPath)
	j.Pid = os.Getpid()
	j.State = contract.JobStateFailed
	j.Error = message
	_ = WriteSidecar(jobFilePath, j)
	return fmt.Errorf("%s", message)
}
