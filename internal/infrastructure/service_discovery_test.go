package infrastructure

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/config"
)

func setupServiceDiscovery() (*ServiceDiscovery, func()) {
	cfg := &config.Config{
		ServiceName:    "market-data-simulator",
		ServiceVersion: "1.0.0",
		GRPCPort:       50051,
		HTTPPort:       8080,
		RedisURL:       "redis://localhost:6379",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce log noise in tests

	sd := NewServiceDiscovery(cfg, logger)

	// Check if Redis is available for testing
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := sd.testConnection(ctx)
	if err != nil {
		// Use mock Redis client for testing when Redis is not available
		sd.redisClient = setupMockRedisClient()
	}

	cleanup := func() {
		if sd.IsRegistered() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			sd.Deregister(ctx)
		}
		sd.Close()
	}

	return sd, cleanup
}

func setupMockRedisClient() *redis.Client {
	// For unit tests without Redis, we'll use a minimal mock
	// In a real implementation, you might use a more sophisticated mock
	return redis.NewClient(&redis.Options{
		Addr: "localhost:9999", // Non-existent address for mock
	})
}

func TestServiceDiscovery_Creation(t *testing.T) {
	sd, cleanup := setupServiceDiscovery()
	defer cleanup()

	assert.NotNil(t, sd)
	assert.NotNil(t, sd.config)
	assert.NotNil(t, sd.logger)
	assert.NotNil(t, sd.redisClient)
	assert.NotNil(t, sd.registration)
	assert.NotNil(t, sd.metrics)
	assert.False(t, sd.IsRegistered())

	// Verify registration structure
	reg := sd.GetRegistration()
	assert.Equal(t, "market-data-simulator", reg.ServiceName)
	assert.Equal(t, "1.0.0", reg.ServiceVersion)
	assert.NotEmpty(t, reg.InstanceID)
	assert.Equal(t, 8080, reg.HTTPPort)
	assert.Equal(t, 50051, reg.GRPCPort)
	assert.Equal(t, "healthy", reg.Health)
	assert.Equal(t, "active", reg.Status)
	assert.Contains(t, reg.Tags, "market-data")
	assert.Contains(t, reg.Tags, "simulator")
	assert.Contains(t, reg.Tags, "grpc")
	assert.Contains(t, reg.Tags, "http")
}

func TestServiceDiscovery_Registration_WithoutRedis(t *testing.T) {
	sd, cleanup := setupServiceDiscovery()
	defer cleanup()

	// Use mock client that will fail
	sd.redisClient = setupMockRedisClient()

	ctx := context.Background()

	// Registration should fail without Redis
	err := sd.Register(ctx)
	assert.Error(t, err)
	assert.False(t, sd.IsRegistered())

	// Verify error is tracked in metrics
	metrics := sd.GetMetrics()
	assert.Greater(t, metrics["error_count"].(int64), int64(0))
	assert.Equal(t, "error", metrics["connection_status"])
}

func TestServiceDiscovery_GetMetrics(t *testing.T) {
	sd, cleanup := setupServiceDiscovery()
	defer cleanup()

	metrics := sd.GetMetrics()

	assert.Contains(t, metrics, "registration_count")
	assert.Contains(t, metrics, "deregistration_count")
	assert.Contains(t, metrics, "heartbeat_count")
	assert.Contains(t, metrics, "discovery_request_count")
	assert.Contains(t, metrics, "healthy_services")
	assert.Contains(t, metrics, "unhealthy_services")
	assert.Contains(t, metrics, "connection_status")
	assert.Contains(t, metrics, "last_heartbeat")
	assert.Contains(t, metrics, "error_count")
	assert.Contains(t, metrics, "is_registered")
	assert.Contains(t, metrics, "instance_id")
	assert.Contains(t, metrics, "service_name")

	assert.Equal(t, int64(0), metrics["registration_count"])
	assert.Equal(t, int64(0), metrics["deregistration_count"])
	assert.Equal(t, int64(0), metrics["heartbeat_count"])
	assert.Equal(t, int64(0), metrics["discovery_request_count"])
	assert.Equal(t, false, metrics["is_registered"])
	assert.Equal(t, "market-data-simulator", metrics["service_name"])
}

func TestServiceDiscovery_UpdateHealth(t *testing.T) {
	sd, cleanup := setupServiceDiscovery()
	defer cleanup()

	ctx := context.Background()

	// Should fail when not registered
	err := sd.UpdateHealth(ctx, "unhealthy")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")

	// Test with mock Redis client to simulate failure
	sd.redisClient = setupMockRedisClient()
	sd.isRegistered = true // Force registered state for testing

	err = sd.UpdateHealth(ctx, "healthy")
	assert.Error(t, err) // Should fail with mock client
}

