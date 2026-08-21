package catalog

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/MacroStudio-Solutions/models-helper/internal/contract"
	"github.com/MacroStudio-Solutions/models-helper/internal/env"
	"github.com/MacroStudio-Solutions/models-helper/internal/fit"
	"github.com/MacroStudio-Solutions/models-helper/internal/format"
	"github.com/MacroStudio-Solutions/models-helper/internal/paths"
)

type TModel struct {
	Id string `json:"id"`
}

type TRawTreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size uint64 `json:"size"`
}

type TClient struct {
	BaseURL  string
	HTTP     *http.Client
	CacheDir string
	CacheTTL time.Duration
}

func NewClient() *TClient {
	return &TClient{
		BaseURL:  strings.TrimRight(env.HfApiBase(), "/"),
		HTTP:     &http.Client{Timeout: 20 * time.Second},
		CacheDir: paths.CacheDir(),
		CacheTTL: 5 * time.Minute,
	}
}

type TCacheFile struct {
	FetchedAt time.Time       `json:"fetchedAt"`
	Body      json.RawMessage `json:"body"`
}

func (c *TClient) cachePath(key string) string {
	sum := sha1.Sum([]byte(key))
	return filepath.Join(c.CacheDir, "catalog-"+hex.EncodeToString(sum[:])+".json")
}

func (c *TClient) readCache(path string) (*TCacheFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cf TCacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, err
	}
	return &cf, nil
}

func (c *TClient) writeCache(path string, body []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}
	cf := TCacheFile{FetchedAt: time.Now().UTC(), Body: json.RawMessage(body)}
	data, err := json.Marshal(cf)
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

func (c *TClient) fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "models-helper/1")
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d em %s", resp.StatusCode, url)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 32<<20))
}

func (c *TClient) fetchCached(ctx context.Context, key string, url string) (json.RawMessage, error) {
	cp := c.cachePath(key)
	if cf, err := c.readCache(cp); err == nil && time.Since(cf.FetchedAt) < c.CacheTTL {
		return cf.Body, nil
	}
	body, err := c.fetch(ctx, url)
	if err != nil {
		if cf, cerr := c.readCache(cp); cerr == nil {
			fmt.Fprintf(os.Stderr, "models-helper: catalogo remoto falhou (%v); servindo copia local de %s\n", err, cf.FetchedAt.Format(time.RFC3339))
			return cf.Body, nil
		}
		return nil, err
	}
	c.writeCache(cp, body)
	return json.RawMessage(body), nil
}

func (c *TClient) List(ctx context.Context, limit int) ([]TModel, error) {
	return c.queryModels(ctx, fmt.Sprintf("list:%d", limit), url.Values{
		"filter":    {"gguf"},
		"sort":      {"downloads"},
		"direction": {"-1"},
		"limit":     {fmt.Sprintf("%d", limit)},
	})
}

func (c *TClient) Search(ctx context.Context, term string, limit int) ([]TModel, error) {
	return c.queryModels(ctx, fmt.Sprintf("search:%s:%d", term, limit), url.Values{
		"filter":    {"gguf"},
		"search":    {term},
		"sort":      {"downloads"},
		"direction": {"-1"},
		"limit":     {fmt.Sprintf("%d", limit)},
	})
}

func (c *TClient) queryModels(ctx context.Context, key string, params url.Values) ([]TModel, error) {
	body, err := c.fetchCached(ctx, key, c.BaseURL+"/api/models?"+params.Encode())
	if err != nil {
		return nil, contract.Errorf("CATALOG_UNAVAILABLE", "falha ao consultar a API publica de modelos: %v", err)
	}
	var models []TModel
	if err := json.Unmarshal(body, &models); err != nil {
		return nil, contract.Errorf("CATALOG_UNAVAILABLE", "resposta invalida da API de modelos: %v", err)
	}
	return models, nil
}

func (c *TClient) Tree(ctx context.Context, repoId string) ([]TTreeFile, error) {
	return c.TreeOf(ctx, repoId, ".gguf")
}

// TreeOf lista os arquivos de peso de um repositorio. A extensao e parametro
// porque o llama.cpp le .gguf e o whisper.cpp le .bin, e um catalogo de
// transcricao que filtrasse por .gguf voltaria vazio.
func (c *TClient) TreeOf(ctx context.Context, repoId string, suffix string) ([]TTreeFile, error) {
	body, err := c.fetchCached(ctx, "tree:"+repoId, c.BaseURL+"/api/models/"+repoId+"/tree/main")
	if err != nil {
		return nil, contract.Errorf("CATALOG_UNAVAILABLE", "falha ao consultar versoes de %s: %v", repoId, err)
	}
	var raw []TRawTreeEntry
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, contract.Errorf("CATALOG_UNAVAILABLE", "resposta invalida da arvore de %s: %v", repoId, err)
	}
	var files []TTreeFile
	for _, e := range raw {
		if e.Type == "file" && strings.HasSuffix(e.Path, suffix) {
			files = append(files, TTreeFile{Path: e.Path, Size: e.Size})
		}
	}
	return files, nil
}

