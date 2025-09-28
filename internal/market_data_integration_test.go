//go:build integration

package internal

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/config"
	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/infrastructure"
)

// TestMarketDataIntegration_RedPhase defines the expected behaviors for market data gRPC integration
// These tests will fail initially and drive our implementation (TDD Red-Green-Refactor)
func TestMarketDataIntegration_SimulationEngine(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("can_simulate_statistical_similarity_to_real_data", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			ServiceName:    "market-data-simulator",
			ServiceVersion: "1.0.0",
			RedisURL:       "redis://localhost:6379",
			GRPCPort:       9096,
			HTTPPort:       8086,
		}

		logger := logrus.New()
		logger.SetLevel(logrus.ErrorLevel)

		// Test market data simulation engine with real data input capability
		simulationEngine := NewMarketDataSimulationEngine(cfg, logger)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Test statistical similarity simulation
		realDataInput := &MarketDataInput{
			Symbol:     "BTC/USD",
			TimeWindow: 24 * time.Hour,
			Source:     "production-feed",
			Prices: []PricePoint{
				{Timestamp: time.Now().Add(-1*time.Hour), Price: 45000.0, Volume: 1.5},
				{Timestamp: time.Now().Add(-30*time.Minute), Price: 45100.0, Volume: 2.1},
				{Timestamp: time.Now(), Price: 45200.0, Volume: 1.8},
			},
		}

		// Should generate statistically similar data
		simulatedData, err := simulationEngine.GenerateStatisticalSimilarity(ctx, realDataInput)
		if err != nil {
			t.Errorf("Failed to generate statistically similar data: %v", err)
		}

		// Validate statistical properties
		if len(simulatedData.Prices) == 0 {
			t.Error("Expected simulated data to contain price points")
		}

		// Verify statistical similarity metrics
		similarity := CalculateStatisticalSimilarity(realDataInput.Prices, simulatedData.Prices)
		if similarity.VolatilityCorrelation < 0.8 {
			t.Errorf("Expected high volatility correlation, got %f", similarity.VolatilityCorrelation)
		}

		if similarity.TrendAlignment < 0.7 {
			t.Errorf("Expected good trend alignment, got %f", similarity.TrendAlignment)
		}

		t.Logf("Statistical similarity test completed with correlation: %f", similarity.VolatilityCorrelation)
	})

	t.Run("can_simulate_market_scenarios", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			ServiceName: "market-data-simulator",
			RedisURL:    "redis://localhost:6379",
		}

		logger := logrus.New()
		logger.SetLevel(logrus.ErrorLevel)

		simulationEngine := NewMarketDataSimulationEngine(cfg, logger)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		realDataInput := &MarketDataInput{
			Symbol:     "ETH/USD",
			TimeWindow: 12 * time.Hour,
			Source:     "production-feed",
			Prices: []PricePoint{
				{Timestamp: time.Now().Add(-2*time.Hour), Price: 3000.0, Volume: 5.2},
				{Timestamp: time.Now().Add(-1*time.Hour), Price: 3020.0, Volume: 4.8},
				{Timestamp: time.Now(), Price: 3015.0, Volume: 5.1},
			},
		}

		// Test rally scenario
		rallyData, err := simulationEngine.GenerateScenario(ctx, realDataInput, ScenarioRally{
			IntensityFactor: 1.5,
			Duration:        1 * time.Hour,
			VolatilityMod:   0.8,
		})
		if err != nil {
			t.Errorf("Failed to generate rally scenario: %v", err)
		}

		if len(rallyData.Prices) == 0 {
			t.Error("Expected rally scenario to generate price data")
		}

		// Test crash scenario
		crashData, err := simulationEngine.GenerateScenario(ctx, realDataInput, ScenarioCrash{
			DropPercentage: 0.15,
			Duration:       30 * time.Minute,
			RecoveryFactor: 0.6,
		})
		if err != nil {
			t.Errorf("Failed to generate crash scenario: %v", err)
		}

		if len(crashData.Prices) == 0 {
			t.Error("Expected crash scenario to generate price data")
		}

		// Test divergence scenario
		divergenceData, err := simulationEngine.GenerateScenario(ctx, realDataInput, ScenarioDivergence{
			FromBaseline:   true,
			DivergenceMag:  0.05,
			TimeToRevert:   2 * time.Hour,
		})
		if err != nil {
			t.Errorf("Failed to generate divergence scenario: %v", err)
		}

		if len(divergenceData.Prices) == 0 {
			t.Error("Expected divergence scenario to generate price data")
		}

		// Test mean reverting scenario
		revertingData, err := simulationEngine.GenerateScenario(ctx, realDataInput, ScenarioMeanReverting{
			RevertSpeed:   0.1,
			NoiseLevel:    0.02,
			BandWidth:     0.03,
		})
		if err != nil {
			t.Errorf("Failed to generate mean reverting scenario: %v", err)
		}

		if len(revertingData.Prices) == 0 {
			t.Error("Expected mean reverting scenario to generate price data")
		}

		t.Logf("Market scenario simulation test completed successfully")
	})

	t.Run("integrates_with_production_market_data_api", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			ServiceName: "market-data-simulator",
			RedisURL:    "redis://localhost:6379",
		}

		logger := logrus.New()
		logger.SetLevel(logrus.ErrorLevel)

		// Test integration with production market data API
		marketDataClient := NewProductionMarketDataClient(cfg, logger)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Should be able to fetch real market data (will gracefully fail if service unavailable)
		realData, err := marketDataClient.FetchRealTimeData(ctx, "BTC/USD", 1*time.Hour)
		if err != nil {
			t.Logf("Production market data not available (expected in test): %v", err)
		} else {
			// If available, validate the data structure
			if realData.Symbol != "BTC/USD" {
				t.Errorf("Expected symbol BTC/USD, got %s", realData.Symbol)
			}

			if len(realData.Prices) == 0 {
				t.Error("Expected real data to contain price points")
			}
		}

		// Test market data subscription (streaming)
		subscription := marketDataClient.SubscribeToRealTimeData(ctx, []string{"BTC/USD", "ETH/USD"})
		if subscription == nil {
			t.Error("Expected subscription to be created")
		}

		// Test market data event channel
		select {
		case event := <-subscription.EventChannel():
			t.Logf("Received market data event: %+v", event)
		case <-time.After(1 * time.Second):
			t.Log("No market data events received (expected in test environment)")
		}

		t.Logf("Production market data API integration test completed")
	})
}

