package tidepool

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewDefaultsAndOptions(t *testing.T) {
	client := New()
	if client.config.QueryURL != defaultQueryURL {
		t.Fatalf("expected default query url %q, got %q", defaultQueryURL, client.config.QueryURL)
	}
	if client.config.IngestURL != defaultIngestURL {
		t.Fatalf("expected default ingest url %q, got %q", defaultIngestURL, client.config.IngestURL)
	}
	if client.config.DefaultNamespace != defaultNamespace {
		t.Fatalf("expected default namespace %q, got %q", defaultNamespace, client.config.DefaultNamespace)
	}
	if client.http == nil {
		t.Fatalf("expected http client to be initialized")
	}

	overridden := New(WithNamespace("tenant-a"), WithDefaultNamespace("tenant-b"))
	if overridden.config.Namespace != "tenant-a" {
		t.Fatalf("expected namespace tenant-a, got %q", overridden.config.Namespace)
	}
	if overridden.config.DefaultNamespace != "tenant-b" {
		t.Fatalf("expected default namespace tenant-b, got %q", overridden.config.DefaultNamespace)
	}

	customHTTP := &http.Client{}
	withHTTP := New(WithHTTPClient(customHTTP))
	if withHTTP.http != customHTTP {
		t.Fatalf("expected custom http client to be used")
	}
	if customHTTP.Timeout != defaultTimeout {
		t.Fatalf("expected timeout %s, got %s", defaultTimeout, customHTTP.Timeout)
	}

	customHTTP2 := &http.Client{Timeout: 5 * time.Second}
	_ = New(WithHTTPClient(customHTTP2), WithTimeout(12*time.Second))
	if customHTTP2.Timeout != 12*time.Second {
		t.Fatalf("expected timeout to be overridden, got %s", customHTTP2.Timeout)
	}
}

func TestNamespaceOrDefaultErrorsWhenMissing(t *testing.T) {
	client := New(WithDefaultNamespace(""), WithNamespace(""))
	_, err := client.namespaceOrDefault("")
	if err == nil {
		t.Fatalf("expected error when namespace missing")
	}
}

func TestValidateVector(t *testing.T) {
	if err := ValidateVector(Vector{}, 0); err == nil {
		t.Fatalf("expected error for empty vector")
	}
	if err := ValidateVector(Vector{0.1, 0.2}, 3); err == nil {
		t.Fatalf("expected error for dimension mismatch")
	}
	if err := ValidateVector(Vector{float32(math.NaN())}, 0); err == nil {
		t.Fatalf("expected error for NaN")
	}
	if err := ValidateVector(Vector{float32(math.Inf(1))}, 0); err == nil {
		t.Fatalf("expected error for Inf")
	}
	if err := ValidateVector(Vector{0.1, 0.2}, 2); err != nil {
		t.Fatalf("expected valid vector, got %v", err)
	}
}

func TestVectorResultUnmarshalJSON(t *testing.T) {
	cases := []struct {
		name string
		json string
		exp  float32
	}{
		{"score", `{"id":"a","score":0.4}`, 0.4},
		{"dist", `{"id":"a","dist":0.5}`, 0.5},
		{"distance", `{"id":"a","distance":0.6}`, 0.6},
		{"default", `{"id":"a"}`, 0},
	}
	for _, tc := range cases {
		var result VectorResult
		if err := json.Unmarshal([]byte(tc.json), &result); err != nil {
			t.Fatalf("%s: unmarshal failed: %v", tc.name, err)
		}
		if result.Score != tc.exp {
			t.Fatalf("%s: expected score %v, got %v", tc.name, tc.exp, result.Score)
		}
	}
}

