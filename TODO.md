# market-data-simulator-go TODO

## epic-TSE-0001: Foundation Services & Infrastructure

### 🏗️ Milestone TSE-0001.1a: Go Services Bootstrapping
**Status**: ✅ COMPLETED
**Priority**: High

**Tasks**:
- [x] Create Go service directory structure following clean architecture
- [x] Implement health check endpoint (REST and gRPC)
- [x] Basic structured logging with levels
- [x] Error handling infrastructure
- [x] Dockerfile for service containerization
- [x] Load component-specific .claude configuration

**BDD Acceptance**: All Go services can start, respond to health checks, and shutdown gracefully

---

### 🔗 Milestone TSE-0001.3b: Go Services gRPC Integration
**Status**: ✅ COMPLETED (Following proven pattern from custodian-simulator-go & exchange-simulator-go)
**Priority**: High

**Tasks** (Following proven TDD Red-Green-Refactor cycle):
- [x] **Phase 1: TDD Red** - Create failing tests for market data gRPC integration with simulation behaviors
- [x] **Phase 2: Infrastructure** - Add Redis dependencies and update .gitignore for Go projects
- [x] **Phase 3: gRPC Server** - Enhanced server with health service, market data streaming, and metrics
- [x] **Phase 4: Configuration** - Configuration service client with HTTP caching, TTL, and market data parameters
- [x] **Phase 5: Discovery** - Service discovery with Redis-based registry, heartbeat, and cleanup
- [x] **Phase 6: Communication** - Inter-service client manager with connection pooling and circuit breaker
- [x] **Phase 7: Integration** - Comprehensive testing with market data scenarios and smart infrastructure detection
- [x] **Phase 8: Validation** - Verify BDD acceptance and complete milestone documentation

**Implementation Pattern** (Replicating proven success from other Go components):
- **Infrastructure Layer**: Configuration client, service discovery, gRPC clients
- **Presentation Layer**: Enhanced gRPC server with health service and market data streaming
- **Domain Layer**: Market data simulation engine with real data integration capability
- **Testing Strategy**: Unit tests with smart dependency skipping, integration tests for market data scenarios
- **Market Data Features**: Statistical similarity, scenario simulation (rally/crash/divergence/reverting), standard API

**BDD Acceptance**: Go services can discover and communicate with each other via gRPC, with market data streaming capabilities

**Dependencies**: TSE-0001.1a (Go Services Bootstrapping), TSE-0001.3a (Core Infrastructure)

**Reference Implementation**: custodian-simulator-go & exchange-simulator-go (✅ COMPLETED) - Use as pattern for architecture and testing

---

### 📊 Milestone TSE-0001.12b: Prometheus Metrics Client
**Status**: ✅ **COMPLETE** - Clean Architecture metrics implementation with BDD testing
**Goal**: Implement Prometheus metrics collection for observability using Clean Architecture patterns
**Pattern**: Following audit-correlator-go, custodian-simulator-go, and exchange-simulator-go proven approach
**Dependencies**: TSE-0001.3b (Go Services gRPC Integration) ✅
**Completed**: 2025-10-09

## ✅ What Was Completed

**Domain Layer** (Clean Architecture): ✅
- Created `MetricsPort` interface in `internal/domain/ports/metrics.go`
- Port abstraction enables future OpenTelemetry migration
- Zero infrastructure dependencies in domain layer

**Infrastructure Layer** (Prometheus Adapter): ✅
- Implemented `PrometheusMetricsAdapter` in `internal/infrastructure/observability/prometheus_adapter.go`
- Thread-safe lazy initialization with double-check locking pattern
- Separate Prometheus registry with Go runtime metrics
- Constant labels: service, instance, version
- Histogram buckets optimized for HTTP API latency (5ms to 10s)
- Dynamic metric registration (counters, histograms, gauges)

**Middleware** (RED Pattern): ✅
- Implemented `REDMetricsMiddleware` in `internal/infrastructure/observability/middleware.go`
- RED metrics: Rate (http_requests_total), Errors (http_request_errors_total), Duration (http_request_duration_seconds)
- Low cardinality labels: method, route pattern (not path), status code
- Unknown routes labeled as "unknown" to prevent metric explosion

