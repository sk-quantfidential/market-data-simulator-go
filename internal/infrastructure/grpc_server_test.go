package infrastructure

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"

	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/config"
	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/services"
)

const bufSize = 1024 * 1024

func setupTestServer(t *testing.T) (*MarketDataGRPCServer, *bufconn.Listener, func()) {
	cfg := &config.Config{
		ServiceName:    "market-data-simulator",
		ServiceVersion: "1.0.0",
		GRPCPort:      9090,
		HTTPPort:      8080,
		LogLevel:      "info",
		RedisURL:      "redis://localhost:6379",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce log noise in tests

	marketDataService := services.NewMarketDataService(cfg, logger)
	server := NewMarketDataGRPCServer(cfg, marketDataService, logger)

	lis := bufconn.Listen(bufSize)

	go func() {
		if err := server.grpcServer.Serve(lis); err != nil {
			// Server stopped
		}
	}()

	cleanup := func() {
		server.Stop()
		lis.Close()
	}

	return server, lis, cleanup
}

func bufDialer(lis *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, url string) (net.Conn, error) {
		return lis.Dial()
	}
}

func TestMarketDataGRPCServer_Creation(t *testing.T) {
	cfg := &config.Config{
		ServiceName:    "market-data-simulator",
		ServiceVersion: "1.0.0",
		GRPCPort:      9090,
	}

	logger := logrus.New()
	marketDataService := services.NewMarketDataService(cfg, logger)
	server := NewMarketDataGRPCServer(cfg, marketDataService, logger)

	assert.NotNil(t, server)
	assert.Equal(t, cfg, server.config)
	assert.Equal(t, logger, server.logger)
	assert.Equal(t, marketDataService, server.marketDataService)
	assert.NotNil(t, server.grpcServer)
	assert.NotNil(t, server.healthServer)
	assert.NotNil(t, server.metrics)
}

func TestMarketDataGRPCServer_HealthService(t *testing.T) {
	_, lis, cleanup := setupTestServer(t)
	defer cleanup()

	// Create client connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer(lis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	require.NoError(t, err)
	defer conn.Close()

	// Test health service
	healthClient := grpc_health_v1.NewHealthClient(conn)

	// Test general health
	resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{
		Service: "",
	})
	require.NoError(t, err)
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.Status)

	// Test market-data service health
	resp, err = healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{
		Service: "market-data",
	})
	require.NoError(t, err)
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.Status)
}

func TestMarketDataGRPCServer_Metrics(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Get initial metrics
	metrics := server.GetMetrics()

	// Verify metrics structure
	assert.Contains(t, metrics, "uptime_seconds")
	assert.Contains(t, metrics, "request_count")
	assert.Contains(t, metrics, "connection_count")
	assert.Contains(t, metrics, "streaming_clients")
	assert.Contains(t, metrics, "avg_response_time_ms")
	assert.Contains(t, metrics, "service_name")
	assert.Contains(t, metrics, "service_version")
	assert.Contains(t, metrics, "health_status")

	// Verify initial values
	assert.Equal(t, "market-data-simulator", metrics["service_name"])
	assert.Equal(t, "1.0.0", metrics["service_version"])
	assert.Equal(t, "SERVING", metrics["health_status"])
	assert.Equal(t, int64(0), metrics["request_count"])
	assert.Equal(t, int64(0), metrics["connection_count"])
	assert.Equal(t, int64(0), metrics["streaming_clients"])

	// Verify uptime is reasonable
	uptime := metrics["uptime_seconds"].(float64)
	assert.Greater(t, uptime, 0.0)
	assert.Less(t, uptime, 10.0) // Should be less than 10 seconds in test
}

func TestMarketDataGRPCServer_UnaryInterceptor(t *testing.T) {
	server, lis, cleanup := setupTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer(lis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	require.NoError(t, err)
	defer conn.Close()

	// Make a health check request to trigger interceptor
	healthClient := grpc_health_v1.NewHealthClient(conn)
	_, err = healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)

	// Check that metrics were updated
	metrics := server.GetMetrics()
	assert.Equal(t, int64(1), metrics["request_count"])

	// Make another request
	_, err = healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)

	// Check that request count increased
	metrics = server.GetMetrics()
	assert.Equal(t, int64(2), metrics["request_count"])
}

func TestMarketDataGRPCServer_Stop(t *testing.T) {
	server, lis, _ := setupTestServer(t)

	// Verify server is initially serving
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer(lis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	require.NoError(t, err)

	healthClient := grpc_health_v1.NewHealthClient(conn)
	resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.Status)

	// Stop the server
	server.Stop()

	// Try to make another request (should fail or return NOT_SERVING)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel2()

	// The health check might return NOT_SERVING or the connection might fail
	resp, err = healthClient.Check(ctx2, &grpc_health_v1.HealthCheckRequest{})
	if err == nil {
		// If no error, status should be NOT_SERVING
		assert.Equal(t, grpc_health_v1.HealthCheckResponse_NOT_SERVING, resp.Status)
	}
	// If there's an error, that's expected after graceful stop

	conn.Close()
	lis.Close()
}

func TestMarketDataGRPCServer_ConcurrentRequests(t *testing.T) {
	server, lis, cleanup := setupTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer(lis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	require.NoError(t, err)
	defer conn.Close()

	healthClient := grpc_health_v1.NewHealthClient(conn)

	// Make concurrent requests
	const numRequests = 10
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			_, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
			assert.NoError(t, err)
			done <- true
		}()
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		select {
		case <-done:
			// Request completed
		case <-time.After(5 * time.Second):
			t.Fatal("Request timed out")
		}
	}

	// Verify metrics reflect all requests
	metrics := server.GetMetrics()
	assert.Equal(t, int64(numRequests), metrics["request_count"])
}

func TestMarketDataGRPCServer_ResponseTimeTracking(t *testing.T) {
	server, lis, cleanup := setupTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer(lis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	require.NoError(t, err)
	defer conn.Close()

	healthClient := grpc_health_v1.NewHealthClient(conn)

	// Make a request
	_, err = healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)

	// Check that response time was recorded
	metrics := server.GetMetrics()
	avgResponseTime := metrics["avg_response_time_ms"].(float64)
	assert.Greater(t, avgResponseTime, 0.0)
	assert.Less(t, avgResponseTime, 1000.0) // Should be less than 1 second
}