# Tidepool Go Client (Dynamic Namespaces)

Tidepool Phase 8 adds dynamic namespaces. The Go client now supports a default
namespace plus per-request overrides so a single client can operate on multiple
namespaces.

## Client Initialization

```go
client := tidepool.New(
	tidepool.WithQueryURL("http://localhost:8080"),
	tidepool.WithIngestURL("http://localhost:8081"),
	tidepool.WithDefaultNamespace("default"), // Optional default
)
```

`WithNamespace` remains as a backward-compatible alias for `WithDefaultNamespace`.

## Method Signatures

```go
client.Upsert(ctx, docs, &tidepool.UpsertOptions{Namespace: "products"})
client.Query(ctx, vector, &tidepool.QueryOptions{Namespace: "products"})
client.Delete(ctx, ids, &tidepool.DeleteOptions{Namespace: "products"})

client.GetNamespace(ctx, "products")
client.ListNamespaces(ctx)

client.GetNamespaceStatus(ctx, "products")
client.Compact(ctx, "products")

client.Status(ctx) // Ingest service status (global)
client.Health(ctx, "query" | "ingest")
```

Pass an empty string to use the configured default namespace.

`ListNamespaces` returns a slice of `NamespaceInfo` entries (not just names), matching the query service response.

## Response Models

```go
type NamespaceStatus struct {
	LastRun    *time.Time `json:"last_run,omitempty"`
	WALFiles   int        `json:"wal_files"`
	WALEntries int        `json:"wal_entries"`
	Segments   int        `json:"segments"`
	TotalVecs  int        `json:"total_vecs"`
	Dimensions int        `json:"dimensions"`
}

type QueryResponse struct {
	Results   []VectorResult `json:"results"`
	Namespace string         `json:"namespace"`
}

type NamespaceInfo struct {
	Namespace         string `json:"namespace"`
	ApproxCount       int64  `json:"approx_count"`
	Dimensions        int    `json:"dimensions"`
	PendingCompaction *bool  `json:"pending_compaction,omitempty"`
}
```

`QueryResponse.Namespace` returns the namespace that was queried.

## Usage Examples

### Multi-Tenant Application

```go
client := tidepool.New(
	tidepool.WithQueryURL("..."),
	tidepool.WithIngestURL("..."),
)

// Each tenant gets their own namespace
func indexTenantData(ctx context.Context, tenantID string, documents []map[string]any) error {
	vectors := make([]tidepool.Document, 0, len(documents))
	for i, doc := range documents {
		vectors = append(vectors, tidepool.Document{
			ID:     fmt.Sprintf("%s-%d", tenantID, i),
			Vector: embed(doc),
		})
	}
	return client.Upsert(ctx, vectors, &tidepool.UpsertOptions{Namespace: "tenant_" + tenantID})
}

func searchTenant(ctx context.Context, tenantID string, query string, topK int) (*tidepool.QueryResponse, error) {
	queryVec := embed(query)
	return client.Query(ctx, queryVec, &tidepool.QueryOptions{TopK: topK, Namespace: "tenant_" + tenantID})
}
```

### Different Data Types

```go
client := tidepool.New(tidepool.WithDefaultNamespace("products"))

_ = client.Upsert(ctx, productVectors, &tidepool.UpsertOptions{Namespace: "products"})
_ = client.Upsert(ctx, userVectors, &tidepool.UpsertOptions{Namespace: "users"})
_ = client.Upsert(ctx, docVectors, &tidepool.UpsertOptions{Namespace: "documents"})

response, _ := client.Query(ctx, queryVec, &tidepool.QueryOptions{Namespace: "products"})
results := response.Results

status, _ := client.GetNamespaceStatus(ctx, "products")
fmt.Printf("Products: %d vectors in %d segments\n", status.TotalVecs, status.Segments)
```

### Namespace Management

```go
status, _ := client.GetNamespaceStatus(ctx, "products")
if status.WALEntries > 1000 {
	_ = client.Compact(ctx, "products")
	fmt.Println("Compaction triggered")
}
```

## Error Handling

If a namespace is restricted by `ALLOWED_NAMESPACES`, the API returns:

```
404 Not Found
{"error": "namespace not found"}
```

The client surfaces this as `ErrNotFound` with the provided message.
