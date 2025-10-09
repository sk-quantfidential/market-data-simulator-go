//go:build unit

package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/handlers"
	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/infrastructure/observability"
)

// TestMetricsHandler defines the expected behaviors for metrics endpoint using Clean Architecture
// Following BDD Given/When/Then pattern
func TestMetricsHandler_Metrics(t *testing.T) {
	t.Run("exposes_prometheus_metrics_through_port", func(t *testing.T) {
		// Given: A Prometheus metrics adapter (implements MetricsPort)
		constantLabels := map[string]string{
			"service":  "market-data-simulator",
			"instance": "market-data-simulator",
			"version":  "1.0.0",
		}
		metricsPort := observability.NewPrometheusMetricsAdapter(constantLabels)

		// And: A metrics handler using the port
		metricsHandler := handlers.NewMetricsHandler(metricsPort)

		// And: A test HTTP server
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/metrics", metricsHandler.Metrics)

		// When: A GET request is made to /metrics
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Then: The response status should be 200 OK
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 OK, got %d", w.Code)
		}

		// And: The response should contain Prometheus metrics format
		body := w.Body.String()
		expectedMetrics := []string{
			"# HELP go_goroutines",
			"# TYPE go_goroutines gauge",
			"# HELP go_info",
			"# TYPE go_info gauge",
			"go_info{version=",
		}

		for _, metric := range expectedMetrics {
			if !strings.Contains(body, metric) {
				t.Errorf("Expected response to contain '%s', but it was not found", metric)
			}
		}
	})

	t.Run("returns_text_plain_content_type", func(t *testing.T) {
		// Given: A Prometheus metrics adapter
		constantLabels := map[string]string{
			"service":  "market-data-simulator",
			"instance": "market-data-simulator",
			"version":  "1.0.0",
		}
		metricsPort := observability.NewPrometheusMetricsAdapter(constantLabels)

		// And: A metrics handler using the port
		metricsHandler := handlers.NewMetricsHandler(metricsPort)

		// And: A test HTTP server
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/metrics", metricsHandler.Metrics)

		// When: A GET request is made to /metrics
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Then: The Content-Type should be text/plain
		contentType := w.Header().Get("Content-Type")
		if !strings.Contains(contentType, "text/plain") {
			t.Errorf("Expected Content-Type to contain 'text/plain', got '%s'", contentType)
		}
	})

	t.Run("includes_standard_go_runtime_metrics", func(t *testing.T) {
		// Given: A Prometheus metrics adapter
		constantLabels := map[string]string{
			"service":  "market-data-simulator",
			"instance": "market-data-simulator",
			"version":  "1.0.0",
		}
		metricsPort := observability.NewPrometheusMetricsAdapter(constantLabels)

		// And: A metrics handler using the port
		metricsHandler := handlers.NewMetricsHandler(metricsPort)

		// And: A test HTTP server
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/metrics", metricsHandler.Metrics)

		// When: A GET request is made to /metrics
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Then: Standard Go runtime metrics should be present
		body := w.Body.String()
		runtimeMetrics := []string{
			"go_goroutines",
			"go_memstats_alloc_bytes",
			"go_threads",
			"process_cpu_seconds_total",
		}

		for _, metric := range runtimeMetrics {
			if !strings.Contains(body, metric) {
				t.Errorf("Expected runtime metric '%s' to be present", metric)
			}
		}
	})

	t.Run("includes_constant_labels_in_all_metrics", func(t *testing.T) {
		// Given: A Prometheus metrics adapter with constant labels
		constantLabels := map[string]string{
			"service":  "market-data-simulator",
			"instance": "market-data-simulator",
			"version":  "1.0.0",
		}
		metricsPort := observability.NewPrometheusMetricsAdapter(constantLabels)

		// And: A custom metric is recorded
		metricsPort.IncCounter("test_counter", map[string]string{
			"test_label": "test_value",
		})

		// And: A metrics handler using the port
		metricsHandler := handlers.NewMetricsHandler(metricsPort)

		// And: A test HTTP server
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/metrics", metricsHandler.Metrics)

		// When: A GET request is made to /metrics
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Then: All metrics should include constant labels
		body := w.Body.String()
		constantLabelChecks := []string{
			`service="market-data-simulator"`,
			`instance="market-data-simulator"`,
			`version="1.0.0"`,
		}

		for _, labelCheck := range constantLabelChecks {
			if !strings.Contains(body, labelCheck) {
				t.Errorf("Expected constant label '%s' in metrics output", labelCheck)
			}
		}

		// And: Custom metric should include test label
		if !strings.Contains(body, "test_counter") {
			t.Error("Expected test_counter metric to be present")
		}
		if !strings.Contains(body, `test_label="test_value"`) {
			t.Error("Expected test_label in test_counter metric")
		}
	})
}
