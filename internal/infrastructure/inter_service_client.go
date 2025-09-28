package infrastructure

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/config"
)

type InterServiceClientManager struct {
	config           *config.Config
	logger           *logrus.Logger
	serviceDiscovery *ServiceDiscovery
	configClient     *ConfigurationClient
	clients          map[string]*ServiceClient
	connections      map[string]*grpc.ClientConn
	metrics          *ClientManagerMetrics
	mu               sync.RWMutex
}

type ServiceClient struct {
	serviceName    string
	serviceType    string
	connection     *grpc.ClientConn
	healthClient   grpc_health_v1.HealthClient
	circuitBreaker *CircuitBreaker
	metrics        *ServiceClientMetrics
	lastUsed       time.Time
	isHealthy      bool
	mu             sync.RWMutex
}

type CircuitBreaker struct {
	state         CircuitState
	failureCount  int64
	successCount  int64
	lastFailTime  time.Time
	lastSuccTime  time.Time
	threshold     int64
	timeout       time.Duration
	mu            sync.RWMutex
}

type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

type ClientManagerMetrics struct {
	mu                sync.RWMutex
	activeConnections int64
	totalRequests     int64
	successfulRequests int64
	failedRequests    int64
	circuitOpenCount  int64
	connectionErrors  int64
	poolSize          int64
}

type ServiceClientMetrics struct {
	mu                sync.RWMutex
	requestCount      int64
	successCount      int64
	errorCount        int64
	avgResponseTime   time.Duration
	lastRequestTime   time.Time
	connectionStatus  string
	circuitState      string
}

type ServiceUnavailableError struct {
	ServiceName string
	Reason      string
}

func (e *ServiceUnavailableError) Error() string {
	return fmt.Sprintf("service %s is unavailable: %s", e.ServiceName, e.Reason)
}

func NewInterServiceClientManager(cfg *config.Config, logger *logrus.Logger, serviceDiscovery *ServiceDiscovery, configClient *ConfigurationClient) *InterServiceClientManager {
	return &InterServiceClientManager{
		config:           cfg,
		logger:           logger,
		serviceDiscovery: serviceDiscovery,
		configClient:     configClient,
		clients:          make(map[string]*ServiceClient),
		connections:      make(map[string]*grpc.ClientConn),
		metrics: &ClientManagerMetrics{
			poolSize: 0,
		},
	}
}

func (cm *InterServiceClientManager) GetClient(ctx context.Context, serviceName, serviceType string) (*ServiceClient, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	clientKey := fmt.Sprintf("%s:%s", serviceName, serviceType)

	// Check if client already exists and is healthy
	if client, exists := cm.clients[clientKey]; exists {
		client.mu.Lock()
		client.lastUsed = time.Now()
		client.mu.Unlock()

		if client.isHealthy && client.circuitBreaker.GetState() != CircuitOpen {
			return client, nil
		}
	}

	// Create or recreate client
	client, err := cm.createClient(ctx, serviceName, serviceType)
	if err != nil {
		cm.updateMetrics(func(m *ClientManagerMetrics) {
			m.connectionErrors++
		})
		return nil, fmt.Errorf("failed to create client for %s: %w", serviceName, err)
	}

	cm.clients[clientKey] = client
	cm.updateMetrics(func(m *ClientManagerMetrics) {
		m.activeConnections++
		m.poolSize = int64(len(cm.clients))
	})

	cm.logger.WithFields(logrus.Fields{
		"service_name": serviceName,
		"service_type": serviceType,
		"client_key":   clientKey,
	}).Info("Created new service client")

	return client, nil
}

func (cm *InterServiceClientManager) GetRiskMonitorClient(ctx context.Context) (*ServiceClient, error) {
	return cm.GetClient(ctx, "risk-monitor", "grpc")
}

func (cm *InterServiceClientManager) GetAuditCorrelatorClient(ctx context.Context) (*ServiceClient, error) {
	return cm.GetClient(ctx, "audit-correlator", "grpc")
}

func (cm *InterServiceClientManager) GetExchangeSimulatorClient(ctx context.Context) (*ServiceClient, error) {
	return cm.GetClient(ctx, "exchange-simulator", "grpc")
}

func (cm *InterServiceClientManager) GetTradingEngineClient(ctx context.Context) (*ServiceClient, error) {
	return cm.GetClient(ctx, "trading-engine", "grpc")
}

