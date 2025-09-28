package handlers

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/config"
	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/proto"
	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/services"
)

type MarketDataGRPCHandler struct {
	proto.UnimplementedMarketDataServiceServer
	config            *config.Config
	logger            *logrus.Logger
	marketDataService *services.MarketDataService
	activeStreams     map[string]*StreamSession
	streamsMutex      sync.RWMutex
}

type StreamSession struct {
	symbols       []string
	updateInterval time.Duration
	ctx           context.Context
	cancel        context.CancelFunc
	lastPrices    map[string]float64
	startTime     time.Time
}

func NewMarketDataGRPCHandler(cfg *config.Config, marketDataService *services.MarketDataService, logger *logrus.Logger) *MarketDataGRPCHandler {
	return &MarketDataGRPCHandler{
		config:            cfg,
		logger:            logger,
		marketDataService: marketDataService,
		activeStreams:     make(map[string]*StreamSession),
	}
}

func (h *MarketDataGRPCHandler) GetPrice(ctx context.Context, req *proto.GetPriceRequest) (*proto.GetPriceResponse, error) {
	h.logger.WithField("symbol", req.Symbol).Info("GetPrice request received")

	price, err := h.marketDataService.GetPrice(req.Symbol)
	if err != nil {
		h.logger.WithError(err).WithField("symbol", req.Symbol).Error("Failed to get price")
		return nil, err
	}

	return &proto.GetPriceResponse{
		Symbol:    req.Symbol,
		Price:     price,
		Timestamp: timestamppb.Now(),
		Source:    "market-data-simulator",
	}, nil
}

func (h *MarketDataGRPCHandler) StreamPrices(req *proto.StreamPricesRequest, stream proto.MarketDataService_StreamPricesServer) error {
	sessionID := fmt.Sprintf("stream_%d", time.Now().UnixNano())
	ctx, cancel := context.WithCancel(stream.Context())

	updateInterval := time.Duration(req.UpdateIntervalMs) * time.Millisecond
	if updateInterval < 100*time.Millisecond {
		updateInterval = 100 * time.Millisecond // Minimum 100ms
	}

	session := &StreamSession{
		symbols:        req.Symbols,
		updateInterval: updateInterval,
		ctx:           ctx,
		cancel:        cancel,
		lastPrices:    make(map[string]float64),
		startTime:     time.Now(),
	}

	h.streamsMutex.Lock()
	h.activeStreams[sessionID] = session
	h.streamsMutex.Unlock()

	defer func() {
		h.streamsMutex.Lock()
		delete(h.activeStreams, sessionID)
		h.streamsMutex.Unlock()
		cancel()
	}()

	h.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
		"symbols":    req.Symbols,
		"interval":   updateInterval,
	}).Info("Starting price stream")

	// Initialize last prices
	for _, symbol := range req.Symbols {
		price, err := h.marketDataService.GetPrice(symbol)
		if err != nil {
			h.logger.WithError(err).WithField("symbol", symbol).Warn("Failed to get initial price")
			price = 100.0 // Default price
		}
		session.lastPrices[symbol] = price
	}

	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.logger.WithField("session_id", sessionID).Info("Stream context cancelled")
			return ctx.Err()
		case <-ticker.C:
			for _, symbol := range req.Symbols {
				priceUpdate := h.generatePriceUpdate(symbol, session)
				if err := stream.Send(priceUpdate); err != nil {
					h.logger.WithError(err).WithField("session_id", sessionID).Error("Failed to send price update")
					return err
				}
			}
		}
	}
}

func (h *MarketDataGRPCHandler) GenerateSimulation(ctx context.Context, req *proto.SimulationRequest) (*proto.SimulationResponse, error) {
	h.logger.WithFields(logrus.Fields{
		"symbol":          req.Symbol,
		"simulation_type": req.SimulationType,
		"start_time":      req.StartTime,
		"end_time":        req.EndTime,
	}).Info("GenerateSimulation request received")

	// Generate historical data (mock)
	historicalData := h.generateHistoricalData(req.Symbol, req.StartTime.AsTime(), req.EndTime.AsTime())

	// Generate simulated data based on simulation type
	simulatedData := h.generateSimulatedData(historicalData, req.SimulationType, req.Parameters)

	// Calculate similarity metrics
	metrics := h.calculateSimilarityMetrics(historicalData, simulatedData)

	simulationID := fmt.Sprintf("sim_%s_%d", req.Symbol, time.Now().Unix())

	return &proto.SimulationResponse{
		Symbol:           req.Symbol,
		HistoricalData:   historicalData,
		SimulatedData:    simulatedData,
		SimilarityMetrics: metrics,
		SimulationId:     simulationID,
	}, nil
}

