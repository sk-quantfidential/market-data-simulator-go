//go:build integration

package config

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

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
