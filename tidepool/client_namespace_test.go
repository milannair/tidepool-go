package tidepool

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type requestRecorder struct {
	mu    sync.Mutex
	paths []string
}

func (r *requestRecorder) record(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.paths = append(r.paths, path)
}

func (r *requestRecorder) contains(path string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.paths {
		if p == path {
			return true
		}
	}
	return false
}

func (r *requestRecorder) count(path string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, p := range r.paths {
		if p == path {
			count++
		}
	}
	return count
}

func newIngestServer(recorder *requestRecorder) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		recorder.record(req.URL.Path)
		if strings.HasSuffix(req.URL.Path, "/status") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"last_run":    "2025-01-01T00:00:00Z",
				"wal_files":   1,
				"wal_entries": 2,
				"segments":    3,
				"total_vecs":  4,
				"dimensions":  5,
			})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}

func newQueryServer(recorder *requestRecorder) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		recorder.record(req.URL.Path)
		if req.Method == http.MethodGet && req.URL.Path == "/v1/namespaces" {
			pending := true
			_ = json.NewEncoder(w).Encode(map[string]any{
				"namespaces": []NamespaceInfo{
					{
						Namespace:         "default",
						ApproxCount:       10,
						Dimensions:        3,
						PendingCompaction: &pending,
					},
					{
						Namespace:   "products",
						ApproxCount: 5,
						Dimensions:  3,
					},
				},
			})
			return
		}
		if req.Method == http.MethodPost && strings.HasPrefix(req.URL.Path, "/v1/vectors/") {
			parts := strings.Split(strings.TrimSuffix(req.URL.Path, "/"), "/")
			namespace := parts[len(parts)-1]
			_ = json.NewEncoder(w).Encode(QueryResponse{
				Namespace: namespace,
				Results: []VectorResult{
					{ID: "a", Score: 0.1},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}

func TestNamespaceOverrides(t *testing.T) {
	ctx := context.Background()
	ingestRecorder := &requestRecorder{}
	queryRecorder := &requestRecorder{}
	ingestServer := newIngestServer(ingestRecorder)
	queryServer := newQueryServer(queryRecorder)
	defer ingestServer.Close()
	defer queryServer.Close()

	client := New(
		WithIngestURL(ingestServer.URL),
		WithQueryURL(queryServer.URL),
		WithDefaultNamespace("default"),
	)

	docs := []Document{{ID: "doc-1", Vector: Vector{0.1, 0.2, 0.3}}}
	if err := client.Upsert(ctx, docs, &UpsertOptions{Namespace: "products"}); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	response, err := client.Query(ctx, Vector{0.1, 0.2, 0.3}, &QueryOptions{Namespace: "products"})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if response.Namespace != "products" {
		t.Fatalf("expected namespace products, got %q", response.Namespace)
	}

	if err := client.Delete(ctx, []string{"doc-1"}, &DeleteOptions{Namespace: "products"}); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if ingestRecorder.count("/v1/vectors/products") != 2 {
		t.Fatalf("expected 2 ingest calls to /v1/vectors/products, got %d", ingestRecorder.count("/v1/vectors/products"))
	}
	if !queryRecorder.contains("/v1/vectors/products") {
		t.Fatalf("expected query call to /v1/vectors/products")
	}
}

func TestDefaultNamespaceFallback(t *testing.T) {
	ctx := context.Background()
	ingestRecorder := &requestRecorder{}
	queryRecorder := &requestRecorder{}
	ingestServer := newIngestServer(ingestRecorder)
	queryServer := newQueryServer(queryRecorder)
	defer ingestServer.Close()
	defer queryServer.Close()

	client := New(
		WithIngestURL(ingestServer.URL),
		WithQueryURL(queryServer.URL),
		WithDefaultNamespace("default"),
	)

	docs := []Document{{ID: "doc-1", Vector: Vector{0.1, 0.2, 0.3}}}
	if err := client.Upsert(ctx, docs, nil); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	if !ingestRecorder.contains("/v1/vectors/default") {
		t.Fatalf("expected default namespace path /v1/vectors/default")
	}
}

func TestNamespaceStatusAndCompact(t *testing.T) {
	ctx := context.Background()
	ingestRecorder := &requestRecorder{}
	queryRecorder := &requestRecorder{}
	ingestServer := newIngestServer(ingestRecorder)
	queryServer := newQueryServer(queryRecorder)
	defer ingestServer.Close()
	defer queryServer.Close()

	client := New(
		WithIngestURL(ingestServer.URL),
		WithQueryURL(queryServer.URL),
		WithDefaultNamespace("default"),
	)

	status, err := client.GetNamespaceStatus(ctx, "products")
	if err != nil {
		t.Fatalf("get namespace status failed: %v", err)
	}
	if status.WALEntries != 2 {
		t.Fatalf("expected wal entries 2, got %d", status.WALEntries)
	}

	if err := client.Compact(ctx, "products"); err != nil {
		t.Fatalf("compact failed: %v", err)
	}

	if !ingestRecorder.contains("/v1/namespaces/products/status") {
		t.Fatalf("expected namespace status endpoint call")
	}
	if !ingestRecorder.contains("/v1/namespaces/products/compact") {
		t.Fatalf("expected namespace compact endpoint call")
	}
}

func TestCrossNamespaceIsolation(t *testing.T) {
	ctx := context.Background()
	ingestRecorder := &requestRecorder{}
	queryRecorder := &requestRecorder{}
	ingestServer := newIngestServer(ingestRecorder)
	queryServer := newQueryServer(queryRecorder)
	defer ingestServer.Close()
	defer queryServer.Close()

	client := New(
		WithIngestURL(ingestServer.URL),
		WithQueryURL(queryServer.URL),
		WithDefaultNamespace("default"),
	)

	a, err := client.Query(ctx, Vector{0.1, 0.2, 0.3}, &QueryOptions{Namespace: "tenant_a"})
	if err != nil {
		t.Fatalf("query tenant_a failed: %v", err)
	}
	b, err := client.Query(ctx, Vector{0.1, 0.2, 0.3}, &QueryOptions{Namespace: "tenant_b"})
	if err != nil {
		t.Fatalf("query tenant_b failed: %v", err)
	}

	if a.Namespace != "tenant_a" || b.Namespace != "tenant_b" {
		t.Fatalf("expected tenant namespaces, got %q and %q", a.Namespace, b.Namespace)
	}
	if !queryRecorder.contains("/v1/vectors/tenant_a") || !queryRecorder.contains("/v1/vectors/tenant_b") {
		t.Fatalf("expected queries against tenant namespaces")
	}
}

func TestListNamespacesReturnsInfo(t *testing.T) {
	ctx := context.Background()
	ingestRecorder := &requestRecorder{}
	queryRecorder := &requestRecorder{}
	ingestServer := newIngestServer(ingestRecorder)
	queryServer := newQueryServer(queryRecorder)
	defer ingestServer.Close()
	defer queryServer.Close()

	client := New(
		WithIngestURL(ingestServer.URL),
		WithQueryURL(queryServer.URL),
		WithDefaultNamespace("default"),
	)

	infos, err := client.ListNamespaces(ctx)
	if err != nil {
		t.Fatalf("list namespaces failed: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2 namespaces, got %d", len(infos))
	}
	if infos[0].Namespace != "default" || infos[0].ApproxCount != 10 || infos[0].Dimensions != 3 {
		t.Fatalf("unexpected namespace info: %+v", infos[0])
	}
	if infos[0].PendingCompaction == nil || *infos[0].PendingCompaction != true {
		t.Fatalf("expected pending_compaction true, got %+v", infos[0].PendingCompaction)
	}
}

func TestTextOnlyQuery(t *testing.T) {
	ctx := context.Background()
	ingestRecorder := &requestRecorder{}
	queryRecorder := &requestRecorder{}
	ingestServer := newIngestServer(ingestRecorder)
	queryServer := newQueryServer(queryRecorder)
	defer ingestServer.Close()
	defer queryServer.Close()

	client := New(
		WithIngestURL(ingestServer.URL),
		WithQueryURL(queryServer.URL),
		WithDefaultNamespace("default"),
	)

	response, err := client.Query(ctx, nil, &QueryOptions{
		Text: "machine learning",
		Mode: QueryModeText,
	})
	if err != nil {
		t.Fatalf("text query failed: %v", err)
	}
	if response.Namespace != "default" {
		t.Fatalf("expected default namespace, got %q", response.Namespace)
	}
	if !queryRecorder.contains("/v1/vectors/default") {
		t.Fatalf("expected text query against default namespace")
	}
}
