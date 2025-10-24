package infrastructure

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/config"
)

func setupConfigurationClient() (*ConfigurationClient, *httptest.Server) {
	cfg := &config.Config{
		ServiceName:    "market-data-simulator",
		ServiceVersion: "1.0.0",
		GRPCPort:       50051,
		HTTPPort:       8080,
	}

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce log noise in tests

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/configuration":
			handleConfigurationRequest(w, r)
		case "/api/v1/health":
			handleHealthRequest(w, r)
		default:
			http.NotFound(w, r)
		}
	}))

	client := NewConfigurationClient(cfg, logger)
	client.baseURL = server.URL

	return client, server
}

func handleConfigurationRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// Mock configuration response
		resp := ConfigurationResponse{
			Key:         "test.key",
			Value:       "test.value",
			Environment: "development",
			Service:     "market-data-simulator",
			Version:     "1.0.0",
			UpdatedAt:   time.Now(),
			TTL:         300, // 5 minutes
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

	case "POST":
		// Mock configuration update
		var req ConfigurationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		resp := ConfigurationResponse{
			Key:         req.Key,
			Value:       req.Value,
			Environment: req.Environment,
			Service:     req.Service,
			Version:     req.Version,
			UpdatedAt:   time.Now(),
			TTL:         300,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleHealthRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func TestConfigurationClient_Creation(t *testing.T) {
	client, server := setupConfigurationClient()
	defer server.Close()

	assert.NotNil(t, client)
	assert.NotNil(t, client.config)
	assert.NotNil(t, client.logger)
	assert.NotNil(t, client.httpClient)
	assert.NotNil(t, client.cache)
	assert.NotNil(t, client.metrics)
	assert.Equal(t, server.URL, client.baseURL)
}

func TestConfigurationClient_GetConfiguration(t *testing.T) {
	client, server := setupConfigurationClient()
	defer server.Close()

	ctx := context.Background()
	key := "test.key"

	value, err := client.GetConfiguration(ctx, key)

	require.NoError(t, err)
	assert.Equal(t, "test.value", value)

	// Verify metrics
	metrics := client.GetMetrics()
	assert.Equal(t, int64(1), metrics["request_count"])
	assert.Equal(t, int64(0), metrics["cache_hits"])
	assert.Equal(t, int64(1), metrics["cache_misses"])
	assert.Equal(t, "healthy", metrics["connection_status"])
}

func TestConfigurationClient_GetConfiguration_CacheHit(t *testing.T) {
	client, server := setupConfigurationClient()
	defer server.Close()

	ctx := context.Background()
	key := "test.key"

	// First request - should hit the service
	value1, err := client.GetConfiguration(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, "test.value", value1)

	// Second request - should hit the cache
	value2, err := client.GetConfiguration(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, "test.value", value2)

	// Verify metrics
	metrics := client.GetMetrics()
	assert.Equal(t, int64(2), metrics["request_count"])
	assert.Equal(t, int64(1), metrics["cache_hits"])
	assert.Equal(t, int64(1), metrics["cache_misses"])
	assert.Greater(t, metrics["cache_hit_rate"], 0.0)
}

func TestConfigurationClient_SetConfiguration(t *testing.T) {
	client, server := setupConfigurationClient()
	defer server.Close()

	ctx := context.Background()
	key := "test.key"
	value := "new.value"

	err := client.SetConfiguration(ctx, key, value)

	require.NoError(t, err)

	// Verify metrics
	metrics := client.GetMetrics()
	assert.Equal(t, int64(1), metrics["request_count"])
	assert.Equal(t, "healthy", metrics["connection_status"])
}

func TestConfigurationClient_SetConfiguration_InvalidatesCache(t *testing.T) {
	client, server := setupConfigurationClient()
	defer server.Close()

	ctx := context.Background()
	key := "test.key"

	// First, get the configuration to cache it
	_, err := client.GetConfiguration(ctx, key)
	require.NoError(t, err)

	// Verify it's in cache
	assert.Equal(t, 1, client.cache.Size())

	// Now set a new value, which should invalidate the cache
	err = client.SetConfiguration(ctx, key, "new.value")
	require.NoError(t, err)

	// Cache should be empty for this key
	_, found := client.cache.Get(key)
	assert.False(t, found)
}

func TestConfigurationClient_HealthCheck(t *testing.T) {
	client, server := setupConfigurationClient()
	defer server.Close()

	ctx := context.Background()

	err := client.HealthCheck(ctx)

	require.NoError(t, err)
}

func TestConfigurationClient_HealthCheck_ServiceUnavailable(t *testing.T) {
	client, server := setupConfigurationClient()
	server.Close() // Close server to simulate unavailable service

	client.baseURL = "http://localhost:99999" // Non-existent service

	ctx := context.Background()

	err := client.HealthCheck(ctx)

	assert.Error(t, err)

	// Verify error is recorded in metrics
	metrics := client.GetMetrics()
	assert.Greater(t, metrics["error_count"].(int64), int64(0))
}

func TestConfigurationClient_GetMetrics(t *testing.T) {
	client, server := setupConfigurationClient()
	defer server.Close()

	// Make some requests to generate metrics
	ctx := context.Background()
	_, _ = client.GetConfiguration(ctx, "key1")
	_, _ = client.GetConfiguration(ctx, "key1") // Cache hit
	_, _ = client.GetConfiguration(ctx, "key2") // Cache miss

	metrics := client.GetMetrics()

	assert.Contains(t, metrics, "request_count")
	assert.Contains(t, metrics, "cache_hits")
	assert.Contains(t, metrics, "cache_misses")
	assert.Contains(t, metrics, "cache_hit_rate")
	assert.Contains(t, metrics, "cache_size")
	assert.Contains(t, metrics, "avg_response_time_ms")
	assert.Contains(t, metrics, "connection_status")
	assert.Contains(t, metrics, "last_request_time")
	assert.Contains(t, metrics, "error_count")

	assert.Equal(t, int64(3), metrics["request_count"])
	assert.Equal(t, int64(1), metrics["cache_hits"])
	assert.Equal(t, int64(2), metrics["cache_misses"])
	assert.InDelta(t, 33.33, metrics["cache_hit_rate"], 1.0) // 1/3 = 33.33%
}

func TestConfigurationClient_InvalidateCache(t *testing.T) {
	client, server := setupConfigurationClient()
	defer server.Close()

	ctx := context.Background()
	key := "test.key"

	// Get configuration to cache it
	_, err := client.GetConfiguration(ctx, key)
	require.NoError(t, err)

	// Verify it's cached
	_, found := client.cache.Get(key)
	assert.True(t, found)

	// Invalidate cache
	client.InvalidateCache(key)

	// Verify it's no longer cached
	_, found = client.cache.Get(key)
	assert.False(t, found)
}

func TestConfigurationClient_ClearCache(t *testing.T) {
	client, server := setupConfigurationClient()
	defer server.Close()

	ctx := context.Background()

	// Get multiple configurations to cache them
	_, _ = client.GetConfiguration(ctx, "key1")
	_, _ = client.GetConfiguration(ctx, "key2")

	// Verify cache has items
	assert.Greater(t, client.cache.Size(), 0)

	// Clear cache
	client.ClearCache()

	// Verify cache is empty
	assert.Equal(t, 0, client.cache.Size())
}

func TestConfigCache_SetAndGet(t *testing.T) {
	logger := logrus.New()
	cache := &ConfigCache{
		items:  make(map[string]*CacheItem),
		ttl:    5 * time.Minute,
		logger: logger,
	}

	key := "test.key"
	value := "test.value"

	// Set and get
	cache.Set(key, value)
	retrievedValue, found := cache.Get(key)

	assert.True(t, found)
	assert.Equal(t, value, retrievedValue)
}

func TestConfigCache_TTLExpiration(t *testing.T) {
	logger := logrus.New()
	cache := &ConfigCache{
		items:  make(map[string]*CacheItem),
		ttl:    5 * time.Minute,
		logger: logger,
	}

	key := "test.key"
	value := "test.value"

	// Set with short TTL
	cache.SetWithTTL(key, value, 1*time.Millisecond)

	// Wait for expiration
	time.Sleep(2 * time.Millisecond)

	// Should not be found
	_, found := cache.Get(key)
	assert.False(t, found)
}

func TestConfigCache_Delete(t *testing.T) {
	logger := logrus.New()
	cache := &ConfigCache{
		items:  make(map[string]*CacheItem),
		ttl:    5 * time.Minute,
		logger: logger,
	}

	key := "test.key"
	value := "test.value"

	cache.Set(key, value)
	cache.Delete(key)

	_, found := cache.Get(key)
	assert.False(t, found)
}

func TestConfigCache_Clear(t *testing.T) {
	logger := logrus.New()
	cache := &ConfigCache{
		items:  make(map[string]*CacheItem),
		ttl:    5 * time.Minute,
		logger: logger,
	}

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	assert.Equal(t, 2, cache.Size())

	cache.Clear()

	assert.Equal(t, 0, cache.Size())
}

func TestConfigCache_Cleanup(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce log noise
	cache := &ConfigCache{
		items:  make(map[string]*CacheItem),
		ttl:    5 * time.Minute,
		logger: logger,
	}

	// Add items with different TTLs
	cache.SetWithTTL("expired", "value1", 1*time.Millisecond)
	cache.SetWithTTL("valid", "value2", 5*time.Minute)

	// Wait for first item to expire
	time.Sleep(2 * time.Millisecond)

	// Run cleanup
	cache.Cleanup()

	// Only valid item should remain
	assert.Equal(t, 1, cache.Size())
	_, found := cache.Get("valid")
	assert.True(t, found)
	_, found = cache.Get("expired")
	assert.False(t, found)
}

func TestConfigurationClient_ErrorHandling(t *testing.T) {
	cfg := &config.Config{
		ServiceName:    "market-data-simulator",
		ServiceVersion: "1.0.0",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	// Create server that returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewConfigurationClient(cfg, logger)
	client.baseURL = server.URL

	ctx := context.Background()

	// Should get error
	_, err := client.GetConfiguration(ctx, "test.key")
	assert.Error(t, err)

	// Error should be recorded in metrics
	metrics := client.GetMetrics()
	assert.Equal(t, "error", metrics["connection_status"])
	assert.Greater(t, metrics["error_count"].(int64), int64(0))
}