func (h *MarketDataGRPCHandler) StreamScenario(req *proto.ScenarioRequest, stream proto.MarketDataService_StreamScenarioServer) error {
	h.logger.WithFields(logrus.Fields{
		"symbol":        req.Symbol,
		"scenario_type": req.ScenarioType,
		"duration":      req.DurationMinutes,
	}).Info("Starting scenario stream")

	ctx := stream.Context()
	startTime := req.StartTime.AsTime()
	duration := time.Duration(req.DurationMinutes) * time.Minute
	endTime := startTime.Add(duration)

	// Get base price
	basePrice, err := h.marketDataService.GetPrice(req.Symbol)
	if err != nil {
		basePrice = 100.0
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	currentTime := startTime
	for currentTime.Before(endTime) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			priceUpdate := h.generateScenarioPrice(req.Symbol, req.ScenarioType, req.Parameters, basePrice, currentTime, startTime, endTime)
			if err := stream.Send(priceUpdate); err != nil {
				return err
			}
			currentTime = currentTime.Add(1 * time.Second)
		}
	}

	return nil
}

func (h *MarketDataGRPCHandler) HealthCheck(ctx context.Context, req *proto.HealthCheckRequest) (*proto.HealthCheckResponse, error) {
	status := proto.HealthStatus_SERVING
	message := "Market Data Service is healthy"

	details := map[string]string{
		"service_name":    h.config.ServiceName,
		"service_version": h.config.ServiceVersion,
		"active_streams":  fmt.Sprintf("%d", len(h.activeStreams)),
	}

	return &proto.HealthCheckResponse{
		Status:    status,
		Message:   message,
		Timestamp: timestamppb.Now(),
		Details:   details,
	}, nil
}

func (h *MarketDataGRPCHandler) generatePriceUpdate(symbol string, session *StreamSession) *proto.PriceUpdate {
	lastPrice := session.lastPrices[symbol]

	// Generate realistic price movement (within 0.5% range)
	changePercent := (rand.Float64() - 0.5) * 0.01 // -0.5% to +0.5%
	newPrice := lastPrice * (1 + changePercent)

	// Generate volume (between 1000 and 10000)
	volume := 1000 + rand.Float64()*9000

	changeAmount := newPrice - lastPrice
	changePercentage := (changeAmount / lastPrice) * 100

	session.lastPrices[symbol] = newPrice

	return &proto.PriceUpdate{
		Symbol:    symbol,
		Price:     newPrice,
		Volume:    volume,
		Timestamp: timestamppb.Now(),
		Source:    "market-data-simulator",
		ChangeInfo: &proto.PriceChangeInfo{
			ChangeAmount:     changeAmount,
			ChangePercentage: changePercentage,
			DailyHigh:        newPrice * 1.02,
			DailyLow:         newPrice * 0.98,
			DailyVolume:      volume * 100,
		},
	}
}

func (h *MarketDataGRPCHandler) generateHistoricalData(symbol string, start, end time.Time) []*proto.PricePoint {
	var data []*proto.PricePoint
	basePrice := 100.0
	current := start

	for current.Before(end) {
		// Simple random walk for historical data
		change := (rand.Float64() - 0.5) * 0.02 // ±1%
		basePrice *= (1 + change)

		data = append(data, &proto.PricePoint{
			Timestamp: timestamppb.New(current),
			Open:      basePrice,
			High:      basePrice * 1.005,
			Low:       basePrice * 0.995,
			Close:     basePrice,
			Volume:    1000 + rand.Float64()*5000,
		})

		current = current.Add(1 * time.Hour)
	}

	return data
}

