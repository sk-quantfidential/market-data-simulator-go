# Pull Request: TSE-0001.12b - Prometheus Metrics with Clean Architecture

**Epic:** TSE-0001 - Foundation Services & Infrastructure
**Milestone:** TSE-0001.12b - Prometheus Metrics (Clean Architecture)
**Branch:** `feature/TSE-0001.12.0-prometheus-metric-client`
**Status:** ✅ Ready for Review

## Summary

This PR implements Prometheus metrics collection using **Clean Architecture principles**, ensuring the domain layer never depends on infrastructure concerns. The implementation follows the port/adapter pattern, enabling future migration to OpenTelemetry without changing domain logic.

**Key Achievements:**
1. ✅ **Clean Architecture**: MetricsPort interface separates domain from infrastructure
2. ✅ **RED Pattern**: Rate, Errors, Duration metrics for all HTTP requests
3. ✅ **Low Cardinality**: Constant labels (service, instance, version) + request labels (method, route, code)
4. ✅ **Future-Proof**: Can swap Prometheus for OpenTelemetry by changing adapter
5. ✅ **Testable**: Mock MetricsPort for unit tests
6. ✅ **Comprehensive Tests**: 8 BDD test scenarios covering all functionality

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                      Presentation Layer                     │
│  ┌────────────────┐  ┌─────────────────────────────────┐  │
│  │  HTTP Handler  │  │   RED Metrics Middleware        │  │
│  │  /metrics      │  │   (instruments all requests)    │  │
│  └────────┬───────┘  └──────────────┬──────────────────┘  │
│           │                          │                      │
└───────────┼──────────────────────────┼──────────────────────┘
            │                          │
            │  depends on interface    │
            ▼                          ▼
┌─────────────────────────────────────────────────────────────┐
│                      Domain Layer (Port)                    │
│  ┌───────────────────────────────────────────────────────┐ │
│  │           MetricsPort (interface)                     │ │
│  │  - IncCounter(name, labels)                           │ │
│  │  - ObserveHistogram(name, value, labels)              │ │
│  │  - SetGauge(name, value, labels)                      │ │
│  │  - GetHTTPHandler() http.Handler                      │ │
│  └───────────────────────────────────────────────────────┘ │
└───────────────────────────┬─────────────────────────────────┘
                            │  implemented by adapter
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                   Infrastructure Layer (Adapter)            │
│  ┌───────────────────────────────────────────────────────┐ │
│  │       PrometheusMetricsAdapter                        │ │
│  │  implements MetricsPort                               │ │
│  │                                                        │ │
│  │  - Uses prometheus/client_golang                      │ │
│  │  - Thread-safe lazy initialization                    │ │
│  │  - Registers Go runtime metrics                       │ │
│  │  - Applies constant labels (service, instance, ver)   │ │
│  └───────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘

Future: Swap for OtelMetricsAdapter without changing domain/presentation
```

## Changes

### 1. Domain Layer - MetricsPort Interface

**File:** `internal/domain/ports/metrics.go` (NEW)

**Purpose:** Define the contract for metrics collection, independent of implementation

**Interface Methods:**
```go
type MetricsPort interface {
    // RED Pattern methods
    IncCounter(name string, labels map[string]string)
    ObserveHistogram(name string, value float64, labels map[string]string)
    SetGauge(name string, value float64, labels map[string]string)

    // HTTP serving
    GetHTTPHandler() http.Handler
}
```

**Clean Architecture Benefits:**
- Domain never imports Prometheus packages
- Interface can be mocked for testing
- Future implementations (OpenTelemetry) implement same interface

### 2. Infrastructure Layer - PrometheusMetricsAdapter

**File:** `internal/infrastructure/observability/prometheus_adapter.go` (NEW)

**Purpose:** Implement MetricsPort using Prometheus client library

**Features:**
- **Thread-safe lazy initialization**: Metrics created on first use
- **Constant labels**: Applied to all metrics (service, instance, version)
- **Separate registry**: Isolated from default Prometheus registry
- **Go runtime metrics**: Automatic collection (goroutines, memory, GC, etc.)
- **Sensible histogram buckets**: 5ms to 10s for request duration

**Implementation Details:**
```go
type PrometheusMetricsAdapter struct {
    registry       *prometheus.Registry
    counters       map[string]*prometheus.CounterVec
    histograms     map[string]*prometheus.HistogramVec
    gauges         map[string]*prometheus.GaugeVec
    mu             sync.RWMutex
    constantLabels map[string]string
}
```

**Lazy Initialization Pattern:**
1. Fast path: Read lock check
2. Slow path: Write lock + double-check + create
3. Thread-safe for concurrent requests

**Histogram Buckets:**
```
5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s, 2.5s, 5s, 10s
```
Chosen for typical HTTP API response times.

### 3. RED Metrics Middleware

**File:** `internal/infrastructure/observability/middleware.go` (NEW)

**Purpose:** Instrument all HTTP requests with RED pattern metrics

**RED Pattern Metrics:**
1. **Rate**: `http_requests_total` (counter)
   - Labels: method, route, code
   - Incremented for every request

2. **Errors**: `http_request_errors_total` (counter)
   - Labels: method, route, code
   - Incremented only for 4xx/5xx responses

3. **Duration**: `http_request_duration_seconds` (histogram)
   - Labels: method, route, code
   - Observes request latency in seconds

**Low Cardinality Enforcement:**
- **Route**: Uses `c.FullPath()` (pattern `/api/v1/prices/:symbol`) NOT full path (`/api/v1/prices/BTC`)
- **Unknown routes**: Labeled as `"unknown"` to avoid metric explosion
- **Method**: HTTP method (GET, POST, etc.) - naturally low cardinality
- **Code**: HTTP status code (200, 404, 500) - naturally low cardinality

**Middleware Usage:**
```go
router.Use(observability.REDMetricsMiddleware(metricsPort))
```

### 4. Metrics Handler

**File:** `internal/handlers/metrics.go` (NEW)

**Clean Architecture Implementation:**
```go
type MetricsHandler struct {
    metricsPort ports.MetricsPort  // Interface dependency
}

func NewMetricsHandler(metricsPort ports.MetricsPort) *MetricsHandler {
    return &MetricsHandler{
        metricsPort: metricsPort,
    }
}

func (h *MetricsHandler) Metrics(c *gin.Context) {
    handler := h.metricsPort.GetHTTPHandler()
    handler.ServeHTTP(c.Writer, c.Request)
}
```

**Benefits:**
- ✅ Depends on interface, not concrete implementation
- ✅ Can be tested with mock MetricsPort
- ✅ Future OpenTelemetry: just pass OtelMetricsAdapter

### 5. Main Server Integration

**File:** `cmd/server/main.go` (MODIFIED)

**Setup Observability:**
```go
// Initialize observability (Clean Architecture: port + adapter)
constantLabels := map[string]string{
    "service":  cfg.ServiceName,         // "market-data-simulator"
    "instance": cfg.ServiceInstanceName, // "market-data-simulator"
    "version":  cfg.ServiceVersion,      // "1.0.0"
}
metricsPort := observability.NewPrometheusMetricsAdapter(constantLabels)

// Add RED metrics middleware (Rate, Errors, Duration)
router.Use(observability.REDMetricsMiddleware(metricsPort))

// Initialize handlers
healthHandler := handlers.NewHealthHandlerWithConfig(cfg, logger)
metricsHandler := handlers.NewMetricsHandler(metricsPort)

// Observability endpoints (separate from business logic)
router.GET("/metrics", metricsHandler.Metrics)