func TestMarketDataIntegration_gRPCStreaming(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("can_stream_market_data_via_grpc", func(t *testing.T) {
		cfg := &config.Config{
			ServiceName:    "market-data-simulator",
			ServiceVersion: "1.0.0",
			RedisURL:       "redis://localhost:6379",
			GRPCPort:       9097,
			HTTPPort:       8087,
		}

		logger := logrus.New()
		logger.SetLevel(logrus.ErrorLevel)

		// Test gRPC streaming server for market data
		marketDataServer := NewMarketDataGRPCServer(cfg, logger)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := marketDataServer.Start(ctx)
		if err != nil {
			t.Skipf("gRPC infrastructure not available: %v", err)
			return
		}
		defer marketDataServer.Stop(ctx)

		// Test market data streaming client
		clientManager := infrastructure.NewInterServiceClientManager(cfg, logger,
			infrastructure.NewServiceDiscoveryClient(cfg, logger),
			infrastructure.NewConfigurationClient(cfg, logger))

		marketDataClient, err := clientManager.GetMarketDataClient()
		if err != nil {
			t.Errorf("Failed to get market data client: %v", err)
		}

		// Test price streaming
		priceStream, err := marketDataClient.StreamPrices(ctx, &PriceStreamRequest{
			Symbols:    []string{"BTC/USD", "ETH/USD"},
			Interval:   "1s",
			BufferSize: 100,
		})
		if err != nil {
			t.Errorf("Failed to create price stream: %v", err)
		}

		// Verify streaming works
		select {
		case update := <-priceStream.Updates():
			if update.Symbol == "" {
				t.Error("Expected price update to have symbol")
			}
			if update.Price <= 0 {
				t.Error("Expected positive price in update")
			}
		case <-time.After(2 * time.Second):
			t.Log("No price updates received (may be expected in test)")
		}

		// Test market data metrics
		metrics := marketDataServer.GetMarketDataMetrics()
		if metrics.ActiveStreams < 0 {
			t.Error("Expected non-negative active streams")
		}

		if metrics.PriceUpdatesCount < 0 {
			t.Error("Expected non-negative price update count")
		}

		t.Logf("gRPC streaming test completed successfully")
	})

	t.Run("handles_market_data_subscription_lifecycle", func(t *testing.T) {
		cfg := &config.Config{
			ServiceName: "market-data-simulator",
			RedisURL:    "redis://localhost:6379",
		}

		logger := logrus.New()
		logger.SetLevel(logrus.ErrorLevel)

		subscriptionManager := NewMarketDataSubscriptionManager(cfg, logger)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Test subscription creation
		subID, err := subscriptionManager.CreateSubscription(ctx, &SubscriptionRequest{
			ClientID: "risk-monitor-client",
			Symbols:  []string{"BTC/USD", "ETH/USD"},
			Interval: "1s",
			BufferSize: 1000,
		})
		if err != nil {
			t.Errorf("Failed to create subscription: %v", err)
		}

		if subID == "" {
			t.Error("Expected non-empty subscription ID")
		}

		// Test subscription status
		status, err := subscriptionManager.GetSubscriptionStatus(ctx, subID)
		if err != nil {
			t.Errorf("Failed to get subscription status: %v", err)
		}

		if status.State != "active" {
			t.Errorf("Expected active subscription, got %s", status.State)
		}

		// Test subscription cancellation
		err = subscriptionManager.CancelSubscription(ctx, subID)
		if err != nil {
			t.Errorf("Failed to cancel subscription: %v", err)
		}

		// Verify cancellation
		cancelledStatus, err := subscriptionManager.GetSubscriptionStatus(ctx, subID)
		if err == nil && cancelledStatus.State != "cancelled" {
			t.Error("Expected subscription to be cancelled")
		}

		t.Logf("Subscription lifecycle test completed successfully")
	})
}