func TestDecodeQueryResponse(t *testing.T) {
	direct := `[{"id":"a","score":0.1}]`
	resp, err := decodeQueryResponse([]byte(direct), "fallback")
	if err != nil {
		t.Fatalf("direct decode failed: %v", err)
	}
	if resp.Namespace != "fallback" || len(resp.Results) != 1 {
		t.Fatalf("unexpected direct response: %+v", resp)
	}

	wrapped := `{"namespace":"ns","results":[{"id":"b","score":0.2}]}`
	resp, err = decodeQueryResponse([]byte(wrapped), "fallback")
	if err != nil {
		t.Fatalf("wrapped decode failed: %v", err)
	}
	if resp.Namespace != "ns" || resp.Results[0].ID != "b" {
		t.Fatalf("unexpected wrapped response: %+v", resp)
	}

	vectors := `{"vectors":[{"id":"c","score":0.3}]}`
	resp, err = decodeQueryResponse([]byte(vectors), "fallback")
	if err != nil {
		t.Fatalf("vectors decode failed: %v", err)
	}
	if resp.Namespace != "fallback" || resp.Results[0].ID != "c" {
		t.Fatalf("unexpected vectors response: %+v", resp)
	}

	invalid := `{"namespace":"ns"}`
	if _, err := decodeQueryResponse([]byte(invalid), "fallback"); err == nil {
		t.Fatalf("expected error for missing results")
	}
}

func TestDecodeNamespaces(t *testing.T) {
	wrapped := `{"namespaces":[{"namespace":"a"},{"namespace":"b"}]}`
	infos, err := decodeNamespaces([]byte(wrapped))
	if err != nil || len(infos) != 2 {
		t.Fatalf("wrapped decode failed: %v", err)
	}

	direct := `[{"namespace":"c"}]`
	infos, err = decodeNamespaces([]byte(direct))
	if err != nil || len(infos) != 1 || infos[0].Namespace != "c" {
		t.Fatalf("direct decode failed: %v", err)
	}

	legacy := `["d","e"]`
	infos, err = decodeNamespaces([]byte(legacy))
	if err != nil || len(infos) != 2 || infos[1].Namespace != "e" {
		t.Fatalf("legacy decode failed: %v", err)
	}

	legacyWrapped := `{"namespace_list":["f","g"]}`
	infos, err = decodeNamespaces([]byte(legacyWrapped))
	if err != nil || len(infos) != 2 || infos[0].Namespace != "f" {
		t.Fatalf("legacy wrapped decode failed: %v", err)
	}

	invalid := `{"namespaces":null}`
	if _, err := decodeNamespaces([]byte(invalid)); err == nil {
		t.Fatalf("expected error for missing namespaces")
	}
}

func TestHandleErrorMapping(t *testing.T) {
	client := New()

	validation := client.handleError(http.StatusBadRequest, []byte(`{"error":"bad"}`))
	if !IsValidationError(validation) {
		t.Fatalf("expected validation error, got %v", validation)
	}

	notFound := client.handleError(http.StatusNotFound, []byte(`{"error":"missing"}`))
	if !IsNotFoundError(notFound) {
		t.Fatalf("expected not found error, got %v", notFound)
	}

	unavailable := client.handleError(http.StatusServiceUnavailable, []byte(`{"error":"down"}`))
	if !IsServiceUnavailableError(unavailable) {
		t.Fatalf("expected service unavailable error, got %v", unavailable)
	}

	generic := client.handleError(http.StatusInternalServerError, []byte(`{"error":"boom"}`))
	if !strings.Contains(generic.Error(), "boom") {
		t.Fatalf("expected error message to include boom")
	}
}