v1 := router.Group("/api/v1")
{
    v1.GET("/health", healthHandler.Health)
    v1.GET("/ready", healthHandler.Ready)
}
```

**Dependency Injection:**
- MetricsPort interface passed to middleware and handler
- Concrete PrometheusMetricsAdapter created once at startup
- All components depend on interface, not implementation

### 6. Comprehensive Tests

**File:** `internal/handlers/metrics_test.go` (NEW)

**Test Scenarios:**
1. ✅ `exposes_prometheus_metrics_through_port`: Verifies /metrics returns Prometheus format
2. ✅ `returns_text_plain_content_type`: Verifies Content-Type header
3. ✅ `includes_standard_go_runtime_metrics`: Verifies Go runtime metrics present
4. ✅ `includes_constant_labels_in_all_metrics`: Verifies service, instance, version labels

**File:** `internal/infrastructure/observability/middleware_test.go` (NEW)

**Test Scenarios:**
1. ✅ `instruments_successful_requests_with_RED_metrics`: Verifies all RED metrics recorded
2. ✅ `instruments_error_requests_with_error_counter`: Verifies error counter for 5xx
3. ✅ `uses_route_pattern_not_full_path`: Verifies `/api/v1/prices/:symbol` not `/api/v1/prices/BTC`
4. ✅ `handles_unknown_routes_without_metric_explosion`: Verifies unknown routes labeled as `"unknown"`

**All tests follow BDD Given/When/Then pattern:**
```go
// Given: A Prometheus metrics adapter
constantLabels := map[string]string{
    "service":  "market-data-simulator",
    "instance": "market-data-simulator",
    "version":  "1.0.0",
}
metricsPort := observability.NewPrometheusMetricsAdapter(constantLabels)

// When: A request is made
req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
router.ServeHTTP(w, req)

// Then: Metrics should be recorded
if !strings.Contains(metricsOutput, "http_requests_total") {
    t.Error("Expected metric to be present")
}
```

## Metrics Exposed

### Standard Go Runtime Metrics

Automatically collected by Prometheus client:
- `go_goroutines`: Number of goroutines
- `go_threads`: Number of OS threads
- `go_memstats_alloc_bytes`: Heap memory allocated
- `process_cpu_seconds_total`: CPU time consumed

### RED Pattern Metrics

**1. http_requests_total** (counter)
```promql
http_requests_total{
  service="market-data-simulator",
  instance="market-data-simulator",
  version="1.0.0",
  method="GET",
  route="/api/v1/health",
  code="200"
}
```

**2. http_request_duration_seconds** (histogram)
```promql
http_request_duration_seconds_bucket{
  service="market-data-simulator",
  instance="market-data-simulator",
  version="1.0.0",
  method="GET",
  route="/api/v1/health",
  code="200",
  le="0.1"
} 42
```

**3. http_request_errors_total** (counter)
```promql
http_request_errors_total{
  service="market-data-simulator",
  instance="market-data-simulator",
  version="1.0.0",
  method="GET",
  route="/api/v1/fail",
  code="500"
}
```

## Example Prometheus Queries

### Request Rate (Requests per second)
```promql
rate(http_requests_total{service="market-data-simulator"}[5m])
```

### Request Rate by Route
```promql
sum by (route) (rate(http_requests_total{service="market-data-simulator"}[5m]))
```

### Request Duration (95th percentile)
```promql
histogram_quantile(0.95,
  sum by (le) (rate(http_request_duration_seconds_bucket{service="market-data-simulator"}[5m]))
)
```

### Error Rate
```promql
rate(http_request_errors_total{service="market-data-simulator"}[5m])
```

### Error Percentage
```promql
(
  rate(http_request_errors_total{service="market-data-simulator"}[5m])
  /
  rate(http_requests_total{service="market-data-simulator"}[5m])
) * 100
```

## Testing Instructions

### 1. Run Unit Tests

```bash
cd /home/skingham/Projects/Quantfidential/trading-ecosystem/market-data-simulator-go

# Run metrics handler tests
go test -v -tags=unit ./internal/handlers/... -run TestMetricsHandler

# Run middleware tests
go test -v -tags=unit ./internal/infrastructure/observability/... -run TestREDMetricsMiddleware

