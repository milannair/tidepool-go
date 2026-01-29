package tidepool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Client is the Tidepool API client.
type Client struct {
	config Config
	http   *http.Client
}

// New creates a new Tidepool client.
func New(opts ...Option) *Client {
	cfg := Config{
		QueryURL:  defaultQueryURL,
		IngestURL: defaultIngestURL,
		Timeout:   defaultTimeout,
		Namespace: defaultNamespace,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: cfg.Timeout,
		}
	} else if cfg.Timeout > 0 {
		httpClient.Timeout = cfg.Timeout
	}

	return &Client{
		config: cfg,
		http:   httpClient,
	}
}

// Health checks service health. Service should be "query" or "ingest".
func (c *Client) Health(ctx context.Context, service string) (*HealthResponse, error) {
	baseURL, err := c.serviceBaseURL(service)
	if err != nil {
		return nil, err
	}

	endpoint, err := joinURL(baseURL, "health")
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var resp HealthResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode health response: %w", err)
	}

	return &resp, nil
}

// Upsert inserts or updates vectors.
func (c *Client) Upsert(ctx context.Context, docs []Document, opts *UpsertOptions) error {
	if len(docs) == 0 {
		return fmt.Errorf("%w: no documents provided", ErrValidation)
	}

	desiredNamespace := ""
	if opts != nil {
		desiredNamespace = opts.Namespace
	}
	namespace, err := c.namespaceOrDefault(desiredNamespace)
	if err != nil {
		return err
	}

	endpoint, err := c.ingestVectorsEndpoint(namespace)
	if err != nil {
		return err
	}

	req := struct {
		Vectors        []Document     `json:"vectors"`
		DistanceMetric DistanceMetric `json:"distance_metric,omitempty"`
	}{
		Vectors: docs,
	}
	if opts != nil && opts.DistanceMetric != "" {
		req.DistanceMetric = opts.DistanceMetric
	}

	_, err = c.doRequest(ctx, http.MethodPost, endpoint, req)
	return err
}

// Query searches for similar vectors.
func (c *Client) Query(ctx context.Context, vector Vector, opts *QueryOptions) ([]VectorResult, error) {
	if err := ValidateVector(vector, 0); err != nil {
		return nil, err
	}

	desiredNamespace := ""
	if opts != nil {
		desiredNamespace = opts.Namespace
	}
	namespace, err := c.namespaceOrDefault(desiredNamespace)
	if err != nil {
		return nil, err
	}

	endpoint, err := c.queryVectorsEndpoint(namespace)
	if err != nil {
		return nil, err
	}

	req := struct {
		Vector         Vector         `json:"vector"`
		TopK           int            `json:"top_k,omitempty"`
		EfSearch       int            `json:"ef_search,omitempty"`
		NProbe         int            `json:"nprobe,omitempty"`
		DistanceMetric DistanceMetric `json:"distance_metric,omitempty"`
		IncludeVectors *bool          `json:"include_vectors,omitempty"`
		Filters        Attributes     `json:"filters,omitempty"`
	}{
		Vector: vector,
	}

	if opts != nil {
		if opts.TopK > 0 {
			req.TopK = opts.TopK
		}
		if opts.EfSearch > 0 {
			req.EfSearch = opts.EfSearch
		}
		if opts.NProbe > 0 {
			req.NProbe = opts.NProbe
		}
		if opts.DistanceMetric != "" {
			req.DistanceMetric = opts.DistanceMetric
		}
		req.Filters = opts.Filters
		req.IncludeVectors = &opts.IncludeVectors
	}

	body, err := c.doRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, err
	}

	results, err := decodeVectorResults(body)
	if err != nil {
		return nil, err
	}

	return results, nil
}

// Delete removes vectors by ID.
func (c *Client) Delete(ctx context.Context, ids []string, opts *DeleteOptions) error {
	if len(ids) == 0 {
		return fmt.Errorf("%w: no ids provided", ErrValidation)
	}

	desiredNamespace := ""
	if opts != nil {
		desiredNamespace = opts.Namespace
	}
	namespace, err := c.namespaceOrDefault(desiredNamespace)
	if err != nil {
		return err
	}

	endpoint, err := c.ingestVectorsEndpoint(namespace)
	if err != nil {
		return err
	}

	req := struct {
		IDs []string `json:"ids"`
	}{
		IDs: ids,
	}

	_, err = c.doRequest(ctx, http.MethodDelete, endpoint, req)
	return err
}

