package infrastructure

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/config"
)

func setupInterServiceClientManager() (*InterServiceClientManager, *ServiceDiscovery, *ConfigurationClient, func()) {
	cfg := &config.Config{
		ServiceName:    "market-data-simulator",
		ServiceVersion: "1.0.0",
		GRPCPort:      9090,
		HTTPPort:      8080,
		RedisURL:      "redis://localhost:6379",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce log noise in tests

	// Create mock service discovery
	serviceDiscovery := NewServiceDiscovery(cfg, logger)

	// Create mock configuration client
	configClient := NewConfigurationClient(cfg, logger)

	// Create inter-service client manager
	clientManager := NewInterServiceClientManager(cfg, logger, serviceDiscovery, configClient)

	cleanup := func() {
		clientManager.Close()
		serviceDiscovery.Close()
	}

	return clientManager, serviceDiscovery, configClient, cleanup
}

func TestInterServiceClientManager_Creation(t *testing.T) {
	cm, _, _, cleanup := setupInterServiceClientManager()
	defer cleanup()

	assert.NotNil(t, cm)
	assert.NotNil(t, cm.config)
	assert.NotNil(t, cm.logger)
	assert.NotNil(t, cm.serviceDiscovery)
	assert.NotNil(t, cm.configClient)
	assert.NotNil(t, cm.clients)
	assert.NotNil(t, cm.connections)
	assert.NotNil(t, cm.metrics)
}

func TestInterServiceClientManager_GetMetrics(t *testing.T) {
	cm, _, _, cleanup := setupInterServiceClientManager()
	defer cleanup()

	metrics := cm.GetMetrics()

	assert.Contains(t, metrics, "active_connections")
	assert.Contains(t, metrics, "total_requests")
	assert.Contains(t, metrics, "successful_requests")
	assert.Contains(t, metrics, "failed_requests")
	assert.Contains(t, metrics, "circuit_open_count")
	assert.Contains(t, metrics, "connection_errors")
	assert.Contains(t, metrics, "pool_size")

	assert.Equal(t, int64(0), metrics["active_connections"])
	assert.Equal(t, int64(0), metrics["total_requests"])
	assert.Equal(t, int64(0), metrics["successful_requests"])
	assert.Equal(t, int64(0), metrics["failed_requests"])
	assert.Equal(t, int64(0), metrics["circuit_open_count"])
	assert.Equal(t, int64(0), metrics["connection_errors"])
	assert.Equal(t, int64(0), metrics["pool_size"])
}

func TestInterServiceClientManager_GetClient_ServiceUnavailable(t *testing.T) {
	cm, _, _, cleanup := setupInterServiceClientManager()
	defer cleanup()

	ctx := context.Background()

	// Try to get client for non-existent service
	client, err := cm.GetClient(ctx, "non-existent-service", "grpc")

	assert.Error(t, err)
	assert.Nil(t, client)

	// Verify error is tracked in metrics
	metrics := cm.GetMetrics()
	assert.Greater(t, metrics["connection_errors"].(int64), int64(0))
}

func TestInterServiceClientManager_GetClientMetrics_NotFound(t *testing.T) {
	cm, _, _, cleanup := setupInterServiceClientManager()
	defer cleanup()

	metrics := cm.GetClientMetrics("non-existent", "grpc")

	assert.Contains(t, metrics, "error")
	assert.Equal(t, "client not found", metrics["error"])
}

func TestInterServiceClientManager_SpecificClientMethods(t *testing.T) {
	cm, _, _, cleanup := setupInterServiceClientManager()
	defer cleanup()

	ctx := context.Background()

	// Test all specific client methods (they should all fail without services)
	testCases := []struct {
		name   string
		method func(context.Context) (*ServiceClient, error)
	}{
		{"GetRiskMonitorClient", cm.GetRiskMonitorClient},
		{"GetAuditCorrelatorClient", cm.GetAuditCorrelatorClient},
		{"GetExchangeSimulatorClient", cm.GetExchangeSimulatorClient},
		{"GetTradingEngineClient", cm.GetTradingEngineClient},
		{"GetTestCoordinatorClient", cm.GetTestCoordinatorClient},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := tc.method(ctx)
			assert.Error(t, err)
			assert.Nil(t, client)
		})
	}
}