# Run with coverage
go test -cover -tags=unit ./internal/handlers/... -run TestMetricsHandler
go test -cover -tags=unit ./internal/infrastructure/observability/...
```

**Expected:** All 8 test scenarios pass ✅

### 2. Build and Run Service

```bash
# Rebuild service
cd /home/skingham/Projects/Quantfidential/trading-ecosystem/orchestrator-docker
docker-compose build market-data-simulator

# Start service
docker-compose up -d market-data-simulator

# Wait for startup
sleep 10
```

### 3. Verify Metrics Endpoint

```bash
# Check metrics endpoint (market-data-simulator on port 8083)
curl http://localhost:8083/metrics

# Should see:
# - # HELP go_goroutines ...
# - # TYPE go_goroutines gauge
# - go_goroutines 13
# - (many more Go runtime metrics)
```

### 4. Generate Traffic and Verify RED Metrics

```bash
# Make some requests
for i in {1..10}; do
  curl http://localhost:8083/api/v1/health
done

# Make an error request (404)
curl http://localhost:8083/nonexistent

# Check RED metrics
curl http://localhost:8083/metrics | grep -E "http_requests_total|http_request_duration|http_request_errors"
```

**Expected Output:**
```
http_requests_total{code="200",method="GET",route="/api/v1/health",...} 10
http_requests_total{code="404",method="GET",route="unknown",...} 1
http_request_duration_seconds_bucket{code="200",...,le="0.005"} 8
http_request_duration_seconds_bucket{code="200",...,le="0.01"} 10
http_request_errors_total{code="404",method="GET",route="unknown",...} 1
```

### 5. Verify Constant Labels

```bash
curl http://localhost:8083/metrics | grep -E "service=|instance=|version="
```

**Expected:**
```
http_requests_total{...,service="market-data-simulator",instance="market-data-simulator",version="1.0.0",...}
```

## Migration Path to OpenTelemetry (Phase 2)

### Current Implementation (Phase 1)
```go
// Prometheus adapter
metricsPort := observability.NewPrometheusMetricsAdapter(constantLabels)
```

### Future Implementation (Phase 2 - No Domain Changes!)
```go
// OpenTelemetry adapter (same interface!)
metricsPort := observability.NewOtelMetricsAdapter(constantLabels)
```

**Steps for OpenTelemetry Migration:**
1. Create `OtelMetricsAdapter` implementing `MetricsPort`
2. Use OpenTelemetry SDK meters instead of Prometheus client
3. Add OpenTelemetry Prometheus bridge for `/metrics` endpoint
4. Swap adapter in `main.go`
5. **Zero changes to handlers, middleware, or domain logic** ✅

**Metric Names Remain the Same:**
- `http_requests_total`
- `http_request_duration_seconds`
- `http_request_errors_total`

**Dashboards Remain the Same:** No Grafana dashboard changes needed!

## Dependencies

**New Dependencies Added:**
- `github.com/prometheus/client_golang v1.23.2`
- `github.com/prometheus/client_model v0.6.2`
- `github.com/prometheus/common v0.66.1`
- `github.com/prometheus/procfs v0.16.1`
- `github.com/beorn7/perks v1.0.1`
- `github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822`

**go.mod Updated:** Yes (go mod tidy ran successfully)

## Files Changed

**New Files:**
- `internal/domain/ports/metrics.go` (41 lines)
- `internal/infrastructure/observability/prometheus_adapter.go` (210 lines)
- `internal/infrastructure/observability/middleware.go` (78 lines)
- `internal/handlers/metrics.go` (30 lines)
- `internal/handlers/metrics_test.go` (180 lines, 4 scenarios)
- `internal/infrastructure/observability/middleware_test.go` (229 lines, 4 scenarios)
- `docs/prs/feature-TSE-0001.12.0-prometheus-metric-client.md` (THIS FILE)

**Modified Files:**
- `cmd/server/main.go` (added observability setup in setupHTTPServer)
- `TODO.md` (added milestone TSE-0001.12b with comprehensive documentation)
- `go.mod` (added Prometheus client dependencies)
- `go.sum` (dependency checksums)

**Total Lines Added:** ~768 lines (code + tests + documentation)

## Test Results

### Handler Tests (4 scenarios)
```
=== RUN   TestMetricsHandler_Metrics
=== RUN   TestMetricsHandler_Metrics/exposes_prometheus_metrics_through_port
=== RUN   TestMetricsHandler_Metrics/returns_text_plain_content_type
=== RUN   TestMetricsHandler_Metrics/includes_standard_go_runtime_metrics
=== RUN   TestMetricsHandler_Metrics/includes_constant_labels_in_all_metrics
--- PASS: TestMetricsHandler_Metrics (0.00s)
    --- PASS: TestMetricsHandler_Metrics/exposes_prometheus_metrics_through_port (0.00s)
    --- PASS: TestMetricsHandler_Metrics/returns_text_plain_content_type (0.00s)
    --- PASS: TestMetricsHandler_Metrics/includes_standard_go_runtime_metrics (0.00s)
    --- PASS: TestMetricsHandler_Metrics/includes_constant_labels_in_all_metrics (0.00s)
