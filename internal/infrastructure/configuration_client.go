package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/config"
)

type ConfigurationClient struct {
	config     *config.Config
	logger     *logrus.Logger
	httpClient *http.Client
	baseURL    string
	cache      *ConfigCache
	metrics    *ConfigMetrics
}

type ConfigCache struct {
	mu     sync.RWMutex
	items  map[string]*CacheItem
	ttl    time.Duration
	logger *logrus.Logger
}

type CacheItem struct {
	Value     interface{}
	ExpiresAt time.Time
	CreatedAt time.Time
}

type ConfigMetrics struct {
	mu                sync.RWMutex
	requestCount      int64
	cacheHits         int64
	cacheMisses       int64
	responseTimeTotal time.Duration
	connectionStatus  string
	lastRequestTime   time.Time
	errorCount        int64
}

type ConfigurationRequest struct {
	Key         string      `json:"key"`
	Environment string      `json:"environment,omitempty"`
	Service     string      `json:"service,omitempty"`
	Version     string      `json:"version,omitempty"`
	Value       interface{} `json:"value,omitempty"`
}

type ConfigurationResponse struct {
	Key         string      `json:"key"`
	Value       interface{} `json:"value"`
	Environment string      `json:"environment"`
	Service     string      `json:"service"`
	Version     string      `json:"version"`
	UpdatedAt   time.Time   `json:"updated_at"`
	TTL         int         `json:"ttl_seconds"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewConfigurationClient(cfg *config.Config, logger *logrus.Logger) *ConfigurationClient {
	cache := &ConfigCache{
		items:  make(map[string]*CacheItem),
		ttl:    5 * time.Minute, // Default 5-minute TTL
		logger: logger,
	}

	metrics := &ConfigMetrics{
		connectionStatus: "unknown",
	}

	client := &ConfigurationClient{
		config: cfg,
		logger: logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  false,
				MaxIdleConnsPerHost: 5,
			},
		},
		baseURL: "http://localhost:8081", // Configuration service URL
		cache:   cache,
		metrics: metrics,
	}

	// Start cache cleanup routine
	go client.startCacheCleanup()

	return client
}

func (c *ConfigurationClient) GetConfiguration(ctx context.Context, key string) (interface{}, error) {
	start := time.Now()
	c.updateMetrics(func(m *ConfigMetrics) {
		m.requestCount++
		m.lastRequestTime = start
	})

	// Check cache first
	if value, found := c.cache.Get(key); found {
		c.updateMetrics(func(m *ConfigMetrics) {
			m.cacheHits++
		})
		c.logger.WithFields(logrus.Fields{
			"key":           key,
			"cache_hit":     true,
			"response_time": time.Since(start),
		}).Debug("Configuration retrieved from cache")
		return value, nil
	}

	// Cache miss - fetch from service
	c.updateMetrics(func(m *ConfigMetrics) {
		m.cacheMisses++
	})

	req := &ConfigurationRequest{
		Key:         key,
		Environment: "development", // Could be configurable
		Service:     c.config.ServiceName,
		Version:     c.config.ServiceVersion,
	}

	resp, err := c.makeRequest(ctx, "GET", "/api/v1/configuration", req)
	if err != nil {
		c.updateMetrics(func(m *ConfigMetrics) {
			m.errorCount++
			m.connectionStatus = "error"
		})
		return nil, fmt.Errorf("failed to get configuration: %w", err)
	}

	c.updateMetrics(func(m *ConfigMetrics) {
		m.connectionStatus = "healthy"
		m.responseTimeTotal += time.Since(start)
	})

	// Cache the result
	if resp.TTL > 0 {
		c.cache.SetWithTTL(key, resp.Value, time.Duration(resp.TTL)*time.Second)
	} else {
		c.cache.Set(key, resp.Value)
	}

	c.logger.WithFields(logrus.Fields{
		"key":           key,
		"cache_hit":     false,
		"response_time": time.Since(start),
		"ttl":           resp.TTL,
	}).Info("Configuration retrieved from service")

	return resp.Value, nil
}

func (c *ConfigurationClient) SetConfiguration(ctx context.Context, key string, value interface{}) error {
	start := time.Now()
	c.updateMetrics(func(m *ConfigMetrics) {
		m.requestCount++
		m.lastRequestTime = start
	})

	req := &ConfigurationRequest{
		Key:         key,
		Value:       value,
		Environment: "development",
		Service:     c.config.ServiceName,
		Version:     c.config.ServiceVersion,
	}

	_, err := c.makeRequest(ctx, "POST", "/api/v1/configuration", req)
	if err != nil {
		c.updateMetrics(func(m *ConfigMetrics) {
			m.errorCount++
			m.connectionStatus = "error"
		})
		return fmt.Errorf("failed to set configuration: %w", err)
	}

	c.updateMetrics(func(m *ConfigMetrics) {
		m.connectionStatus = "healthy"
		m.responseTimeTotal += time.Since(start)
	})

	// Invalidate cache for this key
	c.cache.Delete(key)

	c.logger.WithFields(logrus.Fields{
		"key":           key,
		"response_time": time.Since(start),
	}).Info("Configuration updated in service")

	return nil
}

func (c *ConfigurationClient) GetMetrics() map[string]interface{} {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()

	var avgResponseTime float64
	if c.metrics.requestCount > 0 {
		avgResponseTime = float64(c.metrics.responseTimeTotal) / float64(c.metrics.requestCount) / float64(time.Millisecond)
	}

	cacheSize := c.cache.Size()
	hitRate := float64(0)
	if c.metrics.cacheHits+c.metrics.cacheMisses > 0 {
		hitRate = float64(c.metrics.cacheHits) / float64(c.metrics.cacheHits+c.metrics.cacheMisses) * 100
	}

	return map[string]interface{}{
		"request_count":        c.metrics.requestCount,
		"cache_hits":           c.metrics.cacheHits,
		"cache_misses":         c.metrics.cacheMisses,
		"cache_hit_rate":       hitRate,
		"cache_size":           cacheSize,
		"avg_response_time_ms": avgResponseTime,
		"connection_status":    c.metrics.connectionStatus,
		"last_request_time":    c.metrics.lastRequestTime,
		"error_count":          c.metrics.errorCount,
	}
}

func (c *ConfigurationClient) InvalidateCache(key string) {
	c.cache.Delete(key)
	c.logger.WithField("key", key).Info("Cache invalidated for key")
}

func (c *ConfigurationClient) ClearCache() {
	c.cache.Clear()
	c.logger.Info("Cache cleared")
}

func (c *ConfigurationClient) HealthCheck(ctx context.Context) error {
	// Simple health check - try to get a known configuration or make a ping request
	_, err := c.makeRequest(ctx, "GET", "/api/v1/health", nil)
	if err != nil {
		c.updateMetrics(func(m *ConfigMetrics) {
			m.errorCount++
			m.connectionStatus = "error"
		})
	} else {
		c.updateMetrics(func(m *ConfigMetrics) {
			m.connectionStatus = "healthy"
		})
	}
	return err
}

func (c *ConfigurationClient) makeRequest(ctx context.Context, method, endpoint string, data interface{}) (*ConfigurationResponse, error) {
	var body io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request data: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("%s/%s", c.config.ServiceName, c.config.ServiceVersion))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errorResp ErrorResponse
		if err := json.Unmarshal(responseBody, &errorResp); err == nil {
			return nil, fmt.Errorf("service error: %s (code: %d)", errorResp.Message, errorResp.Code)
		}
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	// For health check, we don't need to parse the response
	if endpoint == "/api/v1/health" {
		return &ConfigurationResponse{}, nil
	}

	var configResp ConfigurationResponse
	if err := json.Unmarshal(responseBody, &configResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &configResp, nil
}

func (c *ConfigurationClient) updateMetrics(fn func(*ConfigMetrics)) {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()
	fn(c.metrics)
}

func (c *ConfigurationClient) startCacheCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cache.Cleanup()
		}
	}
}

// Cache methods
func (cc *ConfigCache) Get(key string) (interface{}, bool) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	item, exists := cc.items[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(item.ExpiresAt) {
		// Item expired, remove it
		delete(cc.items, key)
		return nil, false
	}

	return item.Value, true
}

func (cc *ConfigCache) Set(key string, value interface{}) {
	cc.SetWithTTL(key, value, cc.ttl)
}

func (cc *ConfigCache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.items[key] = &CacheItem{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
		CreatedAt: time.Now(),
	}
}

func (cc *ConfigCache) Delete(key string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	delete(cc.items, key)
}

func (cc *ConfigCache) Clear() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.items = make(map[string]*CacheItem)
}

func (cc *ConfigCache) Size() int {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return len(cc.items)
}

func (cc *ConfigCache) Cleanup() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	now := time.Now()
	expired := 0

	for key, item := range cc.items {
		if now.After(item.ExpiresAt) {
			delete(cc.items, key)
			expired++
		}
	}

	if expired > 0 {
		cc.logger.WithField("expired_items", expired).Debug("Cache cleanup completed")
	}
}