**Presentation Layer** (Handler): ✅
- Created `MetricsHandler` in `internal/handlers/metrics.go`
- Exposes `/metrics` endpoint via MetricsPort interface
- Clean separation between business logic and observability

**Integration** (main.go): ✅
- Added observability initialization in `cmd/server/main.go`
- Metrics middleware applied to all HTTP routes
- `/metrics` endpoint exposed at root level
- Dependency injection following Clean Architecture

**Dependencies** (go.mod): ✅
- Added `github.com/prometheus/client_golang v1.23.2`
- All required Prometheus dependencies resolved

**BDD Testing** (8 scenarios, 8/8 passing): ✅
- **Handler Tests** (`internal/handlers/metrics_test.go`):
  - ✅ exposes_prometheus_metrics_through_port
  - ✅ returns_text_plain_content_type
  - ✅ includes_standard_go_runtime_metrics
  - ✅ includes_constant_labels_in_all_metrics

- **Middleware Tests** (`internal/infrastructure/observability/middleware_test.go`):
  - ✅ instruments_successful_requests_with_RED_metrics
  - ✅ instruments_error_requests_with_error_counter
  - ✅ uses_route_pattern_not_full_path
  - ✅ handles_unknown_routes_without_metric_explosion

## 📈 Metrics Exposed

**RED Pattern Metrics**:
- `http_requests_total{method, route, code, service, instance, version}` - Total HTTP requests
- `http_request_duration_seconds{method, route, code, service, instance, version}` - Request latency histogram
- `http_request_errors_total{method, route, code, service, instance, version}` - HTTP errors (4xx/5xx)

**Go Runtime Metrics**:
- `go_goroutines` - Current goroutine count
- `go_memstats_alloc_bytes` - Memory allocation
- `go_threads` - OS thread count
- `process_cpu_seconds_total` - CPU usage

## 🎯 BDD Acceptance Criteria
> market-data-simulator-go exposes Prometheus metrics at /metrics endpoint with RED pattern instrumentation

**Status**: ✅ Complete - All 8 BDD scenarios passing

## 📝 Files Modified/Created

**Created**:
- `internal/domain/ports/metrics.go` - MetricsPort interface (Clean Architecture)
- `internal/infrastructure/observability/prometheus_adapter.go` - Prometheus implementation (206 lines)
- `internal/infrastructure/observability/middleware.go` - RED metrics middleware (77 lines)
- `internal/handlers/metrics.go` - Metrics endpoint handler (29 lines)
- `internal/handlers/metrics_test.go` - Handler BDD tests (179 lines, 4 scenarios)
- `internal/infrastructure/observability/middleware_test.go` - Middleware BDD tests (228 lines, 4 scenarios)

**Modified**:
- `cmd/server/main.go` - Added metrics initialization and middleware
- `go.mod` - Added Prometheus dependencies
- `go.sum` - Dependency checksums

## 🔒 Low Cardinality Best Practices
✅ Route patterns used (not full paths): `/api/v1/prices/:symbol` not `/api/v1/prices/BTC`
✅ Unknown routes grouped as "unknown"
✅ Limited label values (method, route pattern, code)
✅ No user-provided data in labels

---

### 📊 Milestone TSE-0001.5: Market Data Foundation (PRIMARY)
**Status**: Not Started
**Priority**: CRITICAL - Enables trading and risk monitoring

**Tasks**:
- [ ] Minimal price feed generation for BTC/USD, ETH/USD
- [ ] REST API for current prices (production API)
- [ ] gRPC streaming interface for real-time feeds
- [ ] Basic price simulation with fixed spreads
- [ ] Simple volatility modeling
- [ ] Price history storage (Redis)
- [ ] Prometheus metrics for feed performance

**BDD Acceptance**: Risk Monitor can subscribe to price feeds and receive updates

**Dependencies**: TSE-0001.4 (Data Adapters & Orchestrator Refactoring)

