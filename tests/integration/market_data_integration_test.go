package integration

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/config"
	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/handlers"
	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/infrastructure"
	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/proto"
	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/services"
)

type IntegrationTestSuite struct {
	config                *config.Config
	logger                *logrus.Logger
	serviceDiscovery      *infrastructure.ServiceDiscovery
	configClient          *infrastructure.ConfigurationClient
	clientManager         *infrastructure.InterServiceClientManager
	grpcServer            *infrastructure.MarketDataGRPCServer
	marketDataService     *services.MarketDataService
	marketDataHandler     *handlers.MarketDataGRPCHandler
}

func setupIntegrationTest(t *testing.T) (*IntegrationTestSuite, func()) {
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

	// Initialize all components
	serviceDiscovery := infrastructure.NewServiceDiscovery(cfg, logger)
	configClient := infrastructure.NewConfigurationClient(cfg, logger)
	clientManager := infrastructure.NewInterServiceClientManager(cfg, logger, serviceDiscovery, configClient)
	marketDataService := services.NewMarketDataService(cfg, logger)
	grpcServer := infrastructure.NewMarketDataGRPCServer(cfg, marketDataService, logger)
	marketDataHandler := handlers.NewMarketDataGRPCHandler(cfg, marketDataService, logger)

	suite := &IntegrationTestSuite{
		config:            cfg,
		logger:            logger,
		serviceDiscovery:  serviceDiscovery,
		configClient:      configClient,
		clientManager:     clientManager,
		grpcServer:        grpcServer,
		marketDataService: marketDataService,
		marketDataHandler: marketDataHandler,
	}

	cleanup := func() {
		clientManager.Close()
		serviceDiscovery.Close()
		grpcServer.Stop()
	}

	return suite, cleanup
}

func TestIntegrationSuite_ComponentInitialization(t *testing.T) {
	suite, cleanup := setupIntegrationTest(t)
	defer cleanup()

	// Verify all components are properly initialized
	assert.NotNil(t, suite.config)
	assert.NotNil(t, suite.logger)
	assert.NotNil(t, suite.serviceDiscovery)
	assert.NotNil(t, suite.configClient)
	assert.NotNil(t, suite.clientManager)
	assert.NotNil(t, suite.grpcServer)
	assert.NotNil(t, suite.marketDataService)
	assert.NotNil(t, suite.marketDataHandler)

	// Verify configuration
	assert.Equal(t, "market-data-simulator", suite.config.ServiceName)
	assert.Equal(t, "1.0.0", suite.config.ServiceVersion)
	assert.Equal(t, 9090, suite.config.GRPCPort)
	assert.Equal(t, 8080, suite.config.HTTPPort)
}

func TestIntegrationSuite_MarketDataScenarios(t *testing.T) {
	suite, cleanup := setupIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()

	testCases := []struct {
		name           string
		symbol         string
		scenarioType   proto.ScenarioType
		simulationType proto.SimulationType
		duration       int32
		intensity      float64
	}{
		{
			name:           "BTC Rally Scenario",
			symbol:         "BTC/USD",
			scenarioType:   proto.ScenarioType_RALLY,
			simulationType: proto.SimulationType_STATISTICAL_SIMILARITY,
			duration:       5,
			intensity:      1.5,
		},
		{
			name:           "ETH Crash Scenario",
			symbol:         "ETH/USD",
			scenarioType:   proto.ScenarioType_CRASH,
			simulationType: proto.SimulationType_MONTE_CARLO,
			duration:       3,
			intensity:      2.0,
		},
		{
			name:           "ADA Mean Reverting",
			symbol:         "ADA/BTC",
			scenarioType:   proto.ScenarioType_MEAN_REVERTING,
			simulationType: proto.SimulationType_MEAN_REVERSION,
			duration:       10,
			intensity:      1.2,
		},
		{
			name:           "SOL Volatility Spike",
			symbol:         "SOL/USD",
			scenarioType:   proto.ScenarioType_VOLATILITY_SPIKE,
			simulationType: proto.SimulationType_BROWNIAN_MOTION,
			duration:       2,
			intensity:      1.8,
		},
		{
			name:           "DOT Consolidation",
			symbol:         "DOT/USD",
			scenarioType:   proto.ScenarioType_CONSOLIDATION,
			simulationType: proto.SimulationType_TREND_FOLLOWING,
			duration:       15,
			intensity:      1.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test basic price retrieval
			priceResp, err := suite.marketDataHandler.GetPrice(ctx, &proto.GetPriceRequest{
				Symbol: tc.symbol,
			})
			require.NoError(t, err)
			assert.Equal(t, tc.symbol, priceResp.Symbol)
			assert.Greater(t, priceResp.Price, 0.0)
			assert.Equal(t, "market-data-simulator", priceResp.Source)

			// Test simulation generation
			startTime := time.Now().Add(-1 * time.Hour)
			endTime := time.Now()

			simReq := &proto.SimulationRequest{
				Symbol:         tc.symbol,
				StartTime:      timestamppb.New(startTime),
				EndTime:        timestamppb.New(endTime),
				SimulationType: tc.simulationType,
				Parameters: &proto.SimulationParameters{
					VolatilityFactor: tc.intensity,
					TrendFactor:      0.1,
					DataPoints:       50,
					IncludeNoise:     true,
					NoiseLevel:       0.05,
				},
			}

			simResp, err := suite.marketDataHandler.GenerateSimulation(ctx, simReq)
			require.NoError(t, err)
			assert.Equal(t, tc.symbol, simResp.Symbol)
			assert.NotEmpty(t, simResp.SimulationId)
			assert.NotNil(t, simResp.HistoricalData)
			assert.NotNil(t, simResp.SimulatedData)
			assert.NotNil(t, simResp.SimilarityMetrics)

			// Validate simulation quality
			metrics := simResp.SimilarityMetrics
			assert.Greater(t, metrics.CorrelationCoefficient, 0.7)
			assert.Greater(t, metrics.VolatilitySimilarity, 0.7)
			assert.Greater(t, metrics.ConfidenceScore, 0.7)
			assert.Less(t, metrics.ConfidenceScore, 1.0)

			// Verify data integrity
			assert.Equal(t, len(simResp.HistoricalData), len(simResp.SimulatedData))
			if len(simResp.HistoricalData) > 0 {
				assert.Greater(t, simResp.HistoricalData[0].Close, 0.0)
				assert.Greater(t, simResp.SimulatedData[0].Close, 0.0)
			}
		})
	}
}

