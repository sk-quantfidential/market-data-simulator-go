package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/config"
)

type ServiceDiscovery struct {
	config         *config.Config
	logger         *logrus.Logger
	redisClient    *redis.Client
	registration   *ServiceRegistration
	metrics        *DiscoveryMetrics
	heartbeatStop  chan bool
	heartbeatDone  chan bool
	isRegistered   bool
	mu             sync.RWMutex
}

type ServiceRegistration struct {
	ServiceName    string            `json:"service_name"`
	ServiceVersion string            `json:"service_version"`
	InstanceID     string            `json:"instance_id"`
	Address        string            `json:"address"`
	Port           int               `json:"port"`
	GRPCPort       int               `json:"grpc_port"`
	HTTPPort       int               `json:"http_port"`
	Health         string            `json:"health"`
	Status         string            `json:"status"`
	RegisteredAt   time.Time         `json:"registered_at"`
	LastHeartbeat  time.Time         `json:"last_heartbeat"`
	Metadata       map[string]string `json:"metadata"`
	Tags           []string          `json:"tags"`
}

type ServiceInfo struct {
	ServiceName    string            `json:"service_name"`
	ServiceVersion string            `json:"service_version"`
	InstanceID     string            `json:"instance_id"`
	Address        string            `json:"address"`
	Port           int               `json:"port"`
	GRPCPort       int               `json:"grpc_port"`
	HTTPPort       int               `json:"http_port"`
	Health         string            `json:"health"`
	Status         string            `json:"status"`
	RegisteredAt   time.Time         `json:"registered_at"`
	LastHeartbeat  time.Time         `json:"last_heartbeat"`
	Metadata       map[string]string `json:"metadata"`
	Tags           []string          `json:"tags"`
}

type DiscoveryMetrics struct {
	mu                    sync.RWMutex
	registrationCount     int64
	deregistrationCount   int64
	heartbeatCount        int64
	discoveryRequestCount int64
	healthyServices       int64
	unhealthyServices     int64
	connectionStatus      string
	lastHeartbeat         time.Time
	errorCount            int64
}

func NewServiceDiscovery(cfg *config.Config, logger *logrus.Logger) *ServiceDiscovery {
	// Parse Redis URL
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.WithError(err).Warn("Failed to parse Redis URL, using defaults")
		redisOpts = &redis.Options{
			Addr: "localhost:6379",
		}
	}

	redisClient := redis.NewClient(redisOpts)

	instanceID := fmt.Sprintf("%s-%d", cfg.ServiceName, time.Now().Unix())

	registration := &ServiceRegistration{
		ServiceName:    cfg.ServiceName,
		ServiceVersion: cfg.ServiceVersion,
		InstanceID:     instanceID,
		Address:        "localhost", // Could be configurable
		Port:           cfg.HTTPPort,
		GRPCPort:       cfg.GRPCPort,
		HTTPPort:       cfg.HTTPPort,
		Health:         "healthy",
		Status:         "active",
		RegisteredAt:   time.Now(),
		LastHeartbeat:  time.Now(),
		Metadata: map[string]string{
			"environment": "development",
			"region":      "local",
			"datacenter":  "local",
		},
		Tags: []string{
			"market-data",
			"simulator",
			"grpc",
			"http",
		},
	}

	metrics := &DiscoveryMetrics{
		connectionStatus: "unknown",
	}

	return &ServiceDiscovery{
		config:        cfg,
		logger:        logger,
		redisClient:   redisClient,
		registration:  registration,
		metrics:       metrics,
		heartbeatStop: make(chan bool),
		heartbeatDone: make(chan bool),
	}
}