// GetNamespace returns namespace information.
func (c *Client) GetNamespace(ctx context.Context, namespace string) (*NamespaceInfo, error) {
	if namespace == "" {
		var err error
		namespace, err = c.namespaceOrDefault(namespace)
		if err != nil {
			return nil, err
		}
	}

	endpoint, err := joinURL(c.config.QueryURL, "v1", "namespaces", namespace)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var info NamespaceInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("decode namespace response: %w", err)
	}

	return &info, nil
}

// ListNamespaces returns all namespace names.
func (c *Client) ListNamespaces(ctx context.Context) ([]string, error) {
	endpoint, err := joinURL(c.config.QueryURL, "v1", "namespaces")
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	namespaces, err := decodeNamespaces(body)
	if err != nil {
		return nil, err
	}

	return namespaces, nil
}

// Status returns ingest service status.
func (c *Client) Status(ctx context.Context) (*IngestStatus, error) {
	endpoint, err := joinURL(c.config.IngestURL, "status")
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var status IngestStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("decode status response: %w", err)
	}

	return &status, nil
}

// Compact triggers manual compaction.
func (c *Client) Compact(ctx context.Context) error {
	endpoint, err := joinURL(c.config.IngestURL, "compact")
	if err != nil {
		return err
	}

	_, err = c.doRequest(ctx, http.MethodPost, endpoint, nil)
	return err
}

func (c *Client) ingestVectorsEndpoint(namespace string) (string, error) {
	if namespace == "" {
		return "", fmt.Errorf("%w: namespace is required", ErrValidation)
	}
	return joinURL(c.config.IngestURL, "v1", "vectors", namespace)
}

func (c *Client) queryVectorsEndpoint(namespace string) (string, error) {
	if namespace == "" {
		return "", fmt.Errorf("%w: namespace is required", ErrValidation)
	}
	return joinURL(c.config.QueryURL, "v1", "vectors", namespace)
}

func (c *Client) serviceBaseURL(service string) (string, error) {
	switch strings.ToLower(service) {
	case "query":
		return c.config.QueryURL, nil
	case "ingest":
		return c.config.IngestURL, nil
	default:
		return "", fmt.Errorf("%w: unknown service %q", ErrValidation, service)
	}
}

func (c *Client) namespaceOrDefault(namespace string) (string, error) {
	if namespace != "" {
		return namespace, nil
	}
	if c.config.Namespace != "" {
		return c.config.Namespace, nil
	}
	return "", fmt.Errorf("%w: namespace is required", ErrValidation)
}

func (c *Client) doRequest(ctx context.Context, method, endpoint string, body any) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, c.handleError(resp.StatusCode, respBody)
	}

	return respBody, nil
}

func (c *Client) handleError(statusCode int, body []byte) error {
	var errResp struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(body, &errResp)

	msg := strings.TrimSpace(errResp.Error)
	if msg == "" {
		msg = http.StatusText(statusCode)
	}

	tideErr := &TidepoolError{
		Message:    msg,
		StatusCode: statusCode,
		Response:   body,
	}

	switch statusCode {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge:
		return errors.Join(ErrValidation, tideErr)
	case http.StatusNotFound:
		return errors.Join(ErrNotFound, tideErr)
	case http.StatusServiceUnavailable:
		return errors.Join(ErrServiceUnavailable, tideErr)
	default:
		return tideErr
	}
}

func joinURL(base string, parts ...string) (string, error) {
	if base == "" {
		return "", fmt.Errorf("%w: base URL is required", ErrValidation)
	}
	return url.JoinPath(base, parts...)
}

func decodeVectorResults(data []byte) ([]VectorResult, error) {
	var direct []VectorResult
	if err := json.Unmarshal(data, &direct); err == nil {
		return direct, nil
	}

	var wrapped struct {
		Results []VectorResult `json:"results"`
		Vectors []VectorResult `json:"vectors"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, fmt.Errorf("decode query response: %w", err)
	}
	if wrapped.Results != nil {
		return wrapped.Results, nil
	}
	if wrapped.Vectors != nil {
		return wrapped.Vectors, nil
	}

	return nil, fmt.Errorf("decode query response: missing results")
}

func decodeNamespaces(data []byte) ([]string, error) {
	var direct []string
	if err := json.Unmarshal(data, &direct); err == nil {
		return direct, nil
	}

	var wrapped struct {
		Namespaces    []string `json:"namespaces"`
		NamespaceList []string `json:"namespace_list"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, fmt.Errorf("decode namespaces response: %w", err)
	}
	if wrapped.Namespaces != nil {
		return wrapped.Namespaces, nil
	}
	if wrapped.NamespaceList != nil {
		return wrapped.NamespaceList, nil
	}
	return nil, fmt.Errorf("decode namespaces response: missing namespaces")
}