func TestMarketDataIntegration_InterServiceCommunication(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("communicates_with_risk_monitor_service", func(t *testing.T) {
		cfg := &config.Config{
			ServiceName: "market-data-simulator",
			RedisURL:    "redis://localhost:6379",
		}

		logger := logrus.New()
		logger.SetLevel(logrus.ErrorLevel)

		// Test communication with risk monitor service
		serviceDiscovery := infrastructure.NewServiceDiscoveryClient(cfg, logger)
		err := serviceDiscovery.Start()
		if err != nil {
			t.Skipf("Service discovery infrastructure not available: %v", err)
			return
		}
		defer serviceDiscovery.Stop()

		clientManager := infrastructure.NewInterServiceClientManager(cfg, logger, serviceDiscovery,
			infrastructure.NewConfigurationClient(cfg, logger))

		// Test risk monitor client communication
		riskMonitorClient, err := clientManager.GetRiskMonitorClient()
		if err != nil {
			t.Logf("Risk monitor service not available for test: %v", err)
		} else {
			// Test health check
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err = riskMonitorClient.HealthCheck(ctx)
			if err != nil {
				t.Logf("Risk monitor health check failed (expected in test): %v", err)
			}

			// Test price update notification
			err = riskMonitorClient.NotifyPriceUpdate(ctx, &PriceUpdateNotification{
				Symbol:    "BTC/USD",
				Price:     45300.0,
				Timestamp: time.Now(),
				Source:    "market-data-simulator",
			})
			if err != nil {
				t.Logf("Price update notification failed (expected in test): %v", err)
			}
		}

		t.Logf("Risk monitor communication test completed")
	})

	t.Run("integrates_with_service_discovery_and_config", func(t *testing.T) {
		cfg := &config.Config{
			ServiceName:    "market-data-simulator",
			ServiceVersion: "1.0.0-integration",
			RedisURL:       "redis://localhost:6379",
			GRPCPort:       9098,
			HTTPPort:       8088,
		}

		logger := logrus.New()
		logger.SetLevel(logrus.ErrorLevel)

		// Test complete infrastructure integration
		serviceDiscovery := infrastructure.NewServiceDiscoveryClient(cfg, logger)
		configClient := infrastructure.NewConfigurationClient(cfg, logger)
		clientManager := infrastructure.NewInterServiceClientManager(cfg, logger, serviceDiscovery, configClient)

		// Test service discovery
		err := serviceDiscovery.Start()
		if err != nil {
			t.Skipf("Service discovery not available: %v", err)
			return
		}
		defer serviceDiscovery.Stop()

		// Verify service registration
		time.Sleep(100 * time.Millisecond)
		services, err := serviceDiscovery.DiscoverServices("market-data-simulator")
		if err != nil {
			t.Errorf("Failed to discover market data services: %v", err)
		}

		found := false
		for _, service := range services {
			if service.ServiceName == "market-data-simulator" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find market data simulator service in discovery")
		}

		// Test configuration retrieval for market data parameters
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err = configClient.GetConfiguration(ctx, "market-data.simulation.volatility-factor")
		if err != nil {
			t.Logf("Market data configuration not available (expected in test): %v", err)
		}

		// Test cleanup
		err = clientManager.Close()
		if err != nil {
			t.Errorf("Failed to close client manager: %v", err)
		}

		t.Logf("Infrastructure integration test completed successfully")
	})
}

// Data structures for market data simulation (these will fail compilation until implemented)

type MarketDataInput struct {
	Symbol     string        `json:"symbol"`
	TimeWindow time.Duration `json:"time_window"`
	Source     string        `json:"source"`
	Prices     []PricePoint  `json:"prices"`
}

type PricePoint struct {
	Timestamp time.Time `json:"timestamp"`
	Price     float64   `json:"price"`
	Volume    float64   `json:"volume"`
}