func (sd *ServiceDiscovery) Register(ctx context.Context) error {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	if sd.isRegistered {
		return fmt.Errorf("service already registered")
	}

	// Test Redis connection
	if err := sd.testConnection(ctx); err != nil {
		sd.updateMetrics(func(m *DiscoveryMetrics) {
			m.errorCount++
			m.connectionStatus = "error"
		})
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Register service
	key := sd.getServiceKey()
	registrationData, err := json.Marshal(sd.registration)
	if err != nil {
		return fmt.Errorf("failed to marshal registration data: %w", err)
	}

	// Set service registration with TTL
	err = sd.redisClient.SetEx(ctx, key, registrationData, 30*time.Second).Err()
	if err != nil {
		sd.updateMetrics(func(m *DiscoveryMetrics) {
			m.errorCount++
			m.connectionStatus = "error"
		})
		return fmt.Errorf("failed to register service: %w", err)
	}

	// Add to service list
	listKey := sd.getServiceListKey()
	err = sd.redisClient.SAdd(ctx, listKey, sd.registration.InstanceID).Err()
	if err != nil {
		sd.logger.WithError(err).Warn("Failed to add service to list")
	}

	sd.isRegistered = true
	sd.updateMetrics(func(m *DiscoveryMetrics) {
		m.registrationCount++
		m.connectionStatus = "healthy"
	})

	sd.logger.WithFields(logrus.Fields{
		"service_name": sd.registration.ServiceName,
		"instance_id":  sd.registration.InstanceID,
		"address":      fmt.Sprintf("%s:%d", sd.registration.Address, sd.registration.Port),
		"grpc_port":    sd.registration.GRPCPort,
	}).Info("Service registered successfully")

	// Start heartbeat
	go sd.startHeartbeat()

	return nil
}

func (sd *ServiceDiscovery) Deregister(ctx context.Context) error {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	if !sd.isRegistered {
		return fmt.Errorf("service not registered")
	}

	// Stop heartbeat
	sd.heartbeatStop <- true
	<-sd.heartbeatDone

	// Remove service registration
	key := sd.getServiceKey()
	err := sd.redisClient.Del(ctx, key).Err()
	if err != nil {
		sd.logger.WithError(err).Warn("Failed to remove service registration")
	}

	// Remove from service list
	listKey := sd.getServiceListKey()
	err = sd.redisClient.SRem(ctx, listKey, sd.registration.InstanceID).Err()
	if err != nil {
		sd.logger.WithError(err).Warn("Failed to remove service from list")
	}

	sd.isRegistered = false
	sd.updateMetrics(func(m *DiscoveryMetrics) {
		m.deregistrationCount++
	})

	sd.logger.WithField("instance_id", sd.registration.InstanceID).Info("Service deregistered successfully")

	return nil
}

func (sd *ServiceDiscovery) DiscoverService(ctx context.Context, serviceName string) ([]*ServiceInfo, error) {
	sd.updateMetrics(func(m *DiscoveryMetrics) {
		m.discoveryRequestCount++
	})

	pattern := fmt.Sprintf("services:%s:*", serviceName)
	keys, err := sd.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		sd.updateMetrics(func(m *DiscoveryMetrics) {
			m.errorCount++
		})
		return nil, fmt.Errorf("failed to discover services: %w", err)
	}

	var services []*ServiceInfo
	healthy := int64(0)
	unhealthy := int64(0)

	for _, key := range keys {
		data, err := sd.redisClient.Get(ctx, key).Result()
		if err != nil {
			sd.logger.WithError(err).WithField("key", key).Warn("Failed to get service data")
			continue
		}

		var serviceInfo ServiceInfo
		if err := json.Unmarshal([]byte(data), &serviceInfo); err != nil {
			sd.logger.WithError(err).WithField("key", key).Warn("Failed to unmarshal service data")
			continue
		}

		// Check if service is still healthy (heartbeat within last 60 seconds)
		if time.Since(serviceInfo.LastHeartbeat) > 60*time.Second {
			serviceInfo.Health = "unhealthy"
			serviceInfo.Status = "stale"
			unhealthy++
		} else {
			healthy++
		}

		services = append(services, &serviceInfo)
	}

	sd.updateMetrics(func(m *DiscoveryMetrics) {
		m.healthyServices = healthy
		m.unhealthyServices = unhealthy
	})

	sd.logger.WithFields(logrus.Fields{
		"service_name":      serviceName,
		"discovered_count":  len(services),
		"healthy_services":  healthy,
		"unhealthy_services": unhealthy,
	}).Debug("Service discovery completed")

	return services, nil
}

