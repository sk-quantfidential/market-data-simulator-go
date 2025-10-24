# Market Data Simulator

A high-performance market data simulator built in Go that ingests real market data feeds and provides controllable price manipulation, scenario injection, and realistic market behavior simulation for comprehensive trading system testing.

## 🎯 Overview

The Market Data Simulator serves as an intelligent proxy between real market data providers (CoinGecko, CMC, exchange websockets) and the trading ecosystem. It maintains real market behavior while enabling controlled chaos injection for testing price shocks, divergences, depegging events, and feed disruptions that are impossible to reliably test with live data.

### Key Features
- **Real Data Ingestion**: Connects to multiple market data sources with automatic failover
- **Intelligent Replay**: Historical data replay with configurable speed and timing
- **Price Manipulation**: Inject gradual divergences, sudden shocks, and correlation breaks
- **Feed Disruption**: Simulate stale data, latency spikes, and complete outages
- **Multi-Asset Support**: BTC/USD, USDT/BTC, USDT/ETH, ETH/USD, BTC/ETH with cross-asset correlation
- **Scenario Orchestration**: Complex multi-phase scenarios (depeg events, market crashes)
- **High-Frequency Publishing**: Sub-millisecond price updates with realistic market microstructure

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────┐
│                Market Data Simulator                    │
├─────────────────────────────────────────────────────────┤
│  Data Sources           │  gRPC Services                │
│  ├─CoinGecko API        │  ├─Price Feed Service         │
│  ├─CMC WebSocket        │  ├─Historical Data Service    │
│  ├─Exchange Feeds       │  └─Market Statistics Service  │
│  └─Historical DB        │                               │
├─────────────────────────────────────────────────────────┤
│  Core Engine                                            │
│  ├─Price Aggregator (Multi-source consensus)           │
│  ├─Scenario Engine (Chaos injection orchestration)     │
│  ├─Publication Manager (High-frequency broadcasting)   │
│  ├─Correlation Tracker (Cross-asset relationships)     │
│  └─Circuit Breaker (Abnormal price detection)          │
├─────────────────────────────────────────────────────────┤
│  Chaos Controllers                                      │
│  ├─Price Manipulator (Gradual/sudden price changes)    │
│  ├─Feed Disruptor (Latency, stale data, outages)      │
│  ├─Volatility Injector (Artificial volatility spikes)  │
│  └─Correlation Breaker (Cross-asset divergences)       │
├─────────────────────────────────────────────────────────┤
│  Data Layer                                             │
│  ├─Real-time Prices (In-memory ring buffers)          │
│  ├─Historical OHLCV (Time-series database)            │
│  ├─Scenario Definitions (YAML configurations)         │
│  └─Market Microstructure (Bid/ask spreads, volumes)   │
└─────────────────────────────────────────────────────────┘
```

## 🚀 Quick Start

### Prerequisites
- Go 1.21+
- Docker and Docker Compose
- Protocol Buffers compiler
- Market data API keys (CoinGecko, CMC)

### Development Setup
```bash
# Clone the repository
git clone <repo-url>
cd market-data-simulator

# Install dependencies
go mod download

# Generate protobuf files
make generate-proto

# Configure API keys
cp config/config.example.yaml config/config.yaml
# Edit config.yaml with your API keys

# Run tests
make test

# Start development server
make run-dev
```

### Docker Deployment
```bash
# Build container
docker build -t market-data-simulator .

# Run with docker-compose (recommended)
docker-compose up market-data-simulator