---

### 🔗 Milestone TSE-0001.4.3: Data Adapters & Orchestrator Integration (Market Data)
**Status**: ✅ **COMPLETE** - Config layer integration and smoke tests passing
**Goal**: Integrate market-data-simulator-go with market-data-adapter-go and deploy to orchestrator
**Pattern**: Following audit-correlator-go, custodian-simulator-go, and exchange-simulator-go proven approach (Option A: Smoke Tests)
**Dependencies**: TSE-0001.3b (Go Services gRPC Integration) ✅
**Completed**: 2025-10-01 (Config Integration + Smoke Tests)

## ✅ What Was Completed

**Config Layer Integration**: ✅
- Added market-data-adapter-go dependency to go.mod with local replace directive
- Enhanced Config struct with DataAdapter integration
- Implemented InitializeDataAdapter, GetDataAdapter, DisconnectDataAdapter methods
- Added godotenv for .env file loading
- Updated ports to 8083 (HTTP), 9093 (gRPC) to avoid conflicts

**Smoke Tests**: ✅ (3/3 passing)
- Config load tests with defaults and environment variables ✅
- DataAdapter initialization with graceful degradation ✅
- Repository access validation (6 repositories) ✅

**Integration Pattern** (Following TSE-0001.4.2):
- Config layer handles DataAdapter lifecycle (Connect/Disconnect)
- Graceful degradation when PostgreSQL/Redis unavailable (stub mode)
- Repository access via Config.GetDataAdapter() for service layer

## 🚧 Future Work (Deferred to TSE-0001.5)

**Service Layer Integration** (Not Implemented Yet):
- [ ] PublishPrice using PriceFeedRepository
- [ ] PublishCandle using CandleRepository
- [ ] CreateSnapshot using MarketSnapshotRepository
- [ ] GetActiveSymbols using SymbolRepository

**Orchestrator Deployment** (Deferred):
- [ ] Create PostgreSQL market_data schema (4 tables)
- [ ] Create market-data-adapter ACL user in Redis
- [ ] Add market-data-simulator to docker-compose.yml
- [ ] Validate deployment on 172.20.0.83:8083/9093

## 🎯 BDD Acceptance Criteria (Partially Met)
> market-data-simulator-go uses market-data-adapter-go for all database operations via repository pattern

**Status**: ✅ Config layer integrated, ⏭️ Service layer deferred to TSE-0001.5

## 📋 Integration Task Checklist

### Task 0: Test Infrastructure Foundation
**Goal**: Ensure existing test infrastructure is ready for DataAdapter integration
**Estimated Time**: 30 minutes

#### Steps
- [ ] Verify Makefile has test automation targets (unit, integration, all)
- [ ] Ensure go.mod compiles successfully
- [ ] Confirm no JSON serialization issues (use `json.RawMessage` for metadata fields)
- [ ] Validate existing test coverage baseline
- [ ] Document current build and test status

**Validation**:
```bash
# Compile check
go build ./...

# Run existing tests
go test ./... -v

# Check test coverage
go test ./... -cover
```

**Acceptance Criteria**:
- [ ] Code compiles without errors
- [ ] Existing tests have baseline pass rate
- [ ] Test infrastructure (Makefile) ready for enhancement
- [ ] No critical build issues blocking integration

---

### Task 1: Create market-data-adapter-go Repository
**Goal**: Create new data adapter repository for market data domain operations
**Estimated Time**: 8-10 hours (see market-data-adapter-go/TODO.md for detailed tasks)

This task creates the foundation data adapter repository. See `market-data-adapter-go/TODO.md` for comprehensive implementation plan including:
- Repository structure and Go module setup
- Environment configuration with .env support
- Database schema (price_feeds, candles, market_snapshots, symbols tables)
- Repository interfaces (PriceFeed, Candle, MarketSnapshot, Symbol, ServiceDiscovery, Cache)
- PostgreSQL and Redis implementations
- DataAdapter factory pattern
- BDD behavior testing framework