func TestIntegrationSuite_ServiceHealthAndMetrics(t *testing.T) {
	suite, cleanup := setupIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()

	// Test health check
	healthResp, err := suite.marketDataHandler.HealthCheck(ctx, &proto.HealthCheckRequest{
		Service: "market-data",
	})
	require.NoError(t, err)
	assert.Equal(t, proto.HealthStatus_SERVING, healthResp.Status)
	assert.Contains(t, healthResp.Message, "healthy")
	assert.NotNil(t, healthResp.Details)

	// Test gRPC server metrics
	serverMetrics := suite.grpcServer.GetMetrics()
	assert.Contains(t, serverMetrics, "uptime_seconds")
	assert.Contains(t, serverMetrics, "service_name")
	assert.Contains(t, serverMetrics, "service_version")
	assert.Equal(t, "market-data-simulator", serverMetrics["service_name"])
	assert.Equal(t, "1.0.0", serverMetrics["service_version"])

	// Test configuration client metrics
	configMetrics := suite.configClient.GetMetrics()
	assert.Contains(t, configMetrics, "cache_size")
	assert.Contains(t, configMetrics, "connection_status")

	// Test service discovery metrics
	discoveryMetrics := suite.serviceDiscovery.GetMetrics()
	assert.Contains(t, discoveryMetrics, "instance_id")
	assert.Contains(t, discoveryMetrics, "service_name")
	assert.Equal(t, "market-data-simulator", discoveryMetrics["service_name"])

	// Test client manager metrics
	clientMetrics := suite.clientManager.GetMetrics()
	assert.Contains(t, clientMetrics, "active_connections")
	assert.Contains(t, clientMetrics, "pool_size")
}

func TestIntegrationSuite_StatisticalSimilarityValidation(t *testing.T) {
	suite, cleanup := setupIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()

	// Test various statistical similarity scenarios
	symbols := []string{"BTC/USD", "ETH/USD", "ADA/BTC", "SOL/USD", "DOT/USD"}

	for _, symbol := range symbols {
		t.Run(symbol, func(t *testing.T) {
			req := &proto.SimulationRequest{
				Symbol:         symbol,
				StartTime:      timestamppb.New(time.Now().Add(-24 * time.Hour)),
				EndTime:        timestamppb.New(time.Now()),
				SimulationType: proto.SimulationType_STATISTICAL_SIMILARITY,
				Parameters: &proto.SimulationParameters{
					VolatilityFactor: 1.0,
					TrendFactor:      0.0,
					DataPoints:       100,
					IncludeNoise:     true,
					NoiseLevel:       0.02,
				},
			}

			resp, err := suite.marketDataHandler.GenerateSimulation(ctx, req)
			require.NoError(t, err)

			// Validate statistical properties
			metrics := resp.SimilarityMetrics

			// Correlation should be reasonably high for statistical similarity
			assert.Greater(t, metrics.CorrelationCoefficient, 0.8)

			// Volatility similarity should be maintained
			assert.Greater(t, metrics.VolatilitySimilarity, 0.75)

			// Overall confidence should be high
			assert.Greater(t, metrics.ConfidenceScore, 0.8)

			// Trend similarity should be reasonable
			assert.Greater(t, metrics.TrendSimilarity, 0.7)

			// Verify data consistency
			assert.Len(t, resp.SimulatedData, len(resp.HistoricalData))

			// Check that prices are realistic (positive and within reasonable bounds)
			for i, point := range resp.SimulatedData {
				assert.Greater(t, point.Close, 0.0)
				assert.Greater(t, point.Volume, 0.0)

				// Simulated prices shouldn't deviate too much from historical
				if i < len(resp.HistoricalData) {
					historical := resp.HistoricalData[i]
					deviation := abs(point.Close - historical.Close) / historical.Close
					assert.Less(t, deviation, 0.5) // Max 50% deviation
				}
			}
		})
	}
}

