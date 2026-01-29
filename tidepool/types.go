package tidepool

import "time"

// Vector is a slice of 32-bit floating point numbers.
type Vector []float32

// AttrValue represents any JSON-compatible value.
type AttrValue = any

// Attributes is a map of string keys to JSON values.
type Attributes map[string]AttrValue

// Document represents a vector with metadata.
type Document struct {
	ID         string     `json:"id"`
	Vector     Vector     `json:"vector,omitempty"`
	Attributes Attributes `json:"attributes,omitempty"`
}

// VectorResult is a single query result.
type VectorResult struct {
	ID         string     `json:"id"`
	Dist       float32    `json:"dist"`
	Vector     Vector     `json:"vector,omitempty"`
	Attributes Attributes `json:"attributes,omitempty"`
}

// QueryResponse represents a query response with namespace context.
type QueryResponse struct {
	Results   []VectorResult `json:"results"`
	Namespace string         `json:"namespace"`
}

// DistanceMetric controls how distances are computed.
type DistanceMetric string

const (
	DistanceCosine     DistanceMetric = "cosine_distance"
	DistanceEuclidean  DistanceMetric = "euclidean_squared"
	DistanceDotProduct DistanceMetric = "dot_product"
)

// NamespaceInfo describes a namespace.
type NamespaceInfo struct {
	Namespace         string `json:"namespace"`
	ApproxCount       int64  `json:"approx_count"`
	Dimensions        int    `json:"dimensions"`
	PendingCompaction *bool  `json:"pending_compaction,omitempty"`
}

// NamespaceStatus describes namespace compaction state.
type NamespaceStatus struct {
	LastRun    *time.Time `json:"last_run,omitempty"`
	WALFiles   int        `json:"wal_files"`
	WALEntries int        `json:"wal_entries"`
	Segments   int        `json:"segments"`
	TotalVecs  int        `json:"total_vecs"`
	Dimensions int        `json:"dimensions"`
}

// IngestStatus describes ingest service state.
type IngestStatus struct {
	LastRun    *time.Time `json:"last_run,omitempty"`
	WALFiles   int        `json:"wal_files"`
	WALEntries int        `json:"wal_entries"`
	Segments   int        `json:"segments"`
	TotalVecs  int        `json:"total_vecs"`
	Dimensions int        `json:"dimensions"`
}

// HealthResponse contains service health information.
type HealthResponse struct {
	Service string `json:"service"`
	Status  string `json:"status"`
}

// UpsertOptions configures upsert behavior.
type UpsertOptions struct {
	Namespace      string
	DistanceMetric DistanceMetric
}

// QueryOptions configures query behavior.
type QueryOptions struct {
	TopK           int
	Namespace      string
	DistanceMetric DistanceMetric
	IncludeVectors bool
	Filters        Attributes
	EfSearch       int
	NProbe         int
}

// DeleteOptions configures delete behavior.
type DeleteOptions struct {
	Namespace string
}
