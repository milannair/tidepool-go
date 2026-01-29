package tidepool

import (
	"net/http"
	"time"
)

const (
	defaultQueryURL  = "http://localhost:8080"
	defaultIngestURL = "http://localhost:8081"
	defaultTimeout   = 30 * time.Second
	defaultNamespace = "default"
)

// Config holds client configuration.
type Config struct {
	QueryURL   string
	IngestURL  string
	Timeout    time.Duration
	DefaultNamespace string
	// Namespace is deprecated. Use DefaultNamespace.
	Namespace  string
	HTTPClient *http.Client
}

// Option configures the client.
type Option func(*Config)

// WithQueryURL sets the base URL for the query service.
func WithQueryURL(url string) Option {
	return func(c *Config) {
		c.QueryURL = url
	}
}

// WithIngestURL sets the base URL for the ingest service.
func WithIngestURL(url string) Option {
	return func(c *Config) {
		c.IngestURL = url
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.Timeout = d
	}
}

// WithNamespace sets the default namespace.
func WithNamespace(ns string) Option {
	return func(c *Config) {
		c.Namespace = ns
		c.DefaultNamespace = ns
	}
}

// WithDefaultNamespace sets the default namespace.
func WithDefaultNamespace(ns string) Option {
	return func(c *Config) {
		c.DefaultNamespace = ns
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Config) {
		c.HTTPClient = client
	}
}