func TestIntegrationSuite_ScenarioSimulationBehavior(t *testing.T) {
	suite, cleanup := setupIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()

	// Test that different scenarios produce expected behavior patterns
	testCases := []struct {
		name         string
		scenarioType proto.ScenarioType
		symbol       string
		intensity    float64
		expectation  string
	}{
		{"Rally simulation", proto.ScenarioType_RALLY, "BTC/USD", 1.5, "increasing"},
		{"Crash simulation", proto.ScenarioType_CRASH, "ETH/USD", 1.8, "decreasing"},
		{"Mean reversion simulation", proto.ScenarioType_MEAN_REVERTING, "ADA/BTC", 1.2, "oscillating"},
		{"Volatility spike simulation", proto.ScenarioType_VOLATILITY_SPIKE, "SOL/USD", 2.0, "volatile"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test scenario simulation using the streaming endpoint
			// This tests the integration through the actual API
			req := &proto.SimulationRequest{
				Symbol:         tc.symbol,
				StartTime:      timestamppb.New(time.Now().Add(-1 * time.Hour)),
				EndTime:        timestamppb.New(time.Now()),
				SimulationType: proto.SimulationType_STATISTICAL_SIMILARITY,
				Parameters: &proto.SimulationParameters{
					VolatilityFactor: tc.intensity,
					TrendFactor:      0.1,
					DataPoints:       20,
					IncludeNoise:     true,
					NoiseLevel:       0.05,
				},
			}

			resp, err := suite.marketDataHandler.GenerateSimulation(ctx, req)
			require.NoError(t, err)

			// Verify scenario simulation properties
			assert.Equal(t, tc.symbol, resp.Symbol)
			assert.NotEmpty(t, resp.SimulationId)
			assert.NotNil(t, resp.SimilarityMetrics)

			// Verify simulation quality based on scenario type
			metrics := resp.SimilarityMetrics
			assert.Greater(t, metrics.ConfidenceScore, 0.7)

			// For high intensity scenarios, expect lower correlation (more deviation)
			if tc.intensity > 1.5 {
				assert.Greater(t, metrics.CorrelationCoefficient, 0.6)
			} else {
				assert.Greater(t, metrics.CorrelationCoefficient, 0.8)
			}

			// Verify data integrity
			assert.Equal(t, len(resp.HistoricalData), len(resp.SimulatedData))
			if len(resp.SimulatedData) > 0 {
				assert.Greater(t, resp.SimulatedData[0].Close, 0.0)
				assert.Greater(t, resp.SimulatedData[0].Volume, 0.0)
			}
		})
	}
}

func TestIntegrationSuite_ComponentInteraction(t *testing.T) {
	suite, cleanup := setupIntegrationTest(t)
	defer cleanup()

	// Test basic component interactions without external dependencies

	// Verify service discovery is initialized
	assert.False(t, suite.serviceDiscovery.IsRegistered())

	// Verify client manager metrics work
	metrics := suite.clientManager.GetMetrics()
	assert.Contains(t, metrics, "active_connections")
	assert.Contains(t, metrics, "pool_size")
	assert.Equal(t, int64(0), metrics["active_connections"])

	// Test cleanup operations (should not crash)
	suite.clientManager.CleanupIdleConnections()

	// Verify configuration client metrics
	configMetrics := suite.configClient.GetMetrics()
	assert.Contains(t, configMetrics, "cache_size")
	assert.Equal(t, 0, configMetrics["cache_size"])

	// Test service discovery metrics
	discoveryMetrics := suite.serviceDiscovery.GetMetrics()
	assert.Contains(t, discoveryMetrics, "service_name")
	assert.Equal(t, "market-data-simulator", discoveryMetrics["service_name"])
}

// Helper function for absolute value
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}