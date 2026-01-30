# Tidepool Go Client

Go client library for the Tidepool Query and Ingest services.

## Highlights

- Typed request/response models
- Vector, text, and hybrid queries
- Namespace management and status endpoints
- Small surface area with explicit validation

## Requirements

- Go 1.24+

## Installation

```bash
go get github.com/milannair/tidepool-go
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/milannair/tidepool-go/tidepool"
)

func main() {
	ctx := context.Background()

	client := tidepool.New(
		tidepool.WithQueryURL("http://localhost:8080"),
		tidepool.WithIngestURL("http://localhost:8081"),
		tidepool.WithTimeout(30*time.Second),
		tidepool.WithDefaultNamespace("default"),
	)

	docs := []tidepool.Document{
		{
			ID:     "doc-1",
			Vector: []float32{0.1, 0.2, 0.3},
			Text:   "machine learning guide",
			Attributes: map[string]any{
				"category": "news",
			},
		},
	}

	if err := client.Upsert(ctx, docs, nil); err != nil {
		log.Fatal(err)
	}

	alpha := float32(0.7)
	resp, err := client.Query(ctx, []float32{0.1, 0.2, 0.3}, &tidepool.QueryOptions{
		TopK: 5,
		Text: "neural networks",
		Mode: tidepool.QueryModeHybrid,
		Alpha: &alpha,
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, r := range resp.Results {
		fmt.Printf("%s: %.4f\n", r.ID, r.Score)
	}
}
```

## Configuration

- `WithQueryURL` and `WithIngestURL` set base URLs. Defaults are:
  - Query: `http://localhost:8080`
  - Ingest: `http://localhost:8081`
- `WithDefaultNamespace` sets the namespace used when a request does not provide one. Default is `default`.
- `WithNamespace` is supported for backward compatibility but `WithDefaultNamespace` is preferred.
- `WithTimeout` sets the HTTP timeout on the underlying client.
- `WithHTTPClient` lets you supply a custom `*http.Client` (custom transport, proxy, TLS config, etc.).

## Namespaces

Each write/query/delete can target a specific namespace. If omitted, the client falls back to the configured default namespace.

```go
err := client.Upsert(ctx, docs, &tidepool.UpsertOptions{Namespace: "tenant-a"})
```

## Query Modes

- Vector-only search: provide a vector, omit `Text`.
- Text-only search: set `Text`, omit `Vector`, use `Mode: text`.
- Hybrid search: provide both and set `Mode: hybrid`.

```go
resp, err := client.Query(ctx, nil, &tidepool.QueryOptions{
	Text: "fraud detection",
	Mode: tidepool.QueryModeText,
	TopK: 10,
})
```

## Error Handling

Errors are mapped to sentinel errors for reliable checks:

- `ErrValidation`
- `ErrNotFound`
- `ErrServiceUnavailable`

```go
if err != nil {
	if tidepool.IsValidationError(err) {
		// invalid input
	}
}
```

## Retries

Retries are not built in. If you need retries, wrap calls with your own backoff logic or use a custom `http.Client` transport.

## Testing

```bash
go test ./...
```

## Documentation

- `tidepool-go-client-design.md` — API contract and usage examples
- `docs/GO_CLIENT.md` — namespace usage and API reference

## Contributing

Contributions are welcome. Please open an issue to discuss changes before submitting a PR.

## License

MIT
