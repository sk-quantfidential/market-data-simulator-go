package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/config"
	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/proto"
	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/services"
)

func setupHandler() *MarketDataGRPCHandler {
	cfg := &config.Config{
		ServiceName:    "market-data-simulator",
		ServiceVersion: "1.0.0",
		GRPCPort:       50051,
		HTTPPort:       8080,
		LogLevel:       "info",
		RedisURL:       "redis://localhost:6379",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce log noise in tests

	marketDataService := services.NewMarketDataService(cfg, logger)
	return NewMarketDataGRPCHandler(cfg, marketDataService, logger)
}

func TestMarketDataGRPCHandler_Creation(t *testing.T) {
	handler := setupHandler()

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.config)
	assert.NotNil(t, handler.logger)
	assert.NotNil(t, handler.marketDataService)
	assert.NotNil(t, handler.activeStreams)
}

func TestMarketDataGRPCHandler_GetPrice(t *testing.T) {
	handler := setupHandler()
	ctx := context.Background()

	req := &proto.GetPriceRequest{
		Symbol: "BTC/USD",
	}

	resp, err := handler.GetPrice(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "BTC/USD", resp.Symbol)
	assert.Greater(t, resp.Price, 0.0)
	assert.Equal(t, "market-data-simulator", resp.Source)
	assert.NotNil(t, resp.Timestamp)
}

func TestMarketDataGRPCHandler_GetPrice_DifferentSymbols(t *testing.T) {
	handler := setupHandler()
	ctx := context.Background()

	symbols := []string{"ETH/USD", "BTC/EUR", "ADA/BTC", "INVALID_SYMBOL"}

	for _, symbol := range symbols {
		req := &proto.GetPriceRequest{Symbol: symbol}
		resp, err := handler.GetPrice(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, symbol, resp.Symbol)
		assert.Greater(t, resp.Price, 0.0)
	}
}

func TestMarketDataGRPCHandler_HealthCheck(t *testing.T) {
	handler := setupHandler()
	ctx := context.Background()

	req := &proto.HealthCheckRequest{
		Service: "market-data",
	}

	resp, err := handler.HealthCheck(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, proto.HealthStatus_SERVING, resp.Status)
	assert.Contains(t, resp.Message, "healthy")
	assert.NotNil(t, resp.Timestamp)
	assert.NotNil(t, resp.Details)
	assert.Equal(t, "market-data-simulator", resp.Details["service_name"])
	assert.Equal(t, "1.0.0", resp.Details["service_version"])
}

func TestMarketDataGRPCHandler_GenerateSimulation(t *testing.T) {
	handler := setupHandler()
	ctx := context.Background()

	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	req := &proto.SimulationRequest{
		Symbol:         "BTC/USD",
		StartTime:      timestamppb.New(startTime),
		EndTime:        timestamppb.New(endTime),
		SimulationType: proto.SimulationType_STATISTICAL_SIMILARITY,
		Parameters: &proto.SimulationParameters{
			VolatilityFactor: 1.2,
			TrendFactor:      0.1,
			DataPoints:       100,
			IncludeNoise:     true,
			NoiseLevel:       0.05,
		},
	}

	resp, err := handler.GenerateSimulation(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "BTC/USD", resp.Symbol)
	assert.NotEmpty(t, resp.SimulationId)
	assert.NotNil(t, resp.HistoricalData)
	assert.NotNil(t, resp.SimulatedData)
	assert.NotNil(t, resp.SimilarityMetrics)

	// Verify similarity metrics are realistic
	metrics := resp.SimilarityMetrics
	assert.Greater(t, metrics.CorrelationCoefficient, 0.8)
	assert.Less(t, metrics.CorrelationCoefficient, 1.0)
	assert.Greater(t, metrics.VolatilitySimilarity, 0.7)
	assert.Less(t, metrics.VolatilitySimilarity, 1.0)
	assert.Greater(t, metrics.ConfidenceScore, 0.7)
	assert.Less(t, metrics.ConfidenceScore, 1.0)

	// Verify historical and simulated data have same length
	assert.Equal(t, len(resp.HistoricalData), len(resp.SimulatedData))

	// Verify data structure
	if len(resp.HistoricalData) > 0 {
		historical := resp.HistoricalData[0]
		assert.NotNil(t, historical.Timestamp)
		assert.Greater(t, historical.Close, 0.0)
		assert.Greater(t, historical.Volume, 0.0)
	}

	if len(resp.SimulatedData) > 0 {
		simulated := resp.SimulatedData[0]
		assert.NotNil(t, simulated.Timestamp)
		assert.Greater(t, simulated.Close, 0.0)
		assert.Greater(t, simulated.Volume, 0.0)
	}
}

func TestMarketDataGRPCHandler_GenerateSimulation_DifferentTypes(t *testing.T) {
	handler := setupHandler()
	ctx := context.Background()

	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now()

	simulationTypes := []proto.SimulationType{
		proto.SimulationType_STATISTICAL_SIMILARITY,
		proto.SimulationType_MONTE_CARLO,
		proto.SimulationType_BROWNIAN_MOTION,
		proto.SimulationType_MEAN_REVERSION,
		proto.SimulationType_TREND_FOLLOWING,
	}

	for _, simType := range simulationTypes {
		req := &proto.SimulationRequest{
			Symbol:         "ETH/USD",
			StartTime:      timestamppb.New(startTime),
			EndTime:        timestamppb.New(endTime),
			SimulationType: simType,
			Parameters: &proto.SimulationParameters{
				VolatilityFactor: 1.0,
				TrendFactor:      0.0,
				DataPoints:       10,
			},
		}

		resp, err := handler.GenerateSimulation(ctx, req)

		require.NoError(t, err, "Failed for simulation type: %v", simType)
		assert.NotNil(t, resp)
		assert.Equal(t, "ETH/USD", resp.Symbol)
		assert.NotEmpty(t, resp.SimulationId)
	}
}

