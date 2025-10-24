//go:build unit || !integration

package config

import (
	"os"
	"testing"
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
		if cfg.HTTPPort != 8080 {
			t.Errorf("Expected HTTPPort to be 8080, got %d", cfg.HTTPPort)
		}
		if cfg.GRPCPort != 50051 {
			t.Errorf("Expected GRPCPort to be 50051, got %d", cfg.GRPCPort)
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