func (cm *InterServiceClientManager) GetTestCoordinatorClient(ctx context.Context) (*ServiceClient, error) {
	return cm.GetClient(ctx, "test-coordinator", "grpc")
}

func (cm *InterServiceClientManager) createClient(ctx context.Context, serviceName, serviceType string) (*ServiceClient, error) {
	// Discover service instances
	services, err := cm.serviceDiscovery.GetHealthyInstances(ctx, serviceName)
	if err != nil {
		return nil, fmt.Errorf("service discovery failed: %w", err)
	}

	if len(services) == 0 {
		return nil, &ServiceUnavailableError{
			ServiceName: serviceName,
			Reason:      "no healthy instances found",
		}
	}

	// Select first healthy service (could implement load balancing here)
	service := services[0]
	endpoint := GetServiceEndpoint(service, serviceType)

	// Create gRPC connection
	conn, err := grpc.DialContext(ctx, endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(10*time.Second),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(4*1024*1024), // 4MB
			grpc.MaxCallSendMsgSize(4*1024*1024), // 4MB
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s at %s: %w", serviceName, endpoint, err)
	}

	// Create health client
	healthClient := grpc_health_v1.NewHealthClient(conn)

	// Test connection with health check
	healthResp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("health check failed for %s: %w", serviceName, err)
	}

	isHealthy := healthResp.Status == grpc_health_v1.HealthCheckResponse_SERVING

	// Create circuit breaker
	circuitBreaker := &CircuitBreaker{
		state:     CircuitClosed,
		threshold: 5, // Open circuit after 5 consecutive failures
		timeout:   30 * time.Second, // Try again after 30 seconds
	}

	// Create service client
	client := &ServiceClient{
		serviceName:    serviceName,
		serviceType:    serviceType,
		connection:     conn,
		healthClient:   healthClient,
		circuitBreaker: circuitBreaker,
		lastUsed:       time.Now(),
		isHealthy:      isHealthy,
		metrics: &ServiceClientMetrics{
			connectionStatus: "connected",
			circuitState:     "closed",
		},
	}

	// Store connection for management
	connectionKey := fmt.Sprintf("%s:%s", serviceName, serviceType)
	cm.connections[connectionKey] = conn

	return client, nil
}

func (cm *InterServiceClientManager) PerformHealthChecks(ctx context.Context) {
	cm.mu.RLock()
	clients := make([]*ServiceClient, 0, len(cm.clients))
	for _, client := range cm.clients {
		clients = append(clients, client)
	}
	cm.mu.RUnlock()

	for _, client := range clients {
		go cm.checkClientHealth(ctx, client)
	}
}

func (cm *InterServiceClientManager) checkClientHealth(ctx context.Context, client *ServiceClient) {
	client.mu.Lock()
	defer client.mu.Unlock()

	if client.connection == nil {
		client.isHealthy = false
		client.metrics.connectionStatus = "disconnected"
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := client.healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		client.isHealthy = false
		client.metrics.connectionStatus = "error"
		client.circuitBreaker.recordFailure()
		cm.logger.WithError(err).WithField("service", client.serviceName).Warn("Health check failed")
		return
	}

	client.isHealthy = resp.Status == grpc_health_v1.HealthCheckResponse_SERVING
	if client.isHealthy {
		client.metrics.connectionStatus = "healthy"
		client.circuitBreaker.recordSuccess()
	} else {
		client.metrics.connectionStatus = "unhealthy"
	}
}

func (cm *InterServiceClientManager) CleanupIdleConnections() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	idleTimeout := 5 * time.Minute

	for key, client := range cm.clients {
		client.mu.RLock()
		isIdle := now.Sub(client.lastUsed) > idleTimeout
		client.mu.RUnlock()

		if isIdle {
			cm.logger.WithFields(logrus.Fields{
				"service": client.serviceName,
				"type":    client.serviceType,
			}).Info("Closing idle connection")

			if client.connection != nil {
				client.connection.Close()
			}

			delete(cm.clients, key)
			delete(cm.connections, key)

			cm.updateMetrics(func(m *ClientManagerMetrics) {
				m.activeConnections--
				m.poolSize = int64(len(cm.clients))
			})
		}
	}
}

func (cm *InterServiceClientManager) GetMetrics() map[string]interface{} {
	cm.metrics.mu.RLock()
	defer cm.metrics.mu.RUnlock()

	return map[string]interface{}{
		"active_connections":   cm.metrics.activeConnections,
		"total_requests":       cm.metrics.totalRequests,
		"successful_requests":  cm.metrics.successfulRequests,
		"failed_requests":      cm.metrics.failedRequests,
		"circuit_open_count":   cm.metrics.circuitOpenCount,
		"connection_errors":    cm.metrics.connectionErrors,
		"pool_size":           cm.metrics.poolSize,
	}
}