func escapeFileSegments(file string) string {
	parts := strings.Split(file, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

func (c *TClient) ResolveURL(repoId string, file string) string {
	return c.BaseURL + "/" + repoId + "/resolve/main/" + escapeFileSegments(file)
}

func (c *TClient) Size(ctx context.Context, fileUrl string) (uint64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, fileUrl, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "models-helper/1")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength <= 0 {
		return 0, fmt.Errorf("tamanho desconhecido")
	}
	return uint64(resp.ContentLength), nil
}

func BuildEntry(model TModel, treeFile TTreeFile, machineProfile contract.TMachineProfile, modelsDir string) contract.TCatalogEntry {
	token := QuantToken(treeFile.Path)
	return contract.TCatalogEntry{
		TModelFit:    fit.Compute(machineProfile.RamTotalBytes, machineProfile.RamAvailableBytes, machineProfile.VramBytes, treeFile.Size),
		Name:         DisplayName(model.Id, token),
		RepoId:       model.Id,
		File:         treeFile.Path,
		Quantization: token,
		SizeBytes:    treeFile.Size,
		SizeGb:       fit.SizeGb(treeFile.Size),
		SizeLabel:    format.Bytes(treeFile.Size),
		Installed:    fileExists(filepath.Join(modelsDir, treeFile.Path)),
		Download:     nil,
	}
}

func DefaultEntries(ctx context.Context, c *TClient, models []TModel, machineProfile contract.TMachineProfile, modelsDir string) []contract.TCatalogEntry {
	entries := make([]*contract.TCatalogEntry, len(models))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i, model := range models {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, model TModel) {
			defer wg.Done()
			defer func() { <-sem }()
			files, err := c.Tree(ctx, model.Id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "models-helper: %v\n", err)
				return
			}
			if file, ok := PickDefaultFile(files); ok {
				entry := BuildEntry(model, file, machineProfile, modelsDir)
				entries[i] = &entry
			}
		}(i, model)
	}
	wg.Wait()
	result := make([]contract.TCatalogEntry, 0, len(entries))
	for _, e := range entries {
		if e != nil {
			result = append(result, *e)
		}
	}
	return result
}

func VersionEntries(ctx context.Context, c *TClient, repoId string, machineProfile contract.TMachineProfile, modelsDir string) ([]contract.TCatalogEntry, *contract.THelperError) {
	if err := paths.SafeRepoId(repoId); err != nil {
		return nil, err
	}
	files, err := c.Tree(ctx, repoId)
	if err != nil {
		return nil, err.(*contract.THelperError)
	}
	SortBySize(files)
	model := TModel{Id: repoId}
	entries := make([]contract.TCatalogEntry, 0, len(files))
	for _, f := range files {
		entries = append(entries, BuildEntry(model, f, machineProfile, modelsDir))
	}
	return entries, nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// ---- ordenacao e filtro por viabilidade ---------------------------------

const (
	SortFit        = "fit"
	SortPopularity = "popularity"
	SortSize       = "size"
	FitAny         = "any"
	FitFits        = "fits"
	FitGpu         = "gpu"
)

func IsSortMode(mode string) bool {
	return mode == SortFit || mode == SortPopularity || mode == SortSize
}

func IsFitMode(mode string) bool {
	return mode == FitAny || mode == FitFits || mode == FitGpu
}

// SortEntries reordena no lugar. A ordenacao por viabilidade e estavel de
// proposito: dentro de um mesmo veredito a ordem de chegada — a popularidade
// da API — e a melhor desempate que existe.
func SortEntries(entries []contract.TCatalogEntry, mode string) {
	switch mode {
	case SortFit:
		sort.SliceStable(entries, func(i, j int) bool {
			return entries[i].FitRank < entries[j].FitRank
		})
	case SortSize:
		sort.SliceStable(entries, func(i, j int) bool {
			return entries[i].SizeBytes < entries[j].SizeBytes
		})
	}
}

// FilterByFit descarta o que a maquina nao aguenta. Fora do modo `any` um
// modelo ja instalado nunca e escondido: ele esta no disco, e sumir com ele da
// lista deixaria o operador sem o botao para removê-lo.
func FilterByFit(entries []contract.TCatalogEntry, mode string) []contract.TCatalogEntry {
	if mode == "" || mode == FitAny {
		return entries
	}
	kept := make([]contract.TCatalogEntry, 0, len(entries))
	for _, e := range entries {
		switch mode {
		case FitGpu:
			if e.FitGpu || e.Installed {
				kept = append(kept, e)
			}
		default:
			if e.FitRank != contract.FitRankNo || e.Installed {
				kept = append(kept, e)
			}
		}
	}
	return kept
}

func Limit(entries []contract.TCatalogEntry, max int) []contract.TCatalogEntry {
	if max > 0 && len(entries) > max {
		return entries[:max]
	}
	return entries
}