func TestServiceDiscovery_DiscoverService_WithoutRedis(t *testing.T) {
	sd, cleanup := setupServiceDiscovery()
	defer cleanup()

	// Use mock client that will fail
	sd.redisClient = setupMockRedisClient()

	ctx := context.Background()

	services, err := sd.DiscoverService(ctx, "test-service")
	assert.Error(t, err)
	assert.Nil(t, services)

	// Verify metrics are updated
	metrics := sd.GetMetrics()
	assert.Greater(t, metrics["discovery_request_count"].(int64), int64(0))
	assert.Greater(t, metrics["error_count"].(int64), int64(0))
}

func TestServiceDiscovery_GetHealthyInstances_WithoutRedis(t *testing.T) {
	sd, cleanup := setupServiceDiscovery()
	defer cleanup()

	// Use mock client that will fail
	sd.redisClient = setupMockRedisClient()

	ctx := context.Background()

	services, err := sd.GetHealthyInstances(ctx, "test-service")
	assert.Error(t, err)
	assert.Nil(t, services)
}

func TestServiceDiscovery_ServiceKeys(t *testing.T) {
	sd, cleanup := setupServiceDiscovery()
	defer cleanup()

	serviceKey := sd.getServiceKey()
	assert.Contains(t, serviceKey, "services:market-data-simulator:")
	assert.Contains(t, serviceKey, sd.registration.InstanceID)

	listKey := sd.getServiceListKey()
	assert.Equal(t, "service_list:market-data-simulator", listKey)
}