func (sd *ServiceDiscovery) GetHealthyInstances(ctx context.Context, serviceName string) ([]*ServiceInfo, error) {
	allServices, err := sd.DiscoverService(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	var healthyServices []*ServiceInfo
	for _, service := range allServices {
		if service.Health == "healthy" && service.Status == "active" {
			healthyServices = append(healthyServices, service)
		}
	}

	return healthyServices, nil
}

func (sd *ServiceDiscovery) UpdateHealth(ctx context.Context, health string) error {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	if !sd.isRegistered {
		return fmt.Errorf("service not registered")
	}

	sd.registration.Health = health
	sd.registration.LastHeartbeat = time.Now()

	key := sd.getServiceKey()
	registrationData, err := json.Marshal(sd.registration)
	if err != nil {
		return fmt.Errorf("failed to marshal registration data: %w", err)
	}

	err = sd.redisClient.SetEx(ctx, key, registrationData, 30*time.Second).Err()
	if err != nil {
		return fmt.Errorf("failed to update health: %w", err)
	}

	sd.logger.WithFields(logrus.Fields{
		"instance_id": sd.registration.InstanceID,
		"health":      health,
	}).Debug("Health status updated")

	return nil
}

func (sd *ServiceDiscovery) GetMetrics() map[string]interface{} {
	sd.metrics.mu.RLock()
	defer sd.metrics.mu.RUnlock()

	return map[string]interface{}{
		"registration_count":       sd.metrics.registrationCount,
		"deregistration_count":     sd.metrics.deregistrationCount,
		"heartbeat_count":          sd.metrics.heartbeatCount,
		"discovery_request_count":  sd.metrics.discoveryRequestCount,
		"healthy_services":         sd.metrics.healthyServices,
		"unhealthy_services":       sd.metrics.unhealthyServices,
		"connection_status":        sd.metrics.connectionStatus,
		"last_heartbeat":           sd.metrics.lastHeartbeat,
		"error_count":              sd.metrics.errorCount,
		"is_registered":            sd.isRegistered,
		"instance_id":              sd.registration.InstanceID,
		"service_name":             sd.registration.ServiceName,
	}
}

func (sd *ServiceDiscovery) IsRegistered() bool {
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	return sd.isRegistered
}

func (sd *ServiceDiscovery) GetRegistration() *ServiceInfo {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	return &ServiceInfo{
		ServiceName:    sd.registration.ServiceName,
		ServiceVersion: sd.registration.ServiceVersion,
		InstanceID:     sd.registration.InstanceID,
		Address:        sd.registration.Address,
		Port:           sd.registration.Port,
		GRPCPort:       sd.registration.GRPCPort,
		HTTPPort:       sd.registration.HTTPPort,
		Health:         sd.registration.Health,
		Status:         sd.registration.Status,
		RegisteredAt:   sd.registration.RegisteredAt,
		LastHeartbeat:  sd.registration.LastHeartbeat,
		Metadata:       sd.registration.Metadata,
		Tags:           sd.registration.Tags,
	}
}

func (sd *ServiceDiscovery) CleanupStaleServices(ctx context.Context) error {
	pattern := "services:*"
	keys, err := sd.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get service keys: %w", err)
	}

	cleaned := 0
	for _, key := range keys {
		// Check if key exists and get TTL
		ttl, err := sd.redisClient.TTL(ctx, key).Result()
		if err != nil {
			continue
		}

		// If TTL is -1 (no expiration) or the key has expired, check the data
		if ttl == -1 {
			data, err := sd.redisClient.Get(ctx, key).Result()
			if err != nil {
				continue
			}

			var serviceInfo ServiceInfo
			if err := json.Unmarshal([]byte(data), &serviceInfo); err != nil {
				continue
			}

			// Remove stale services (no heartbeat for 2 minutes)
			if time.Since(serviceInfo.LastHeartbeat) > 2*time.Minute {
				sd.redisClient.Del(ctx, key)
				cleaned++
			}
		}
	}

	if cleaned > 0 {
		sd.logger.WithField("cleaned_services", cleaned).Info("Cleaned up stale services")
	}

	return nil
}