type SimulatedData struct {
	Symbol string       `json:"symbol"`
	Prices []PricePoint `json:"prices"`
	Metadata map[string]interface{} `json:"metadata"`
}

type StatisticalSimilarity struct {
	VolatilityCorrelation float64 `json:"volatility_correlation"`
	TrendAlignment        float64 `json:"trend_alignment"`
	VolumeCorrelation     float64 `json:"volume_correlation"`
}

type ScenarioRally struct {
	IntensityFactor float64       `json:"intensity_factor"`
	Duration        time.Duration `json:"duration"`
	VolatilityMod   float64       `json:"volatility_mod"`
}

type ScenarioCrash struct {
	DropPercentage float64       `json:"drop_percentage"`
	Duration       time.Duration `json:"duration"`
	RecoveryFactor float64       `json:"recovery_factor"`
}

type ScenarioDivergence struct {
	FromBaseline  bool          `json:"from_baseline"`
	DivergenceMag float64       `json:"divergence_magnitude"`
	TimeToRevert  time.Duration `json:"time_to_revert"`
}

type ScenarioMeanReverting struct {
	RevertSpeed float64 `json:"revert_speed"`
	NoiseLevel  float64 `json:"noise_level"`
	BandWidth   float64 `json:"band_width"`
}

type PriceStreamRequest struct {
	Symbols    []string `json:"symbols"`
	Interval   string   `json:"interval"`
	BufferSize int      `json:"buffer_size"`
}

type PriceUpdateNotification struct {
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`
}

type SubscriptionRequest struct {
	ClientID   string   `json:"client_id"`
	Symbols    []string `json:"symbols"`
	Interval   string   `json:"interval"`
	BufferSize int      `json:"buffer_size"`
}

type SubscriptionStatus struct {
	ID    string `json:"id"`
	State string `json:"state"`
	SymbolCount int `json:"symbol_count"`
	LastUpdate  time.Time `json:"last_update"`
}

// Interface definitions (these will fail compilation until implemented)

type MarketDataSimulationEngine interface {
	GenerateStatisticalSimilarity(ctx context.Context, input *MarketDataInput) (*SimulatedData, error)
	GenerateScenario(ctx context.Context, input *MarketDataInput, scenario interface{}) (*SimulatedData, error)
}

type ProductionMarketDataClient interface {
	FetchRealTimeData(ctx context.Context, symbol string, timeWindow time.Duration) (*MarketDataInput, error)
	SubscribeToRealTimeData(ctx context.Context, symbols []string) MarketDataSubscription
}

type MarketDataSubscription interface {
	EventChannel() <-chan interface{}
	Close() error
}

type MarketDataGRPCServer interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	GetMarketDataMetrics() MarketDataMetrics
}

type MarketDataClient interface {
	StreamPrices(ctx context.Context, req *PriceStreamRequest) (PriceStream, error)
}

type PriceStream interface {
	Updates() <-chan PriceUpdate
	Close() error
}

type PriceUpdate struct {
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Volume    float64   `json:"volume"`
	Timestamp time.Time `json:"timestamp"`
}

type MarketDataMetrics struct {
	ActiveStreams     int   `json:"active_streams"`
	PriceUpdatesCount int64 `json:"price_updates_count"`
	SubscriberCount   int   `json:"subscriber_count"`
}

type MarketDataSubscriptionManager interface {
	CreateSubscription(ctx context.Context, req *SubscriptionRequest) (string, error)
	GetSubscriptionStatus(ctx context.Context, subID string) (*SubscriptionStatus, error)
	CancelSubscription(ctx context.Context, subID string) error
}

type RiskMonitorClient interface {
	HealthCheck(ctx context.Context) error
	NotifyPriceUpdate(ctx context.Context, notification *PriceUpdateNotification) error
}

// Factory functions (these will fail compilation until implemented)

func NewMarketDataSimulationEngine(cfg *config.Config, logger *logrus.Logger) MarketDataSimulationEngine {
	return nil // Will fail until implemented
}

func NewProductionMarketDataClient(cfg *config.Config, logger *logrus.Logger) ProductionMarketDataClient {
	return nil // Will fail until implemented
}

func NewMarketDataGRPCServer(cfg *config.Config, logger *logrus.Logger) MarketDataGRPCServer {
	return nil // Will fail until implemented
}

func NewMarketDataSubscriptionManager(cfg *config.Config, logger *logrus.Logger) MarketDataSubscriptionManager {
	return nil // Will fail until implemented
}

func CalculateStatisticalSimilarity(real, simulated []PricePoint) StatisticalSimilarity {
	return StatisticalSimilarity{} // Will fail until implemented
}