func TestServiceDiscovery_Deregister_NotRegistered(t *testing.T) {
	sd, cleanup := setupServiceDiscovery()
	defer cleanup()

	ctx := context.Background()

	err := sd.Deregister(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

func TestServiceDiscovery_CleanupStaleServices_WithoutRedis(t *testing.T) {
	sd, cleanup := setupServiceDiscovery()
	defer cleanup()

	// Use mock client that will fail
	sd.redisClient = setupMockRedisClient()

	ctx := context.Background()

	err := sd.CleanupStaleServices(ctx)
	assert.Error(t, err)
}

func TestServiceDiscovery_Close(t *testing.T) {
	sd, _ := setupServiceDiscovery()

	err := sd.Close()
	assert.NoError(t, err)
}

// Utility function tests
func TestFilterServicesByTag(t *testing.T) {
	services := []*ServiceInfo{
		{
			ServiceName: "service1",
			Tags:        []string{"tag1", "tag2"},
		},
		{
			ServiceName: "service2",
			Tags:        []string{"tag2", "tag3"},
		},
		{
			ServiceName: "service3",
			Tags:        []string{"tag1"},
		},
	}

	filtered := FilterServicesByTag(services, "tag1")
	assert.Len(t, filtered, 2)
	assert.Equal(t, "service1", filtered[0].ServiceName)
	assert.Equal(t, "service3", filtered[1].ServiceName)

	filtered = FilterServicesByTag(services, "tag2")
	assert.Len(t, filtered, 2)
	assert.Equal(t, "service1", filtered[0].ServiceName)
	assert.Equal(t, "service2", filtered[1].ServiceName)

	filtered = FilterServicesByTag(services, "nonexistent")
	assert.Len(t, filtered, 0)
}

func TestFilterServicesByMetadata(t *testing.T) {
	services := []*ServiceInfo{
		{
			ServiceName: "service1",
			Metadata:    map[string]string{"env": "dev", "region": "us-east"},
		},
		{
			ServiceName: "service2",
			Metadata:    map[string]string{"env": "prod", "region": "us-east"},
		},
		{
			ServiceName: "service3",
			Metadata:    map[string]string{"env": "dev", "region": "us-west"},
		},
	}

	filtered := FilterServicesByMetadata(services, "env", "dev")
	assert.Len(t, filtered, 2)
	assert.Equal(t, "service1", filtered[0].ServiceName)
	assert.Equal(t, "service3", filtered[1].ServiceName)

	filtered = FilterServicesByMetadata(services, "region", "us-east")
	assert.Len(t, filtered, 2)
	assert.Equal(t, "service1", filtered[0].ServiceName)
	assert.Equal(t, "service2", filtered[1].ServiceName)

	filtered = FilterServicesByMetadata(services, "nonexistent", "value")
	assert.Len(t, filtered, 0)
}

func TestGetServiceEndpoint(t *testing.T) {
	service := &ServiceInfo{
		Address:  "localhost",
		Port:     8080,
		GRPCPort: 50051,
		HTTPPort: 8080,
	}

	// Test HTTP endpoint
	endpoint := GetServiceEndpoint(service, "http")
	assert.Equal(t, "http://localhost:8080", endpoint)

	// Test gRPC endpoint
	endpoint = GetServiceEndpoint(service, "grpc")
	assert.Equal(t, "localhost:50051", endpoint)

	// Test default endpoint
	endpoint = GetServiceEndpoint(service, "tcp")
	assert.Equal(t, "localhost:8080", endpoint)

	// Test case insensitivity
	endpoint = GetServiceEndpoint(service, "HTTP")
	assert.Equal(t, "http://localhost:8080", endpoint)

	endpoint = GetServiceEndpoint(service, "GRPC")
	assert.Equal(t, "localhost:50051", endpoint)
}

// Integration tests (require Redis)
func TestServiceDiscovery_Registration_Integration(t *testing.T) {
	sd, cleanup := setupServiceDiscovery()
	defer cleanup()

	ctx := context.Background()

	// Test Redis connection first
	err := sd.testConnection(ctx)
	if err != nil {
		t.Skip("Redis not available for integration test")
	}

	// Register service
	err = sd.Register(ctx)
	require.NoError(t, err)
	assert.True(t, sd.IsRegistered())

	// Verify metrics
	metrics := sd.GetMetrics()
	assert.Equal(t, int64(1), metrics["registration_count"])
	assert.Equal(t, "healthy", metrics["connection_status"])

	// Try to register again (should fail)
	err = sd.Register(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")

	// Deregister
	err = sd.Deregister(ctx)
	require.NoError(t, err)
	assert.False(t, sd.IsRegistered())

	// Verify deregistration metrics
	metrics = sd.GetMetrics()
	assert.Equal(t, int64(1), metrics["deregistration_count"])
}

func TestServiceDiscovery_Discovery_Integration(t *testing.T) {
	sd1, cleanup1 := setupServiceDiscovery()
	defer cleanup1()

	sd2, cleanup2 := setupServiceDiscovery()
	defer cleanup2()

	ctx := context.Background()

	// Test Redis connection first
	err := sd1.testConnection(ctx)
	if err != nil {
		t.Skip("Redis not available for integration test")
	}

	// Register first service
	err = sd1.Register(ctx)
	require.NoError(t, err)

	// Register second service
	err = sd2.Register(ctx)
	require.NoError(t, err)

	// Wait a moment for registration to complete
	time.Sleep(100 * time.Millisecond)

	// Discover services
	services, err := sd1.DiscoverService(ctx, "market-data-simulator")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(services), 2)

	// Get healthy instances
	healthyServices, err := sd1.GetHealthyInstances(ctx, "market-data-simulator")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(healthyServices), 2)

	// Verify service details
	for _, service := range healthyServices {
		assert.Equal(t, "market-data-simulator", service.ServiceName)
		assert.Equal(t, "1.0.0", service.ServiceVersion)
		assert.Equal(t, "healthy", service.Health)
		assert.Equal(t, "active", service.Status)
	}
}

func TestServiceDiscovery_UpdateHealth_Integration(t *testing.T) {
	sd, cleanup := setupServiceDiscovery()
	defer cleanup()

	ctx := context.Background()

	// Test Redis connection first
	err := sd.testConnection(ctx)
	if err != nil {
		t.Skip("Redis not available for integration test")
	}

	// Register service
	err = sd.Register(ctx)
	require.NoError(t, err)

	// Update health
	err = sd.UpdateHealth(ctx, "unhealthy")
	require.NoError(t, err)

	reg := sd.GetRegistration()
	assert.Equal(t, "unhealthy", reg.Health)

	// Update back to healthy
	err = sd.UpdateHealth(ctx, "healthy")
	require.NoError(t, err)

	reg = sd.GetRegistration()
	assert.Equal(t, "healthy", reg.Health)
}

func TestServiceDiscovery_Heartbeat_Integration(t *testing.T) {
	sd, cleanup := setupServiceDiscovery()
	defer cleanup()

	ctx := context.Background()

	// Test Redis connection first
	err := sd.testConnection(ctx)
	if err != nil {
		t.Skip("Redis not available for integration test")
	}

	// Register service (starts heartbeat)
	err = sd.Register(ctx)
	require.NoError(t, err)

	// Wait for at least one heartbeat
	time.Sleep(16 * time.Second)

	// Check metrics
	metrics := sd.GetMetrics()
	assert.Greater(t, metrics["heartbeat_count"].(int64), int64(0))
	assert.False(t, metrics["last_heartbeat"].(time.Time).IsZero())
}