**Acceptance Criteria**:
- [ ] market-data-adapter-go repository created with full structure
- [ ] All repository interfaces defined
- [ ] PostgreSQL implementation complete
- [ ] Redis implementation complete
- [ ] Comprehensive test suite (20+ scenarios, 80%+ pass rate)
- [ ] Build passing and tests validated
- [ ] Ready for integration with market-data-simulator-go

---

### Task 2: Refactor Infrastructure Layer
**Goal**: Replace direct database access with market-data-adapter-go repositories
**Estimated Time**: 2 hours

#### Files to Modify

**internal/infrastructure/service_discovery.go**:
- Replace direct Redis access with `DataAdapter.ServiceDiscoveryRepository`
- Use `RegisterService()`, `UpdateHeartbeat()`, `Deregister()` methods
- Implement graceful fallback (stub mode) when DataAdapter unavailable

**internal/infrastructure/configuration_client.go**:
- Replace local cache with `DataAdapter.CacheRepository`
- Use `Set()`, `Get()`, `DeleteByPattern()` for configuration caching
- Update cache stats using `GetKeysByPattern()`
- Maintain TTL management through DataAdapter

**internal/config/config.go**:
- Add `dataAdapter` field to Config struct
- Implement `InitializeDataAdapter(ctx, logger)` method
- Load DataAdapter from environment using `adapters.NewMarketDataAdapterFromEnv()`
- Add `GetDataAdapter()` method for service layer access
- Implement graceful degradation when connection fails

**cmd/server/main.go**:
- Initialize DataAdapter after config loading
- Connect to DataAdapter: `config.InitializeDataAdapter(ctx, logger)`
- Add cleanup in shutdown: `defer config.GetDataAdapter().Disconnect(ctx)`
- Verify lifecycle management (Connect → Use → Disconnect)

#### go.mod Updates
```go
require (
    github.com/quantfidential/trading-ecosystem/market-data-adapter-go v0.1.0
    // ... existing dependencies
)

replace github.com/quantfidential/trading-ecosystem/market-data-adapter-go => ../market-data-adapter-go
```

**Validation**:
```bash
# Verify imports resolve
go mod tidy

# Build with DataAdapter integration
go build ./...

# Check for compilation errors
echo $?  # Should be 0
```

**Acceptance Criteria**:
- [ ] Service discovery using DataAdapter.ServiceDiscoveryRepository
- [ ] Configuration caching using DataAdapter.CacheRepository
- [ ] DataAdapter initialized in config layer
- [ ] Proper lifecycle management (Connect/Disconnect)
- [ ] Build compiles successfully
- [ ] No direct Redis/PostgreSQL client usage in infrastructure

---

### Task 3: Update Service Layer
**Goal**: Integrate market data domain operations with repository patterns
**Estimated Time**: 2-3 hours

#### Files to Modify/Create

