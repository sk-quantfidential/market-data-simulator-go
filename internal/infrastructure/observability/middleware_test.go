//go:build unit

package observability_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/infrastructure/observability"
)

// TestREDMetricsMiddleware verifies RED pattern metrics instrumentation
// Following BDD Given/When/Then pattern
func TestREDMetricsMiddleware(t *testing.T) {
	t.Run("instruments_successful_requests_with_RED_metrics", func(t *testing.T) {
		// Given: A Prometheus metrics adapter
		constantLabels := map[string]string{
			"service":  "market-data-simulator",
			"instance": "market-data-simulator",
			"version":  "1.0.0",
		}
		metricsPort := observability.NewPrometheusMetricsAdapter(constantLabels)

		// And: A Gin router with RED metrics middleware
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(observability.REDMetricsMiddleware(metricsPort))

		// And: A test endpoint
		router.GET("/api/v1/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "healthy"})
		})

		// When: A successful request is made
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Then: The request should succeed
		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		// And: Metrics should be recorded
		// Get metrics output
		metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		metricsW := httptest.NewRecorder()
		metricsPort.GetHTTPHandler().ServeHTTP(metricsW, metricsReq)
		metricsOutput := metricsW.Body.String()

		// RED Metric 1: http_requests_total should be incremented
		if !strings.Contains(metricsOutput, "http_requests_total") {
			t.Error("Expected http_requests_total metric to be present")
		}

		// RED Metric 2: http_request_duration_seconds should be observed
		if !strings.Contains(metricsOutput, "http_request_duration_seconds") {
			t.Error("Expected http_request_duration_seconds metric to be present")
		}

		// Verify labels are included (method, route, code)
		if !strings.Contains(metricsOutput, `method="GET"`) {
			t.Error("Expected method label in metrics")
		}
		if !strings.Contains(metricsOutput, `route="/api/v1/health"`) {
			t.Error("Expected route label in metrics")
		}
		if !strings.Contains(metricsOutput, `code="200"`) {
			t.Error("Expected code label in metrics")
		}

		// Verify constant labels are included
		if !strings.Contains(metricsOutput, `service="market-data-simulator"`) {
			t.Error("Expected service constant label in metrics")
		}
		if !strings.Contains(metricsOutput, `instance="market-data-simulator"`) {
			t.Error("Expected instance constant label in metrics")
		}
		if !strings.Contains(metricsOutput, `version="1.0.0"`) {
			t.Error("Expected version constant label in metrics")
		}
	})

	t.Run("instruments_error_requests_with_error_counter", func(t *testing.T) {
		// Given: A Prometheus metrics adapter
		constantLabels := map[string]string{
			"service":  "market-data-simulator",
			"instance": "market-data-simulator",
			"version":  "1.0.0",
		}
		metricsPort := observability.NewPrometheusMetricsAdapter(constantLabels)

		// And: A Gin router with RED metrics middleware
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(observability.REDMetricsMiddleware(metricsPort))

		// And: An endpoint that returns an error
		router.GET("/api/v1/fail", func(c *gin.Context) {
			c.JSON(500, gin.H{"error": "internal server error"})
		})

		// When: An error request is made
		req := httptest.NewRequest(http.MethodGet, "/api/v1/fail", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Then: The request should return 500
		if w.Code != 500 {
			t.Errorf("Expected status 500, got %d", w.Code)
		}

		// And: Error metrics should be recorded
		metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		metricsW := httptest.NewRecorder()
		metricsPort.GetHTTPHandler().ServeHTTP(metricsW, metricsReq)
		metricsOutput := metricsW.Body.String()

		// RED Metric 3: http_request_errors_total should be incremented
		if !strings.Contains(metricsOutput, "http_request_errors_total") {
			t.Error("Expected http_request_errors_total metric to be present")
		}

		// Verify error labels
		if !strings.Contains(metricsOutput, `code="500"`) {
			t.Error("Expected code=500 label in error metrics")
		}
	})

	t.Run("uses_route_pattern_not_full_path", func(t *testing.T) {
		// Given: A Prometheus metrics adapter
		constantLabels := map[string]string{
			"service":  "market-data-simulator",
			"instance": "market-data-simulator",
			"version":  "1.0.0",
		}
		metricsPort := observability.NewPrometheusMetricsAdapter(constantLabels)

		// And: A Gin router with RED metrics middleware
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(observability.REDMetricsMiddleware(metricsPort))

		// And: A parameterized route
		router.GET("/api/v1/prices/:symbol", func(c *gin.Context) {
			c.JSON(200, gin.H{"price": 100.0})
		})

		// When: Multiple requests are made with different parameters
		symbols := []string{"BTC", "ETH", "SOL"}
		for _, symbol := range symbols {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/"+symbol, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}

		// Then: Metrics should use route pattern, not full paths
		metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		metricsW := httptest.NewRecorder()
		metricsPort.GetHTTPHandler().ServeHTTP(metricsW, metricsReq)
		metricsOutput := metricsW.Body.String()

		// Should contain route pattern
		if !strings.Contains(metricsOutput, `route="/api/v1/prices/:symbol"`) {
			t.Error("Expected route pattern '/api/v1/prices/:symbol' in metrics")
		}

		// Should NOT contain individual paths (low cardinality check)
		badPatterns := []string{
			`route="/api/v1/prices/BTC"`,
			`route="/api/v1/prices/ETH"`,
			`route="/api/v1/prices/SOL"`,
		}

		for _, pattern := range badPatterns {
			if strings.Contains(metricsOutput, pattern) {
				t.Errorf("Metrics should not contain full path: %s (violates low cardinality)", pattern)
			}
		}
	})

	t.Run("handles_unknown_routes_without_metric_explosion", func(t *testing.T) {
		// Given: A Prometheus metrics adapter
		constantLabels := map[string]string{
			"service":  "market-data-simulator",
			"instance": "market-data-simulator",
			"version":  "1.0.0",
		}
		metricsPort := observability.NewPrometheusMetricsAdapter(constantLabels)

		// And: A Gin router with RED metrics middleware
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(observability.REDMetricsMiddleware(metricsPort))

		// And: No routes defined (all requests will be 404)

		// When: Multiple 404 requests are made with different paths
		unknownPaths := []string{"/random1", "/random2", "/random3"}
		for _, path := range unknownPaths {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}

		// Then: All 404s should be grouped under "unknown" route
		metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		metricsW := httptest.NewRecorder()
		metricsPort.GetHTTPHandler().ServeHTTP(metricsW, metricsReq)
		metricsOutput := metricsW.Body.String()

		// Should contain "unknown" route
		if !strings.Contains(metricsOutput, `route="unknown"`) {
			t.Error("Expected route='unknown' for 404 requests")
		}

		// Should NOT contain individual unknown paths
		for _, path := range unknownPaths {
			badPattern := `route="` + path + `"`
			if strings.Contains(metricsOutput, badPattern) {
				t.Errorf("Metrics should not contain unknown path: %s (violates low cardinality)", path)
			}
		}
	})
}