func TestMarketDataGRPCHandler_GeneratePriceUpdate(t *testing.T) {
	handler := setupHandler()

	originalPrice := 50000.0
	session := &StreamSession{
		symbols:        []string{"BTC/USD"},
		updateInterval: 1 * time.Second,
		lastPrices:     map[string]float64{"BTC/USD": originalPrice},
		startTime:      time.Now(),
	}

	update := handler.generatePriceUpdate("BTC/USD", session)

	assert.NotNil(t, update)
	assert.Equal(t, "BTC/USD", update.Symbol)
	assert.Greater(t, update.Price, 0.0)
	assert.Greater(t, update.Volume, 0.0)
	assert.Equal(t, "market-data-simulator", update.Source)
	assert.NotNil(t, update.Timestamp)
	assert.NotNil(t, update.ChangeInfo)

	// Verify price stayed within reasonable range (±0.5% of original price)
	priceChange := (update.Price - originalPrice) / originalPrice
	assert.Greater(t, priceChange, -0.01) // Not more than 1% down
	assert.Less(t, priceChange, 0.01)     // Not more than 1% up

	// Verify change info is calculated correctly
	expectedChange := update.Price - originalPrice
	expectedChangePercent := (expectedChange / originalPrice) * 100
	assert.InDelta(t, expectedChange, update.ChangeInfo.ChangeAmount, 0.001)
	assert.InDelta(t, expectedChangePercent, update.ChangeInfo.ChangePercentage, 0.001)

	// Verify the session was updated with the new price
	assert.Equal(t, update.Price, session.lastPrices["BTC/USD"])
}

func TestMarketDataGRPCHandler_GenerateScenarioPrice(t *testing.T) {
	handler := setupHandler()

	basePrice := 100.0
	startTime := time.Now()
	endTime := startTime.Add(10 * time.Minute)
	currentTime := startTime.Add(5 * time.Minute) // Halfway through

	scenarios := []proto.ScenarioType{
		proto.ScenarioType_RALLY,
		proto.ScenarioType_CRASH,
		proto.ScenarioType_DIVERGENCE,
		proto.ScenarioType_MEAN_REVERTING,
		proto.ScenarioType_VOLATILITY_SPIKE,
		proto.ScenarioType_CONSOLIDATION,
	}

	for _, scenario := range scenarios {
		params := &proto.ScenarioParameters{
			Intensity:         1.5,
			DurationFactor:    1.0,
			RecoveryFactor:    0.5,
			GradualTransition: true,
		}

		update := handler.generateScenarioPrice("TEST/USD", scenario, params, basePrice, currentTime, startTime, endTime)

		assert.NotNil(t, update, "Failed for scenario: %v", scenario)
		assert.Equal(t, "TEST/USD", update.Symbol)
		assert.Greater(t, update.Price, 0.0)
		assert.Greater(t, update.Volume, 0.0)
		assert.Equal(t, "scenario-simulator", update.Source)
		assert.NotNil(t, update.ChangeInfo)

		// For specific scenarios, verify price direction
		switch scenario {
		case proto.ScenarioType_RALLY:
			assert.GreaterOrEqual(t, update.Price, basePrice, "Rally should increase price")
		case proto.ScenarioType_CRASH:
			assert.LessOrEqual(t, update.Price, basePrice, "Crash should decrease price")
		}
	}
}

func TestMarketDataGRPCHandler_CalculateSimilarityMetrics(t *testing.T) {
	handler := setupHandler()

	// Create sample historical data
	historical := []*proto.PricePoint{
		{Close: 100.0, Volume: 1000},
		{Close: 101.0, Volume: 1100},
		{Close: 99.5, Volume: 950},
		{Close: 102.0, Volume: 1200},
	}

	// Create sample simulated data
	simulated := []*proto.PricePoint{
		{Close: 100.2, Volume: 1020},
		{Close: 100.8, Volume: 1080},
		{Close: 99.3, Volume: 970},
		{Close: 101.8, Volume: 1180},
	}

	metrics := handler.calculateSimilarityMetrics(historical, simulated)

	assert.NotNil(t, metrics)
	assert.Greater(t, metrics.CorrelationCoefficient, 0.7)
	assert.Less(t, metrics.CorrelationCoefficient, 1.0)
	assert.Greater(t, metrics.VolatilitySimilarity, 0.7)
	assert.Less(t, metrics.VolatilitySimilarity, 1.0)
	assert.Greater(t, metrics.ReturnDistributionSimilarity, 0.7)
	assert.Less(t, metrics.ReturnDistributionSimilarity, 1.0)
	assert.Greater(t, metrics.TrendSimilarity, 0.7)
	assert.Less(t, metrics.TrendSimilarity, 1.0)
	assert.Greater(t, metrics.ConfidenceScore, 0.7)
	assert.Less(t, metrics.ConfidenceScore, 1.0)
}

func TestMarketDataGRPCHandler_CalculateSimilarityMetrics_EmptyData(t *testing.T) {
	handler := setupHandler()

	// Test with empty data
	metrics := handler.calculateSimilarityMetrics([]*proto.PricePoint{}, []*proto.PricePoint{})

	assert.NotNil(t, metrics)
	assert.Equal(t, 0.0, metrics.CorrelationCoefficient)
	assert.Equal(t, 0.0, metrics.VolatilitySimilarity)
	assert.Equal(t, 0.0, metrics.ReturnDistributionSimilarity)
	assert.Equal(t, 0.0, metrics.TrendSimilarity)
	assert.Equal(t, 0.0, metrics.ConfidenceScore)
}