PASS
ok  	github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/handlers	0.022s
```

### Middleware Tests (4 scenarios)
```
=== RUN   TestREDMetricsMiddleware
=== RUN   TestREDMetricsMiddleware/instruments_successful_requests_with_RED_metrics
=== RUN   TestREDMetricsMiddleware/instruments_error_requests_with_error_counter
=== RUN   TestREDMetricsMiddleware/uses_route_pattern_not_full_path
=== RUN   TestREDMetricsMiddleware/handles_unknown_routes_without_metric_explosion
--- PASS: TestREDMetricsMiddleware (0.01s)
    --- PASS: TestREDMetricsMiddleware/instruments_successful_requests_with_RED_metrics (0.00s)
    --- PASS: TestREDMetricsMiddleware/instruments_error_requests_with_error_counter (0.00s)
    --- PASS: TestREDMetricsMiddleware/uses_route_pattern_not_full_path (0.00s)
    --- PASS: TestREDMetricsMiddleware/handles_unknown_routes_without_metric_explosion (0.00s)
PASS
ok  	github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/infrastructure/observability	0.021s
```

**✅ All 8 BDD test scenarios passing**

## Merge Checklist

- [x] Clean Architecture port/adapter pattern implemented
- [x] MetricsPort interface defined in domain layer
- [x] PrometheusMetricsAdapter implements MetricsPort
- [x] RED metrics middleware created
- [x] /metrics endpoint handler created
- [x] Constant labels applied (service, instance, version)
- [x] Low-cardinality request labels (method, route, code)
- [x] All unit tests passing (8 test scenarios)
- [x] BDD Given/When/Then test pattern followed
- [x] Integration with main.go complete
- [x] Dependencies added to go.mod
- [x] TODO.md updated with milestone TSE-0001.12b
- [x] TODO-MASTER.md updated with achievement
- [x] PR documentation complete
- [x] Pattern follows audit-correlator-go proven approach

## Approval

**Ready for Merge**: ✅ Yes

All requirements satisfied:
- ✅ Clean Architecture principles followed
- ✅ Domain layer independent of infrastructure
- ✅ Future-proof for OpenTelemetry migration
- ✅ RED pattern metrics implemented
- ✅ Low-cardinality labels enforced
- ✅ Comprehensive test coverage (8/8 passing)
- ✅ Follows proven pattern from audit-correlator-go
- ✅ Documentation complete
- ✅ Ready for TSE-0001.12a (Metrics Infrastructure) integration

---

**Epic:** TSE-0001.12b
**Branch:** feature/TSE-0001.12.0-prometheus-metric-client
**Test Results:** 8/8 tests passing
**Build Status:** ✅ Successful
**Pattern Source:** audit-correlator-go (proven implementation)

🎯 **Achievement:** Prometheus metrics with Clean Architecture - market-data-simulator-go ready for observability!

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