# Verify health and data flow
curl http://localhost:8082/health
curl http://localhost:8082/api/v1/prices/BTC-USD/current
```

## 📡 API Reference

### gRPC Services

#### Price Feed Service
```protobuf
service PriceFeedService {
  rpc SubscribePrices(SubscribeRequest) returns (stream PriceUpdate);
  rpc GetCurrentPrice(PriceRequest) returns (Price);
  rpc GetPriceHistory(HistoryRequest) returns (PriceHistoryResponse);
  rpc GetMarketDepth(MarketDepthRequest) returns (MarketDepthResponse);
}
```

#### Historical Data Service
```protobuf
service HistoricalDataService {
  rpc GetOHLCV(OHLCVRequest) returns (OHLCVResponse);
  rpc GetVolumeProfile(VolumeProfileRequest) returns (VolumeProfileResponse);
  rpc GetCorrelationMatrix(CorrelationRequest) returns (CorrelationResponse);
}
```

### REST Endpoints

#### Production APIs (Risk Monitor & Trading Systems)
```
GET    /api/v1/prices/{symbol}/current
GET    /api/v1/prices/{symbol}/history?start=&end=&interval=
GET    /api/v1/orderbook/{symbol}
GET    /api/v1/stats/{symbol}/24h
GET    /api/v1/correlations?symbols=BTC-USD,ETH-USD
POST   /api/v1/subscribe (WebSocket endpoint)
```

#### Chaos Engineering APIs (Audit Only)
```
POST   /chaos/price-shock
POST   /chaos/gradual-divergence
POST   /chaos/feed-disruption
POST   /chaos/stablecoin-depeg
POST   /chaos/correlation-break
POST   /chaos/volatility-spike
GET    /chaos/active-scenarios
DELETE /chaos/stop-scenario/{scenario_id}
```

#### State Inspection APIs (Development/Audit)
```
GET    /debug/data-sources/status
GET    /debug/price-buffers
GET    /debug/scenario-timeline
GET    /debug/correlation-matrix
GET    /metrics (Prometheus format)
```

## 📊 Real Data Integration

### Data Source Configuration
```yaml
data_sources:
  primary:
    - name: "coingecko"
      type: "rest_api"
      url: "https://api.coingecko.com/api/v3"
      api_key: "${COINGECKO_API_KEY}"
      rate_limit: 50  # requests per minute
      priority: 1
    
  secondary:
    - name: "coinmarketcap"
      type: "websocket"
      url: "wss://stream.coinmarketcap.com/ws"
      api_key: "${CMC_API_KEY}"
      priority: 2
    
  exchange_feeds:
    - name: "binance"
      type: "websocket"
      url: "wss://stream.binance.com:9443/ws"
      symbols: ["btcusdt", "ethusdt", "btceth"]
      priority: 3
```

### Data Aggregation Strategy
```
Price Consensus Algorithm:
1. Collect prices from all available sources
2. Remove outliers beyond 2 standard deviations
3. Calculate weighted average based on source priority and volume
4. Apply smoothing filter to reduce noise
5. Validate against circuit breaker thresholds
6. Publish final price with confidence score
```

### Failover & Reliability
- **Automatic Failover**: Switch to backup sources on primary failure
- **Data Quality Checks**: Validate prices against expected ranges and correlations
- **Circuit Breakers**: Halt publishing on abnormal price movements (>20% in 1 minute)
- **Staleness Detection**: Alert when data sources become stale (>30 seconds)

## 🎭 Chaos Engineering Scenarios

### Price Shock Injection
```bash
# Sudden 15% BTC price drop over 5 minutes
curl -X POST localhost:8082/chaos/price-shock \
  -d '{
    "symbol": "BTC-USD",
    "shock_percentage": -15.0,
    "duration_seconds": 300,
    "pattern": "sudden_drop",
    "recovery_time_seconds": 1800
  }'
```

### Gradual Stablecoin Depeg
```bash
# USDT slowly depegs from USD over 36 hours
curl -X POST localhost:8082/chaos/stablecoin-depeg \
  -d '{
    "symbol": "USDT-USD",
    "target_deviation": -0.05,
    "duration_hours": 36,
    "pattern": "gradual_linear",
    "volatility_increase": 10.0
  }'
```

### Cross-Asset Divergence
```bash
# BTC spot vs futures divergence
curl -X POST localhost:8082/chaos/gradual-divergence \
  -d '{
    "base_symbol": "BTC-USD",
    "target_symbol": "BTC-USD-PERP",
    "max_spread_bps": 500,
    "buildup_hours": 4,
    "sustain_hours": 8,
    "recovery_hours": 2
  }'
```

### Feed Disruption
```bash
# Simulate 20% packet loss and 2s latency on primary feed
curl -X POST localhost:8082/chaos/feed-disruption \
  -d '{
    "source": "coingecko",
    "packet_loss_percentage": 20,
    "latency_ms": 2000,
    "duration_seconds": 3600,
    "affect_symbols": ["BTC-USD", "ETH-USD"]
  }'
```

### Market Crash Scenario
```bash
# Coordinated crash across all major assets
curl -X POST localhost:8082/chaos/market-crash \
  -d '{
    "crash_percentage": -15.0,
    "crash_duration_minutes": 30,
    "symbols": ["BTC-USD", "ETH-USD", "BTC-ETH"],
    "correlation_increase": 0.95,
    "volume_spike_multiplier": 5.0,
    "recovery_pattern": "slow_bounce"
  }'