**internal/services/market_data.go**:
```go
package services

import (
    "context"
    "time"
    "github.com/quantfidential/trading-ecosystem/market-data-adapter-go/pkg/adapters"
    "github.com/quantfidential/trading-ecosystem/market-data-adapter-go/pkg/models"
    "github.com/shopspring/decimal"
    "github.com/sirupsen/logrus"
)

type MarketDataService struct {
    config      *config.Config
    logger      *logrus.Logger
    dataAdapter adapters.DataAdapter
}

func NewMarketDataService(cfg *config.Config, logger *logrus.Logger) *MarketDataService {
    return &MarketDataService{
        config:      cfg,
        logger:      logger,
        dataAdapter: cfg.GetDataAdapter(),
    }
}

// Price feed operations
func (s *MarketDataService) PublishPrice(ctx context.Context, symbol string, price decimal.Decimal) error {
    if s.dataAdapter == nil {
        return errors.New("data adapter not initialized")
    }

    priceFeed := &models.PriceFeed{
        Symbol:    symbol,
        Price:     price,
        Timestamp: time.Now(),
        Source:    "simulator",
    }

    if err := s.dataAdapter.PriceFeedRepository().Create(ctx, priceFeed); err != nil {
        s.logger.WithError(err).Error("Failed to publish price")
        return err
    }

    return nil
}

// Candle operations
func (s *MarketDataService) PublishCandle(ctx context.Context, candle *models.Candle) error {
    if s.dataAdapter == nil {
        return errors.New("data adapter not initialized")
    }

    return s.dataAdapter.CandleRepository().Create(ctx, candle)
}

// Market snapshot operations
func (s *MarketDataService) CreateSnapshot(ctx context.Context, symbol string) (*models.MarketSnapshot, error) {
    if s.dataAdapter == nil {
        return nil, errors.New("data adapter not initialized")
    }

    // Get latest price
    latestPrice, err := s.dataAdapter.PriceFeedRepository().GetLatestBySymbol(ctx, symbol)
    if err != nil {
        return nil, err
    }

    snapshot := &models.MarketSnapshot{
        Symbol:      symbol,
        LastPrice:   latestPrice.Price,
        Timestamp:   time.Now(),
    }

    if err := s.dataAdapter.MarketSnapshotRepository().Create(ctx, snapshot); err != nil {
        return nil, err
    }

    return snapshot, nil
}

// Symbol operations
func (s *MarketDataService) GetActiveSymbols(ctx context.Context) ([]*models.Symbol, error) {
    if s.dataAdapter == nil {
        return nil, errors.New("data adapter not initialized")
    }

    query := &models.SymbolQuery{
        IsActive: ptr(true),
    }

    return s.dataAdapter.SymbolRepository().Query(ctx, query)
}
```

**internal/handlers/market_data.go**:
- Update to use `MarketDataService` for all operations
- Remove any direct database access
- Delegate all data operations to service layer
- Use models from `market-data-adapter-go/pkg/models`

**internal/handlers/health.go**:
- Add DataAdapter health check
- Report market data service status
- Include database connectivity status

**Models Migration**:
- Replace local models with `market-data-adapter-go/pkg/models`:
  - `models.PriceFeed` - Real-time price data
  - `models.Candle` - OHLCV candle data
  - `models.MarketSnapshot` - Market state snapshots
  - `models.Symbol` - Trading symbol metadata
  - `models.PriceFeedQuery`, `models.CandleQuery` - Query models

**Validation**:
```bash
# Build with service layer updates
go build ./...

# Run unit tests
go test ./internal/services/... -v
go test ./internal/handlers/... -v
```

**Acceptance Criteria**:
- [ ] All price feed operations through PriceFeedRepository
- [ ] All candle operations through CandleRepository
- [ ] All snapshot operations through MarketSnapshotRepository
- [ ] All symbol operations through SymbolRepository
- [ ] Models from market-data-adapter-go/pkg/models
- [ ] Handlers delegate to service layer
- [ ] No direct database access in service/handler layers
- [ ] Health checks integrated
- [ ] Build compiles successfully

---

### Task 4: Test Integration with Orchestrator
**Goal**: Enable tests to use shared orchestrator services
**Estimated Time**: 1 hour

#### Create .env.example
```bash
# Market Data Simulator Configuration
# Copy this to .env and update with your orchestrator credentials

# Service Identity
SERVICE_NAME=market-data-simulator
SERVICE_VERSION=1.0.0
ENVIRONMENT=development

# Server Configuration
HTTP_PORT=8086
GRPC_PORT=9096

# PostgreSQL Configuration (orchestrator credentials)
POSTGRES_URL=postgres://market_data_adapter:market-data-adapter-db-pass@localhost:5432/trading_ecosystem?sslmode=disable

# PostgreSQL Connection Pool
MAX_CONNECTIONS=25
MAX_IDLE_CONNECTIONS=10
CONNECTION_MAX_LIFETIME=300s

# Redis Configuration (orchestrator credentials)
# Production: Use market-data-adapter user
# Testing: Use admin user for full access
REDIS_URL=redis://market-data-adapter:market-data-pass@localhost:6379/0

# Redis Connection Pool
REDIS_POOL_SIZE=10
REDIS_MIN_IDLE_CONNS=2

# Cache Configuration
CACHE_TTL=300s
CACHE_NAMESPACE=market_data

# Service Discovery
SERVICE_DISCOVERY_NAMESPACE=market_data
HEARTBEAT_INTERVAL=30s
SERVICE_TTL=90s

# Market Data Configuration
DEFAULT_SYMBOLS=BTC/USD,ETH/USD,ADA/USD,SOL/USD,DOT/USD
PRICE_UPDATE_INTERVAL=1s
CANDLE_INTERVAL=1m

# Test Environment
TEST_POSTGRES_URL=postgres://market_data_adapter:market-data-adapter-db-pass@localhost:5432/trading_ecosystem?sslmode=disable
TEST_REDIS_URL=redis://admin:admin-secure-pass@localhost:6379/0

# Logging
LOG_LEVEL=info
LOG_FORMAT=json
```

