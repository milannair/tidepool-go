# Tidepool Go Client

Go client library for the Tidepool Query and Ingest services.

## Status

The client implementation is available and tracks the v1 Tidepool API contract.

## Documentation

- `tidepool-go-client-design.md` — API contract and usage examples.
- `docs/GO_CLIENT.md` — Dynamic namespace usage and API reference.

## Package Layout

```
tidepool/
├── client.go       // Main client implementation
├── types.go        // Data types and enums
├── errors.go       // Error types
└── options.go      // Functional options
```

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
	)

	docs := []tidepool.Document{
		{
			ID:     "doc-1",
			Vector: []float32{0.1, 0.2, 0.3},
			Attributes: map[string]any{
				"title": "Example",
			},
		},
	}

	if err := client.Upsert(ctx, docs, nil); err != nil {
		log.Fatal(err)
	}

	response, err := client.Query(ctx, []float32{0.1, 0.2, 0.3}, &tidepool.QueryOptions{
		TopK: 5,
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, r := range response.Results {
		fmt.Printf("%s: %.4f\n", r.ID, r.Dist)
	}
}
```

## Contributing

Contributions are welcome. Please open an issue to discuss changes before submitting a PR.

## License

MIT