```

### Volatility Injection
```bash
# 10x normal volatility for 2 hours
curl -X POST localhost:8082/chaos/volatility-spike \
  -d '{
    "symbol": "BTC-USD",
    "volatility_multiplier": 10.0,
    "duration_hours": 2,
    "pattern": "random_walk",
    "preserve_trend": true
  }'
```

## 📈 Market Microstructure Simulation

### Bid-Ask Spread Modeling
```go
type SpreadModel struct {
    BaseSpreadBPS     float64  // Base spread in basis points
    VolatilityFactor  float64  // Spread widens with volatility
    LiquidityFactor   float64  // Spread narrows with volume
    TimeOfDayFactor   float64  // Spreads wider during off-hours
    AssetSpecific     float64  // Asset-specific spread characteristics
}

// Example spreads:
// BTC/USD: 1-5 bps (highly liquid)
// ETH/USD: 2-8 bps (good liquidity)
// BTC/ETH: 5-15 bps (cross-rate)
// USDT/USD: 0.1-2 bps (stablecoin)
```

### Volume Profile Generation
- **Realistic Volume**: Based on historical patterns and current market conditions
- **Time-of-Day Effects**: Higher volume during overlapping trading sessions
- **Volatility Correlation**: Volume spikes during price movements
- **Weekend Effects**: Reduced volume during crypto weekends

### Order Book Simulation
```
Depth Level    BTC/USD Example
├── Level 1   │ $44,998.50 (2.5 BTC) | $45,001.50 (1.8 BTC)
├── Level 2   │ $44,995.00 (5.2 BTC) | $45,005.00 (4.1 BTC)
├── Level 3   │ $44,990.00 (8.7 BTC) | $45,010.00 (6.3 BTC)
└── Level 10  │ $44,950.00 (25 BTC)  | $45,050.00 (22 BTC)
```

## ⏱️ High-Frequency Publishing

### Publication Frequencies
```yaml
publication_config:
  tick_data:
    frequency: "100ms"    # 10 updates per second
    symbols: ["BTC-USD", "ETH-USD"]
    include_volume: true
  
  ohlcv_data:
    intervals: ["1m", "5m", "15m", "1h", "4h", "1d"]
    lag_tolerance: "5s"   # Maximum delay before publishing
  
  market_depth:
    frequency: "1s"       # Order book snapshots
    depth_levels: 10
```

### Performance Optimizations
- **Ring Buffers**: Lock-free circular buffers for price history
- **Batch Publishing**: Group updates to reduce network overhead
- **Compression**: Protobuf compression for high-frequency streams
- **Connection Pooling**: Efficient client connection management

## 📊 Monitoring & Observability

### Prometheus Metrics
```
# Data ingestion metrics
market_data_source_latency_ms{source="coingecko", symbol="BTC-USD"}
market_data_source_errors_total{source, error_type}
market_data_price_updates_total{symbol, source}

# Price quality metrics
market_data_price_deviation{symbol, source}  # Deviation from consensus
market_data_staleness_seconds{symbol}        # Time since last update
market_data_confidence_score{symbol}         # Price confidence (0-1)

# Chaos injection metrics
market_data_scenario_active{scenario_type, symbol}
market_data_price_manipulation_active{symbol, manipulation_type}
market_data_artificial_volatility{symbol}

# System performance
market_data_publication_latency_ms{symbol, client_type}
market_data_subscriber_count{symbol}
market_data_memory_usage_bytes{buffer_type}
```

### OpenTelemetry Tracing
- **Data Pipeline**: Complete trace from ingestion to publication
- **Chaos Injection**: Track scenario execution and price manipulation
- **Client Subscriptions**: Monitor subscription lifecycle and performance
- **Cross-Service**: Correlation with trading system price consumption

### Structured Logging
```json
{
  "timestamp": "2025-09-16T14:23:45.123Z",
  "level": "info",
  "service": "market-data-simulator",
  "correlation_id": "scenario-crash-001",
  "event": "price_manipulation_started",
  "symbol": "BTC-USD",
  "scenario_type": "price_shock",
  "original_price": "45000.00",
  "target_price": "38250.00",
  "manipulation_duration_s": 300,
  "recovery_duration_s": 1800
}
```

## 🧪 Testing

### Unit Tests
```bash
# Run all unit tests
make test

# Test price aggregation logic
go test ./internal/aggregator -v

# Test chaos injection scenarios
go test ./internal/chaos -v -run TestPriceShock
```

### Integration Tests
```bash
# Test with real data sources (requires API keys)
make test-integration

# Test scenario orchestration
make test-scenarios