func (sd *ServiceDiscovery) startHeartbeat() {
	ticker := time.NewTicker(15 * time.Second) // Heartbeat every 15 seconds
	defer ticker.Stop()

	for {
		select {
		case <-sd.heartbeatStop:
			sd.heartbeatDone <- true
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := sd.sendHeartbeat(ctx)
			cancel()

			if err != nil {
				sd.logger.WithError(err).Warn("Failed to send heartbeat")
				sd.updateMetrics(func(m *DiscoveryMetrics) {
					m.errorCount++
				})
			} else {
				sd.updateMetrics(func(m *DiscoveryMetrics) {
					m.heartbeatCount++
					m.lastHeartbeat = time.Now()
				})
			}
		}
	}
}

func (sd *ServiceDiscovery) sendHeartbeat(ctx context.Context) error {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	if !sd.isRegistered {
		return fmt.Errorf("service not registered")
	}

	sd.registration.LastHeartbeat = time.Now()

	key := sd.getServiceKey()
	registrationData, err := json.Marshal(sd.registration)
	if err != nil {
		return fmt.Errorf("failed to marshal registration data: %w", err)
	}

	// Refresh TTL on heartbeat
	err = sd.redisClient.SetEx(ctx, key, registrationData, 30*time.Second).Err()
	if err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}

	return nil
}

func (sd *ServiceDiscovery) testConnection(ctx context.Context) error {
	return sd.redisClient.Ping(ctx).Err()
}

func (sd *ServiceDiscovery) getServiceKey() string {
	return fmt.Sprintf("services:%s:%s", sd.registration.ServiceName, sd.registration.InstanceID)
}

func (sd *ServiceDiscovery) getServiceListKey() string {
	return fmt.Sprintf("service_list:%s", sd.registration.ServiceName)
}

func (sd *ServiceDiscovery) updateMetrics(fn func(*DiscoveryMetrics)) {
	sd.metrics.mu.Lock()
	defer sd.metrics.mu.Unlock()
	fn(sd.metrics)
}

func (sd *ServiceDiscovery) Close() error {
	if sd.isRegistered {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		sd.Deregister(ctx)
	}
	return sd.redisClient.Close()
}

// Utility functions
func FilterServicesByTag(services []*ServiceInfo, tag string) []*ServiceInfo {
	var filtered []*ServiceInfo
	for _, service := range services {
		for _, serviceTag := range service.Tags {
			if serviceTag == tag {
				filtered = append(filtered, service)
				break
			}
		}
	}
	return filtered
}

func FilterServicesByMetadata(services []*ServiceInfo, key, value string) []*ServiceInfo {
	var filtered []*ServiceInfo
	for _, service := range services {
		if metaValue, exists := service.Metadata[key]; exists && metaValue == value {
			filtered = append(filtered, service)
		}
	}
	return filtered
}

func GetServiceEndpoint(service *ServiceInfo, protocol string) string {
	switch strings.ToLower(protocol) {
	case "grpc":
		return fmt.Sprintf("%s:%d", service.Address, service.GRPCPort)
	case "http":
		return fmt.Sprintf("http://%s:%d", service.Address, service.HTTPPort)
	default:
		return fmt.Sprintf("%s:%d", service.Address, service.Port)
	}
}