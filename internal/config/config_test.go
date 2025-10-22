package config

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// TestConfig_Load tests the configuration loading
func TestConfig_Load(t *testing.T) {
	t.Run("load_config_with_defaults", func(t *testing.T) {
		// Given: No environment variables set (cleared)
		os.Clearenv()

		// When: Loading config
		cfg := Load()

		// Then: Should load with defaults
		if cfg.ServiceName != "market-data-simulator" {
			t.Errorf("Expected ServiceName to be 'market-data-simulator', got '%s'", cfg.ServiceName)
		}
		if cfg.HTTPPort != 8083 {
			t.Errorf("Expected HTTPPort to be 8083, got %d", cfg.HTTPPort)
		}
		if cfg.GRPCPort != 9093 {
			t.Errorf("Expected GRPCPort to be 9093, got %d", cfg.GRPCPort)
		}
	})

	t.Run("load_config_with_env_vars", func(t *testing.T) {
		// Given: Custom environment variables
		os.Setenv("SERVICE_NAME", "test-market-data")
		os.Setenv("HTTP_PORT", "8888")
		os.Setenv("GRPC_PORT", "9999")
		defer os.Clearenv()

		// When: Loading config
		cfg := Load()

		// Then: Should use environment variables
		if cfg.ServiceName != "test-market-data" {
			t.Errorf("Expected ServiceName to be 'test-market-data', got '%s'", cfg.ServiceName)
		}
		if cfg.HTTPPort != 8888 {
			t.Errorf("Expected HTTPPort to be 8888, got %d", cfg.HTTPPort)
		}
		if cfg.GRPCPort != 9999 {
			t.Errorf("Expected GRPCPort to be 9999, got %d", cfg.GRPCPort)
		}
	})
}

// TestConfig_GetDataAdapter tests the GetDataAdapter method
func TestConfig_GetDataAdapter(t *testing.T) {
	t.Run("get_data_adapter_before_initialization", func(t *testing.T) {
		// Given: A new config
		cfg := Load()

		// When: Getting DataAdapter before initialization
		adapter := cfg.GetDataAdapter()

		// Then: Should return nil
		if adapter != nil {
			t.Error("Expected GetDataAdapter to return nil before initialization")
		}
	})
}

// TestConfig_DataAdapterInitialization tests the DataAdapter initialization in config
func TestConfig_DataAdapterInitialization(t *testing.T) {
	t.Run("data_adapter_graceful_degradation_without_infrastructure", func(t *testing.T) {
		// Given: A config with invalid database URLs
		os.Setenv("POSTGRES_URL", "postgres://invalid:invalid@localhost:9999/invalid?sslmode=disable")
		os.Setenv("REDIS_URL", "redis://invalid@localhost:9999/0")
		defer os.Unsetenv("POSTGRES_URL")
		defer os.Unsetenv("REDIS_URL")

		cfg := Load()
		logger := logrus.New()
		logger.SetLevel(logrus.ErrorLevel) // Reduce noise

		// When: Attempting to initialize DataAdapter
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := cfg.InitializeDataAdapter(ctx, logger)

		// Then: Should fail gracefully (returns error but doesn't panic)
		if err == nil {
			t.Log("DataAdapter initialized (infrastructure available)")
		} else {
			t.Logf("DataAdapter failed gracefully: %v", err)
		}

		// GetDataAdapter should return nil when initialization failed
		adapter := cfg.GetDataAdapter()
		if err != nil && adapter != nil {
			t.Error("Expected GetDataAdapter to return nil when initialization failed")
		}
	})

	t.Run("data_adapter_with_orchestrator_infrastructure", func(t *testing.T) {
		// Given: Config with orchestrator URLs (from docker-compose.yml)
		os.Setenv("POSTGRES_URL", "postgres://market_data_adapter:market-data-adapter-db-pass@localhost:5432/trading_ecosystem?sslmode=disable")
		os.Setenv("REDIS_URL", "redis://market-data-adapter:market-data-pass@localhost:6379/0")
		defer os.Unsetenv("POSTGRES_URL")
		defer os.Unsetenv("REDIS_URL")

		cfg := Load()
		logger := logrus.New()
		logger.SetLevel(logrus.WarnLevel)

		// When: Attempting to initialize DataAdapter
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := cfg.InitializeDataAdapter(ctx, logger)

		// Then: Should connect if orchestrator is running
		if err == nil {
			t.Log("✓ DataAdapter initialized successfully (orchestrator available)")
			adapter := cfg.GetDataAdapter()
			if adapter == nil {
				t.Error("Expected GetDataAdapter to return non-nil when initialization succeeded")
			}

			// Verify repositories are accessible
			if adapter.PriceFeedRepository() == nil {
				t.Error("Expected PriceFeedRepository to be non-nil")
			}
			if adapter.CandleRepository() == nil {
				t.Error("Expected CandleRepository to be non-nil")
			}
			if adapter.MarketSnapshotRepository() == nil {
				t.Error("Expected MarketSnapshotRepository to be non-nil")
			}
			if adapter.SymbolRepository() == nil {
				t.Error("Expected SymbolRepository to be non-nil")
			}
			if adapter.ServiceDiscoveryRepository() == nil {
				t.Error("Expected ServiceDiscoveryRepository to be non-nil")
			}
			if adapter.CacheRepository() == nil {
				t.Error("Expected CacheRepository to be non-nil")
			}

			// Cleanup
			if err := cfg.DisconnectDataAdapter(ctx); err != nil {
				t.Logf("Disconnect error: %v", err)
			}
		} else {
			t.Logf("⏭️ Skipping DataAdapter test (orchestrator not running): %v", err)
			t.Skip("Orchestrator infrastructure not available")
		}
	})
}