# Load test with high-frequency publishing
make load-test-publishing
```

### Scenario Validation Tests
```bash
# Validate stablecoin depeg scenario
go test ./internal/scenarios -run TestStablecoinDepeg

# Test market crash with recovery
go test ./internal/scenarios -run TestMarketCrashRecovery

# Validate feed disruption handling
go test ./internal/scenarios -run TestFeedDisruption
```

## ⚙️ Configuration

### Environment Variables
```bash
# Core settings
MARKET_DATA_PORT=8082
MARKET_DATA_GRPC_PORT=50053
MARKET_DATA_LOG_LEVEL=info

# Data source API keys
COINGECKO_API_KEY=your_coingecko_key
COINMARKETCAP_API_KEY=your_cmc_key
BINANCE_API_KEY=your_binance_key  # Optional for public data

# Publication settings
TICK_FREQUENCY_MS=100
MAX_SUBSCRIBERS=1000
ENABLE_COMPRESSION=true

# Chaos engineering
CHAOS_ENABLED=true
MAX_PRICE_DEVIATION=0.25  # 25% maximum price manipulation
SCENARIO_TIMEOUT_HOURS=48
```

### Configuration File (config.yaml)
```yaml
market_data:
  supported_symbols:
    - symbol: "BTC-USD"
      base_asset: "BTC"
      quote_asset: "USD"
      decimals: 2
      min_price: 1000
      max_price: 200000
      circuit_breaker_threshold: 0.20
    
    - symbol: "USDT-USD"
      base_asset: "USDT"
      quote_asset: "USD"
      decimals: 4
      min_price: 0.90
      max_price: 1.10
      depeg_alert_threshold: 0.02

publication:
  tick_frequency_ms: 100
  batch_size: 50
  compression_enabled: true
  max_subscribers_per_symbol: 200

chaos_scenarios:
  price_shock:
    max_deviation: 0.25
    max_duration_hours: 2
    recovery_required: true
  
  stablecoin_depeg:
    max_deviation: 0.10
    max_duration_hours: 72
    gradual_only: true
```

## 🐳 Docker Configuration

### Dockerfile
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o market-data-simulator cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /app/market-data-simulator /usr/local/bin/
COPY --from=builder /app/config/config.yaml /etc/market-data/
EXPOSE 8080 50051
CMD ["market-data-simulator", "--config=/etc/market-data/config.yaml"]
```

### Health Checks
```yaml
healthcheck:
  test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8082/health"]
  interval: 15s
  timeout: 5s
  retries: 3
  start_period: 30s
```

## 🔒 Security & Rate Limiting

### API Key Management
- **Environment Variables**: Secure API key storage
- **Rotation**: Automatic API key rotation support
- **Rate Limiting**: Respect data source rate limits
- **Circuit Breakers**: Prevent API quota exhaustion

### Data Validation
- **Price Validation**: Sanity checks on incoming price data
- **Timestamp Validation**: Ensure data freshness and ordering
- **Source Validation**: Verify data source authenticity
- **Anomaly Detection**: Flag unusual price movements for review

## 🚀 Performance

### Benchmarks
- **Price Ingestion**: >1,000 price updates/second from multiple sources
- **Publication Latency**: <10ms from ingestion to client delivery
- **Subscriber Support**: >1,000 concurrent subscribers per symbol
- **Memory Usage**: <500MB with full price history buffers

### Scaling Considerations
- **Horizontal Scaling**: Symbol-based sharding for high load
- **Caching Strategy**: Multi-level caching for frequently accessed data
- **Connection Management**: Efficient WebSocket connection pooling
- **Data Retention**: Automatic cleanup of old price data

## 🤝 Contributing

### Development Workflow
1. Create feature branch from `main`
2. Implement changes with comprehensive tests
3. Test with real data sources using test API keys
4. Validate chaos scenarios work as expected
5. Update metrics and monitoring
6. Submit pull request with scenario examples

### Code Standards
- **Go Best Practices**: Follow standard Go project layout
- **Financial Precision**: Use decimal types for all price calculations
- **Thread Safety**: Ensure all price operations are thread-safe
- **Documentation**: Document all chaos scenarios with examples

## 📚 References

- **Market Data Standards**: [Link to financial data specifications]
- **Chaos Engineering**: [Link to chaos engineering best practices]
- **Protobuf Schemas**: [Link to market data API definitions]
- **Data Source APIs**: [Links to CoinGecko, CMC, exchange documentation]

---

**Status**: 🚧 Development Phase  
**Maintainer**: [Your team]  
**Last Updated**: September 2025