func (h *MarketDataGRPCHandler) generateSimulatedData(historicalData []*proto.PricePoint, simType proto.SimulationType, params *proto.SimulationParameters) []*proto.PricePoint {
	var simulatedData []*proto.PricePoint

	volatilityFactor := 1.0
	if params != nil {
		volatilityFactor = params.VolatilityFactor
	}

	for i, historical := range historicalData {
		// Apply simulation type logic
		var simulatedPrice float64
		switch simType {
		case proto.SimulationType_STATISTICAL_SIMILARITY:
			// Add some noise while maintaining statistical properties
			noise := (rand.Float64() - 0.5) * 0.01 * volatilityFactor
			simulatedPrice = historical.Close * (1 + noise)
		case proto.SimulationType_MONTE_CARLO:
			// More complex Monte Carlo simulation
			drift := 0.001
			diffusion := 0.02 * volatilityFactor
			simulatedPrice = historical.Close * math.Exp(drift + diffusion*rand.NormFloat64())
		default:
			simulatedPrice = historical.Close
		}

		simulatedData = append(simulatedData, &proto.PricePoint{
			Timestamp: historical.Timestamp,
			Open:      simulatedPrice,
			High:      simulatedPrice * 1.005,
			Low:       simulatedPrice * 0.995,
			Close:     simulatedPrice,
			Volume:    historical.Volume * (0.8 + rand.Float64()*0.4), // ±20% volume variation
		})

		// Add trend if specified
		if params != nil && i > 0 {
			trend := params.TrendFactor * 0.001
			simulatedData[i].Close *= (1 + trend)
		}
	}

	return simulatedData
}

func (h *MarketDataGRPCHandler) generateScenarioPrice(symbol string, scenarioType proto.ScenarioType, params *proto.ScenarioParameters, basePrice float64, currentTime, startTime, endTime time.Time) *proto.PriceUpdate {
	progress := float64(currentTime.Sub(startTime)) / float64(endTime.Sub(startTime))

	intensity := 1.0
	if params != nil {
		intensity = params.Intensity
	}

	var priceMultiplier float64 = 1.0

	switch scenarioType {
	case proto.ScenarioType_RALLY:
		// Exponential growth that slows down over time
		priceMultiplier = 1.0 + (intensity-1.0)*math.Pow(progress, 0.5)
	case proto.ScenarioType_CRASH:
		// Sharp decline that levels off
		priceMultiplier = 1.0 - (intensity-1.0)*progress*0.5
	case proto.ScenarioType_DIVERGENCE:
		// Oscillating pattern
		priceMultiplier = 1.0 + (intensity-1.0)*0.1*math.Sin(progress*math.Pi*4)
	case proto.ScenarioType_MEAN_REVERTING:
		// Returns to baseline over time
		deviation := (intensity - 1.0) * 0.2 * math.Sin(progress*math.Pi*2)
		priceMultiplier = 1.0 + deviation*math.Exp(-progress*3)
	}

	finalPrice := basePrice * priceMultiplier
	volume := 1000 + rand.Float64()*9000*intensity

	return &proto.PriceUpdate{
		Symbol:    symbol,
		Price:     finalPrice,
		Volume:    volume,
		Timestamp: timestamppb.New(currentTime),
		Source:    "scenario-simulator",
		ChangeInfo: &proto.PriceChangeInfo{
			ChangeAmount:     finalPrice - basePrice,
			ChangePercentage: ((finalPrice - basePrice) / basePrice) * 100,
			DailyHigh:        finalPrice * 1.02,
			DailyLow:         finalPrice * 0.98,
			DailyVolume:      volume * 100,
		},
	}
}

func (h *MarketDataGRPCHandler) calculateSimilarityMetrics(historical, simulated []*proto.PricePoint) *proto.StatisticalMetrics {
	if len(historical) == 0 || len(simulated) == 0 {
		return &proto.StatisticalMetrics{
			CorrelationCoefficient:        0.0,
			VolatilitySimilarity:         0.0,
			ReturnDistributionSimilarity: 0.0,
			TrendSimilarity:              0.0,
			ConfidenceScore:              0.0,
		}
	}

	// Calculate simple correlation (mock implementation)
	correlation := 0.85 + rand.Float64()*0.1 // 0.85-0.95
	volatilitySimilarity := 0.80 + rand.Float64()*0.15 // 0.80-0.95
	returnSimilarity := 0.75 + rand.Float64()*0.20 // 0.75-0.95
	trendSimilarity := 0.82 + rand.Float64()*0.13 // 0.82-0.95

	confidenceScore := (correlation + volatilitySimilarity + returnSimilarity + trendSimilarity) / 4.0

	return &proto.StatisticalMetrics{
		CorrelationCoefficient:        correlation,
		VolatilitySimilarity:         volatilitySimilarity,
		ReturnDistributionSimilarity: returnSimilarity,
		TrendSimilarity:              trendSimilarity,
		ConfidenceScore:              confidenceScore,
	}
}