#### Update Makefile
```makefile
.PHONY: test test-unit test-integration test-all check-env

# Load .env file if it exists
ifneq (,$(wildcard .env))
    include .env
    export
endif

check-env:
 @if [ ! -f .env ]; then \
  echo "Warning: .env not found. Copy .env.example to .env"; \
  exit 1; \
 fi

test-unit:
 @if [ -f .env ]; then set -a && . ./.env && set +a; fi && \
 go test ./internal/... -v -short

test-integration: check-env
 @set -a && . ./.env && set +a && \
 go test ./tests/... -v

test-all: check-env
 @set -a && . ./.env && set +a && \
 go test ./... -v

build:
 go build -v ./...

clean:
 go clean -testcache
```

#### Update .gitignore
```
# Environment files (security)
.env
.env.local
.env.*.local

# Test artifacts
coverage.out
coverage.html
*.test

# Go build artifacts
*.exe
*.exe~
*.dll
*.so
*.dylib
market-data-simulator
```

#### Add godotenv to go.mod
```bash
go get github.com/joho/godotenv@v1.5.1
```

**Validation**:
```bash
# Create .env from template
cp .env.example .env

# Verify environment loading
make check-env

# Run tests with orchestrator connection
make test-unit
make test-integration
```

**Acceptance Criteria**:
- [ ] .env.example created with orchestrator credentials
- [ ] Makefile enhanced with .env loading
- [ ] godotenv dependency added
- [ ] .gitignore updated for security
- [ ] .env created from template
- [ ] Tests can load environment configuration
- [ ] DataAdapter connects to orchestrator services

---

### Task 5: Configuration Integration
**Goal**: Align environment configuration with orchestrator patterns
**Estimated Time**: 30 minutes (merged with Task 4)

Already completed in Task 4:
- [x] .env.example created
- [x] Environment configuration aligned
- [x] DataAdapter lifecycle in main.go
- [x] Proper connection management

**Validation**:
```bash
# Verify configuration loading
go run cmd/server/main.go

# Check DataAdapter initialization in logs
# Should see: "DataAdapter connected" or "Running in stub mode"
```

**Acceptance Criteria**:
- [ ] Environment variables loaded from .env
- [ ] DataAdapter initialized with orchestrator credentials
- [ ] Graceful fallback when infrastructure unavailable
- [ ] Configuration documented in .env.example

---

### Task 6: Docker Deployment Integration
**Goal**: Package market-data-simulator-go for orchestrator deployment
**Estimated Time**: 1 hour

#### Update Dockerfile

```dockerfile
# Multi-stage build for market-data-simulator
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy market-data-adapter-go dependency from parent context
COPY market-data-adapter-go/ ./market-data-adapter-go/

# Copy market-data-simulator-go files
COPY market-data-simulator-go/go.mod market-data-simulator-go/go.sum ./market-data-simulator-go/
WORKDIR /build/market-data-simulator-go
RUN go mod download

# Copy source code and build
COPY market-data-simulator-go/ .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o market-data-simulator ./cmd/server

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk --no-cache add ca-certificates wget && \
    addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/market-data-simulator-go/market-data-simulator /app/market-data-simulator

# Set ownership
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose ports
EXPOSE 8086 9096

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8086/api/v1/health || exit 1

# Run the service
CMD ["./market-data-simulator"]
```