func (cm *InterServiceClientManager) GetClientMetrics(serviceName, serviceType string) map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	clientKey := fmt.Sprintf("%s:%s", serviceName, serviceType)
	client, exists := cm.clients[clientKey]
	if !exists {
		return map[string]interface{}{
			"error": "client not found",
		}
	}

	client.metrics.mu.RLock()
	defer client.metrics.mu.RUnlock()

	return map[string]interface{}{
		"service_name":       serviceName,
		"service_type":       serviceType,
		"request_count":      client.metrics.requestCount,
		"success_count":      client.metrics.successCount,
		"error_count":        client.metrics.errorCount,
		"avg_response_time":  client.metrics.avgResponseTime.Milliseconds(),
		"last_request_time":  client.metrics.lastRequestTime,
		"connection_status":  client.metrics.connectionStatus,
		"circuit_state":      client.metrics.circuitState,
		"is_healthy":         client.isHealthy,
	}
}

func (cm *InterServiceClientManager) Close() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for key, conn := range cm.connections {
		if err := conn.Close(); err != nil {
			cm.logger.WithError(err).WithField("connection", key).Warn("Failed to close connection")
		}
	}

	cm.clients = make(map[string]*ServiceClient)
	cm.connections = make(map[string]*grpc.ClientConn)

	cm.logger.Info("All inter-service connections closed")
	return nil
}

func (cm *InterServiceClientManager) updateMetrics(fn func(*ClientManagerMetrics)) {
	cm.metrics.mu.Lock()
	defer cm.metrics.mu.Unlock()
	fn(cm.metrics)
}

// ServiceClient methods
func (sc *ServiceClient) GetConnection() *grpc.ClientConn {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.connection
}

func (sc *ServiceClient) IsHealthy() bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.isHealthy
}

func (sc *ServiceClient) GetServiceName() string {
	return sc.serviceName
}

func (sc *ServiceClient) GetServiceType() string {
	return sc.serviceType
}

func (sc *ServiceClient) RecordRequest(duration time.Duration, success bool) {
	sc.metrics.mu.Lock()
	defer sc.metrics.mu.Unlock()

	sc.metrics.requestCount++
	sc.metrics.lastRequestTime = time.Now()

	if success {
		sc.metrics.successCount++
		sc.circuitBreaker.recordSuccess()
	} else {
		sc.metrics.errorCount++
		sc.circuitBreaker.recordFailure()
	}

	// Update average response time
	if sc.metrics.requestCount > 1 {
		sc.metrics.avgResponseTime = time.Duration(
			(int64(sc.metrics.avgResponseTime)*(sc.metrics.requestCount-1) + int64(duration)) / sc.metrics.requestCount,
		)
	} else {
		sc.metrics.avgResponseTime = duration
	}

	sc.metrics.circuitState = sc.circuitBreaker.GetStateString()
}

// CircuitBreaker methods
func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successCount++
	cb.lastSuccTime = time.Now()

	if cb.state == CircuitHalfOpen {
		cb.state = CircuitClosed
		cb.failureCount = 0
	}
}

func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailTime = time.Now()

	if cb.state == CircuitClosed && cb.failureCount >= cb.threshold {
		cb.state = CircuitOpen
	} else if cb.state == CircuitHalfOpen {
		cb.state = CircuitOpen
	}
}

func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.state == CircuitOpen && time.Since(cb.lastFailTime) > cb.timeout {
		cb.mu.RUnlock()
		cb.mu.Lock()
		if cb.state == CircuitOpen && time.Since(cb.lastFailTime) > cb.timeout {
			cb.state = CircuitHalfOpen
		}
		cb.mu.Unlock()
		cb.mu.RLock()
	}

	return cb.state
}

func (cb *CircuitBreaker) GetStateString() string {
	switch cb.GetState() {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

func (cb *CircuitBreaker) GetMetrics() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"state":         cb.GetStateString(),
		"failure_count": cb.failureCount,
		"success_count": cb.successCount,
		"threshold":     cb.threshold,
		"timeout":       cb.timeout.Seconds(),
		"last_failure":  cb.lastFailTime,
		"last_success":  cb.lastSuccTime,
	}
}