func TestInterServiceClientManager_CleanupIdleConnections(t *testing.T) {
	cm, _, _, cleanup := setupInterServiceClientManager()
	defer cleanup()

	// No connections to cleanup initially
	cm.CleanupIdleConnections()

	metrics := cm.GetMetrics()
	assert.Equal(t, int64(0), metrics["active_connections"])
}

func TestInterServiceClientManager_PerformHealthChecks(t *testing.T) {
	cm, _, _, cleanup := setupInterServiceClientManager()
	defer cleanup()

	ctx := context.Background()

	// No clients to check initially
	cm.PerformHealthChecks(ctx)

	// Should complete without error
	assert.True(t, true)
}

func TestInterServiceClientManager_Close(t *testing.T) {
	cm, _, _, cleanup := setupInterServiceClientManager()
	defer cleanup()

	err := cm.Close()
	assert.NoError(t, err)

	// Verify connections are cleared
	assert.Empty(t, cm.clients)
	assert.Empty(t, cm.connections)
}

// ServiceClient tests
func TestServiceClient_Methods(t *testing.T) {
	client := &ServiceClient{
		serviceName: "test-service",
		serviceType: "grpc",
		isHealthy:   true,
		metrics: &ServiceClientMetrics{
			requestCount: 0,
			successCount: 0,
			errorCount:   0,
		},
		circuitBreaker: &CircuitBreaker{
			state:     CircuitClosed,
			threshold: 5,
			timeout:   30 * time.Second,
		},
	}

	assert.Equal(t, "test-service", client.GetServiceName())
	assert.Equal(t, "grpc", client.GetServiceType())
	assert.True(t, client.IsHealthy())
	assert.Nil(t, client.GetConnection()) // No connection set in test
}

func TestServiceClient_RecordRequest(t *testing.T) {
	client := &ServiceClient{
		serviceName: "test-service",
		serviceType: "grpc",
		metrics: &ServiceClientMetrics{
			requestCount: 0,
			successCount: 0,
			errorCount:   0,
		},
		circuitBreaker: &CircuitBreaker{
			state:     CircuitClosed,
			threshold: 5,
			timeout:   30 * time.Second,
		},
	}

	// Record successful request
	client.RecordRequest(100*time.Millisecond, true)

	client.metrics.mu.RLock()
	assert.Equal(t, int64(1), client.metrics.requestCount)
	assert.Equal(t, int64(1), client.metrics.successCount)
	assert.Equal(t, int64(0), client.metrics.errorCount)
	assert.Equal(t, 100*time.Millisecond, client.metrics.avgResponseTime)
	assert.False(t, client.metrics.lastRequestTime.IsZero())
	client.metrics.mu.RUnlock()

	// Record failed request
	client.RecordRequest(200*time.Millisecond, false)

	client.metrics.mu.RLock()
	assert.Equal(t, int64(2), client.metrics.requestCount)
	assert.Equal(t, int64(1), client.metrics.successCount)
	assert.Equal(t, int64(1), client.metrics.errorCount)
	assert.Equal(t, 150*time.Millisecond, client.metrics.avgResponseTime) // (100+200)/2
	client.metrics.mu.RUnlock()
}

// CircuitBreaker tests
func TestCircuitBreaker_States(t *testing.T) {
	cb := &CircuitBreaker{
		state:     CircuitClosed,
		threshold: 3,
		timeout:   1 * time.Second,
	}

	// Initially closed
	assert.Equal(t, CircuitClosed, cb.GetState())
	assert.Equal(t, "closed", cb.GetStateString())

	// Record failures to open circuit
	cb.recordFailure()
	assert.Equal(t, CircuitClosed, cb.GetState())

	cb.recordFailure()
	assert.Equal(t, CircuitClosed, cb.GetState())

	cb.recordFailure()
	assert.Equal(t, CircuitOpen, cb.GetState())
	assert.Equal(t, "open", cb.GetStateString())

	// Wait for timeout and check half-open
	time.Sleep(1100 * time.Millisecond)
	assert.Equal(t, CircuitHalfOpen, cb.GetState())
	assert.Equal(t, "half-open", cb.GetStateString())

	// Record success to close circuit
	cb.recordSuccess()
	assert.Equal(t, CircuitClosed, cb.GetState())
}

