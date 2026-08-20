package catalog

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MacroStudio-Solutions/models-helper/internal/contract"
)

func newCountingServer(t *testing.T, hits *atomic.Int64) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/models", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"id":"org/tiny"},{"id":"org/big"}]`)
	})
	mux.HandleFunc("/api/models/org/tiny/tree/main", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"path":"tiny-Q4_K_M.gguf","type":"file","size":1000},{"path":"tiny-Q8_0.gguf","type":"file","size":2000}]`)
	})
	return httptest.NewServer(mux)
}

func newTestClient(t *testing.T, server *httptest.Server, ttl time.Duration) *TClient {
	t.Helper()
	t.Setenv("MODELS_HELPER_HF_API", server.URL)
	return &TClient{
		BaseURL:  server.URL,
		HTTP:     server.Client(),
		CacheDir: t.TempDir(),
		CacheTTL: ttl,
	}
}

func TestCacheHitsWithinTTL(t *testing.T) {
	var hits atomic.Int64
	server := newCountingServer(t, &hits)
	defer server.Close()
	client := newTestClient(t, server, 5*time.Minute)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		models, err := client.List(ctx, 6)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(models) != 2 {
			t.Fatalf("models %d", len(models))
		}
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("esperado 1 hit no servidor, obtido %d", got)
	}

	files, err := client.Tree(ctx, "org/tiny")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("files %d", len(files))
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("esperado 2 hits (list + tree), obtido %d", got)
	}
	_, _ = client.Tree(ctx, "org/tiny")
	if got := hits.Load(); got != 2 {
		t.Fatalf("tree deve vir do cache, hits %d", got)
	}
}

func TestCacheExpiresByTimestamp(t *testing.T) {
	var hits atomic.Int64
	server := newCountingServer(t, &hits)
	defer server.Close()
	client := newTestClient(t, server, 50*time.Millisecond)
	ctx := context.Background()

	if _, err := client.List(ctx, 6); err != nil {
		t.Fatalf("err: %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	if _, err := client.List(ctx, 6); err != nil {
		t.Fatalf("err: %v", err)
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("esperado refetch apos expiracao, hits %d", got)
	}
}

func TestBackdatedCacheFileRefetches(t *testing.T) {
	var hits atomic.Int64
	server := newCountingServer(t, &hits)
	defer server.Close()
	client := newTestClient(t, server, 5*time.Minute)
	ctx := context.Background()

	if _, err := client.List(ctx, 6); err != nil {
		t.Fatalf("err: %v", err)
	}
	cp := client.cachePath("list:6")
	data, err := os.ReadFile(cp)
	if err != nil {
		t.Fatalf("cache ausente: %v", err)
	}
	backdated := []byte(`{"fetchedAt":"2020-01-01T00:00:00Z","body":[{"id":"org/tiny"},{"id":"org/big"}]}`)
	_ = data
	if err := os.WriteFile(cp, backdated, 0644); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := client.List(ctx, 6); err != nil {
		t.Fatalf("err: %v", err)
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("esperado refetch com carimbo velho, hits %d", got)
	}
}

func TestRemoteFailureServesLocalCopy(t *testing.T) {
	var hits atomic.Int64
	server := newCountingServer(t, &hits)
	client := newTestClient(t, server, 5*time.Minute)
	ctx := context.Background()

	if _, err := client.List(ctx, 6); err != nil {
		t.Fatalf("err: %v", err)
	}
	cp := client.cachePath("list:6")
	os.WriteFile(cp, []byte(`{"fetchedAt":"2020-01-01T00:00:00Z","body":[{"id":"org/tiny"},{"id":"org/big"}]}`), 0644)
	server.Close()

	models, err := client.List(ctx, 6)
	if err != nil {
		t.Fatalf("copia local valida deve servir apos falha remota: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("models %d", len(models))
	}
}

func TestRemoteFailureWithoutCacheErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()
	client := newTestClient(t, server, 5*time.Minute)
	if _, err := client.List(context.Background(), 6); err == nil {
		t.Fatalf("sem copia local a falha deve ser nomeada")
	}
}

func TestResolveURLAndSize(t *testing.T) {
	var headOK atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			headOK.Store(true)
			w.Header().Set("Content-Length", "12345")
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()
	client := newTestClient(t, server, time.Minute)

	url := client.ResolveURL("org/tiny", "sub/tiny-Q4_K_M.gguf")
	if url != server.URL+"/org/tiny/resolve/main/sub/tiny-Q4_K_M.gguf" {
		t.Fatalf("url %s", url)
	}
	size, err := client.Size(context.Background(), url)
	if err != nil || size != 12345 {
		t.Fatalf("size %d err %v", size, err)
	}
	if !headOK.Load() {
		t.Fatalf("HEAD nao usado")
	}
}

func TestDefaultEntriesOrderAndContent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/models", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"id":"org/tiny"},{"id":"org/none"}]`)
	})
	mux.HandleFunc("/api/models/org/tiny/tree/main", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"path":"tiny-Q8_0.gguf","type":"file","size":2000},{"path":"tiny-Q4_K_M.gguf","type":"file","size":1000}]`)
	})
	mux.HandleFunc("/api/models/org/none/tree/main", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"path":"readme.txt","type":"file","size":10}]`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	client := newTestClient(t, ts, time.Minute)
	modelsDir := filepath.Join(t.TempDir(), "llama-cpp")

	entries := DefaultEntries(context.Background(), client, []TModel{{Id: "org/tiny"}, {Id: "org/none"}}, contract.TMachineProfile{}, modelsDir)
	if len(entries) != 1 {
		t.Fatalf("esperado apenas repositorio com gguf, obtido %d", len(entries))
	}
	e := entries[0]
	if e.RepoId != "org/tiny" || e.File != "tiny-Q4_K_M.gguf" || e.Quantization != "Q4_K_M" || e.SizeBytes != 1000 {
		t.Fatalf("entry %+v", e)
	}
	if e.Name != "tiny Q4_K_M" {
		t.Fatalf("name %s", e.Name)
	}
	if e.Download != nil {
		t.Fatalf("download devia ser null")
	}
}

func TestVersionEntriesSortedAndValidated(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/models/org/tiny/tree/main", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"path":"tiny-Q8_0.gguf","type":"file","size":2000},{"path":"tiny-Q4_K_M.gguf","type":"file","size":1000}]`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	client := newTestClient(t, ts, time.Minute)

	if _, herr := VersionEntries(context.Background(), client, "invalido", contract.TMachineProfile{}, t.TempDir()); herr == nil {
		t.Fatalf("repo invalido deve ser recusado")
	}
	entries, herr := VersionEntries(context.Background(), client, "org/tiny", contract.TMachineProfile{}, t.TempDir())
	if herr != nil {
		t.Fatalf("err %v", herr)
	}
	if len(entries) != 2 || entries[0].SizeBytes > entries[1].SizeBytes {
		t.Fatalf("esperado ordenacao por tamanho crescente: %+v", entries)
	}
}