**Build Command** (from parent directory):
```bash
cd /path/to/trading-ecosystem
docker build -f market-data-simulator-go/Dockerfile -t market-data-simulator:latest .
```

**Validation**:
```bash
# Build image
docker build -f market-data-simulator-go/Dockerfile -t market-data-simulator:latest .

# Check image size (should be <100MB)
docker images market-data-simulator:latest

# Test run
docker run --rm -p 8085:8080 -p 50055:50051 \
  -e POSTGRES_URL="postgres://market_data_adapter:market-data-adapter-db-pass@host.docker.internal:5432/trading_ecosystem" \
  -e REDIS_URL="redis://market-data-adapter:market-data-pass@host.docker.internal:6379/0" \
  market-data-simulator:latest
```

**Acceptance Criteria**:
- [ ] Dockerfile builds from parent context
- [ ] Multi-stage build optimized
- [ ] Image size under 100MB
- [ ] Non-root user security
- [ ] Health check configured
- [ ] Ports exposed correctly (8086, 9096)
- [ ] Image builds successfully

---

## 📊 Success Metrics

| Metric | Target | Status |
|--------|--------|--------|
| Tasks Complete | 7 | ⏳ 0/7 |
| Build Status | Pass | ⏳ Pending |
| DataAdapter Integration | Complete | ⏳ Pending |
| Test Environment | Working | ⏳ Pending |
| Docker Image | Built | ⏳ Pending |
| Repository Pattern | Implemented | ⏳ Pending |
| Orchestrator Ready | Yes | ⏳ Pending |

---

## 🔧 Validation Commands

### Development Workflow
```bash
# 1. Create .env from template
cp .env.example .env

# 2. Ensure market-data-adapter-go is available
ls ../market-data-adapter-go/

# 3. Update go.mod dependencies
go mod tidy

# 4. Build application
make build

# 5. Run unit tests
make test-unit

# 6. Run integration tests (requires orchestrator)
make test-integration

# 7. Build Docker image
cd ..
docker build -f market-data-simulator-go/Dockerfile -t market-data-simulator:latest .
```

### Orchestrator Integration
See `orchestrator-docker/TODO.md` for:
- PostgreSQL market_data schema setup
- Redis ACL configuration
- docker-compose service definition
- Deployment validation

---

## 🎯 Epic TSE-0001.4 Integration

**Pattern Established**: Following audit-correlator-go, custodian-simulator-go, and exchange-simulator-go proven approach

**Integration Steps**:
1. ✅ Test Infrastructure Foundation
2. ✅ Create market-data-adapter-go
3. ✅ Refactor Infrastructure Layer
4. ✅ Update Service Layer
5. ✅ Test Integration
6. ✅ Configuration Integration
7. ✅ Docker Deployment

**Ready for Orchestrator Deployment**: Once all tasks complete, service can be deployed to docker-compose

---

**Last Updated**: 2025-09-30
**Estimated Completion**: 6-8 hours following proven pattern

---

### 📈 Milestone TSE-0001.13a: Data Flow Integration
**Status**: Not Started
**Priority**: Medium

**Tasks**:
- [ ] End-to-end market data flow testing
- [ ] Market data delivery to risk monitor validation
- [ ] Data latency and accuracy validation
- [ ] Price feed resilience testing

**BDD Acceptance**: Market data flows correctly from simulator to risk monitor with acceptable latency

**Dependencies**: TSE-0001.8b (Risk Monitor Alert Generation), TSE-0001.11 (Audit Infrastructure)

---

## Implementation Notes

- **Data Source**: Start with static price simulation, prepare for real data integration
- **Production API**: REST endpoints that risk monitor will use
- **Audit API**: Separate endpoints for chaos injection and internal state
- **Performance**: Low-latency price distribution critical for trading
- **Chaos Ready**: Design for controlled price manipulation scenarios

---

**Last Updated**: 2025-09-17