func TestDoRequestHeaders(t *testing.T) {
	t.Run("no body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Accept") != "application/json" {
				t.Fatalf("expected Accept header to be application/json")
			}
			if r.Header.Get("Content-Type") != "" {
				t.Fatalf("expected Content-Type header to be empty")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		client := New(WithHTTPClient(srv.Client()))
		if _, err := client.doRequest(context.Background(), http.MethodGet, srv.URL, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("with body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Accept") != "application/json" {
				t.Fatalf("expected Accept header to be application/json")
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Fatalf("expected Content-Type header to be application/json")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		client := New(WithHTTPClient(srv.Client()))
		if _, err := client.doRequest(context.Background(), http.MethodPost, srv.URL, map[string]any{"a": 1}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestQueryValidation(t *testing.T) {
	client := New(WithDefaultNamespace("default"))
	ctx := context.Background()

	_, err := client.Query(ctx, Vector{0.1}, &QueryOptions{TopK: -1})
	if err == nil {
		t.Fatalf("expected error for negative top_k")
	}

	_, err = client.Query(ctx, Vector{0.1}, &QueryOptions{Mode: "invalid"})
	if err == nil {
		t.Fatalf("expected error for invalid mode")
	}

	_, err = client.Query(ctx, Vector{0.1}, &QueryOptions{Fusion: "invalid"})
	if err == nil {
		t.Fatalf("expected error for invalid fusion")
	}

	_, err = client.Query(ctx, nil, &QueryOptions{Mode: QueryModeVector})
	if err == nil {
		t.Fatalf("expected error for missing vector")
	}

	_, err = client.Query(ctx, Vector{0.1}, &QueryOptions{Mode: QueryModeText})
	if err == nil {
		t.Fatalf("expected error for missing text")
	}

	_, err = client.Query(ctx, Vector{0.1}, &QueryOptions{Mode: QueryModeHybrid, Text: ""})
	if err == nil {
		t.Fatalf("expected error for missing hybrid text")
	}

	rrf := 0
	_, err = client.Query(ctx, Vector{0.1}, &QueryOptions{RRFK: &rrf, Text: "hi"})
	if err == nil {
		t.Fatalf("expected error for invalid rrf_k")
	}
}

func TestQueryRequestPayload(t *testing.T) {
	var captured map[string]any
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		_ = json.NewEncoder(w).Encode([]VectorResult{{ID: "a", Score: 0.1}})
	}))
	defer srv.Close()

	client := New(WithQueryURL(srv.URL), WithDefaultNamespace("default"))
	alpha := float32(1.5)
	rrf := 10
	_, err := client.Query(context.Background(), Vector{0.1, 0.2}, &QueryOptions{
		Text:           "hello",
		TopK:           7,
		IncludeVectors: false,
		DistanceMetric: DistanceCosine,
		Filters:        Attributes{"tag": "a"},
		Alpha:          &alpha,
		Fusion:         FusionRRF,
		RRFK:           &rrf,
	})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if path != "/v1/vectors/default" {
		t.Fatalf("expected query path /v1/vectors/default, got %q", path)
	}
	if captured["mode"] != "hybrid" {
		t.Fatalf("expected mode hybrid, got %v", captured["mode"])
	}
	if captured["include_vectors"] != false {
		t.Fatalf("expected include_vectors false")
	}
	if captured["distance_metric"] != string(DistanceCosine) {
		t.Fatalf("expected distance_metric cosine_distance")
	}
	if captured["top_k"] != float64(7) {
		t.Fatalf("expected top_k 7")
	}
	if captured["fusion"] != string(FusionRRF) {
		t.Fatalf("expected fusion rrf")
	}
	if captured["rrf_k"] != float64(10) {
		t.Fatalf("expected rrf_k 10")
	}
	alphaVal, _ := captured["alpha"].(float64)
	if alphaVal != 1.0 {
		t.Fatalf("expected alpha clamped to 1.0, got %v", captured["alpha"])
	}
}

func TestTextOnlyQueryPayload(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(QueryResponse{Namespace: "docs", Results: []VectorResult{{ID: "a", Score: 0.1}}})
	}))
	defer srv.Close()

	client := New(WithQueryURL(srv.URL), WithDefaultNamespace("docs"))
	_, err := client.Query(context.Background(), nil, &QueryOptions{Text: "hello", Mode: QueryModeText})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if _, ok := captured["vector"]; ok {
		t.Fatalf("expected vector omitted for text-only query")
	}
	if captured["mode"] != "text" {
		t.Fatalf("expected mode text, got %v", captured["mode"])
	}
}

func TestUpsertDeleteValidation(t *testing.T) {
	client := New(WithDefaultNamespace("default"))
	if err := client.Upsert(context.Background(), nil, nil); err == nil {
		t.Fatalf("expected error for empty upsert")
	}
	if err := client.Delete(context.Background(), nil, nil); err == nil {
		t.Fatalf("expected error for empty delete")
	}
}