func TestCircuitBreaker_HalfOpenToOpen(t *testing.T) {
	cb := &CircuitBreaker{
		state:     CircuitHalfOpen,
		threshold: 3,
		timeout:   1 * time.Second,
	}

	// Failure in half-open state should open circuit
	cb.recordFailure()
	assert.Equal(t, CircuitOpen, cb.GetState())
}

func TestCircuitBreaker_GetMetrics(t *testing.T) {
	cb := &CircuitBreaker{
		state:        CircuitClosed,
		failureCount: 2,
		successCount: 5,
		threshold:    3,
		timeout:      30 * time.Second,
		lastFailTime: time.Now().Add(-1 * time.Minute),
		lastSuccTime: time.Now(),
	}

	metrics := cb.GetMetrics()

	assert.Contains(t, metrics, "state")
	assert.Contains(t, metrics, "failure_count")
	assert.Contains(t, metrics, "success_count")
	assert.Contains(t, metrics, "threshold")
	assert.Contains(t, metrics, "timeout")
	assert.Contains(t, metrics, "last_failure")
	assert.Contains(t, metrics, "last_success")

	assert.Equal(t, "closed", metrics["state"])
	assert.Equal(t, int64(2), metrics["failure_count"])
	assert.Equal(t, int64(5), metrics["success_count"])
	assert.Equal(t, int64(3), metrics["threshold"])
	assert.Equal(t, 30.0, metrics["timeout"])
}

// ServiceUnavailableError tests
func TestServiceUnavailableError(t *testing.T) {
	err := &ServiceUnavailableError{
		ServiceName: "test-service",
		Reason:      "no healthy instances",
	}

	expectedMsg := "service test-service is unavailable: no healthy instances"
	assert.Equal(t, expectedMsg, err.Error())
}

// Integration-style tests (without actual services)
func TestInterServiceClientManager_ConnectionLifecycle(t *testing.T) {
	cm, _, _, cleanup := setupInterServiceClientManager()
	defer cleanup()

	ctx := context.Background()

	// Initially no connections
	metrics := cm.GetMetrics()
	assert.Equal(t, int64(0), metrics["active_connections"])

	// Try to get a client (will fail but should track error)
	_, err := cm.GetClient(ctx, "test-service", "grpc")
	assert.Error(t, err)

	// Verify error is tracked
	metrics = cm.GetMetrics()
	assert.Greater(t, metrics["connection_errors"].(int64), int64(0))

	// Cleanup should work even with failed connections
	err = cm.Close()
	assert.NoError(t, err)
}

func TestInterServiceClientManager_ConcurrentAccess(t *testing.T) {
	cm, _, _, cleanup := setupInterServiceClientManager()
	defer cleanup()

	ctx := context.Background()

	// Test concurrent access to GetClient (should handle race conditions)
	done := make(chan bool, 5)

	for i := 0; i < 5; i++ {
		go func() {
			_, err := cm.GetClient(ctx, "test-service", "grpc")
			assert.Error(t, err) // Expected to fail without service
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 5; i++ {
		select {
		case <-done:
			// Continue
		case <-time.After(5 * time.Second):
			t.Fatal("Goroutine timed out")
		}
	}

	// Verify error count
	metrics := cm.GetMetrics()
	assert.Equal(t, int64(5), metrics["connection_errors"])
}

func TestInterServiceClientManager_MetricsUpdateConcurrency(t *testing.T) {
	cm, _, _, cleanup := setupInterServiceClientManager()
	defer cleanup()

	// Test concurrent metrics updates
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			cm.updateMetrics(func(m *ClientManagerMetrics) {
				m.totalRequests++
			})
			done <- true
		}()
	}

	// Wait for all updates
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// Continue
		case <-time.After(2 * time.Second):
			t.Fatal("Metrics update timed out")
		}
	}

	metrics := cm.GetMetrics()
	assert.Equal(t, int64(10), metrics["total_requests"])
}