package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/config"
	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/handlers"
	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/infrastructure"
	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/proto"
	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/services"
)

func main() {
	cfg := config.Load()

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.JSONFormatter{})

	// Add instance context to all logs
	logger = logger.WithFields(logrus.Fields{
		"service_name":  cfg.ServiceName,
		"instance_name": cfg.ServiceInstanceName,
		"environment":   cfg.Environment,
	}).Logger

	logger.Info("Starting market-data-simulator service")

	// Initialize DataAdapter
	ctx := context.Background()
	if err := cfg.InitializeDataAdapter(ctx, logger); err != nil {
		logger.WithError(err).Warn("Failed to initialize data adapter, continuing in stub mode")
	} else {
		logger.Info("Data adapter initialized successfully")
	}

	marketDataService := services.NewMarketDataService(cfg, logger)

	// Create enhanced gRPC server with market data service
	grpcServer := infrastructure.NewMarketDataGRPCServer(cfg, marketDataService, logger)
	marketDataHandler := handlers.NewMarketDataGRPCHandler(cfg, marketDataService, logger)
	proto.RegisterMarketDataServiceServer(grpcServer.GetGRPCServer(), marketDataHandler)

	httpServer := setupHTTPServer(cfg, marketDataService, logger)

	go func() {
		logger.WithField("port", cfg.GRPCPort).Info("Starting enhanced gRPC server")
		if err := grpcServer.Start(); err != nil {
			logger.WithError(err).Fatal("Failed to start gRPC server")
		}
	}()

	go func() {
		logger.WithField("port", cfg.HTTPPort).Info("Starting HTTP server")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Failed to start HTTP server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down servers...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.WithError(err).Error("HTTP server forced to shutdown")
	}

	grpcServer.Stop()

	// Disconnect DataAdapter
	if err := cfg.DisconnectDataAdapter(shutdownCtx); err != nil {
		logger.WithError(err).Error("Failed to disconnect data adapter")
	}

	logger.Info("Servers shutdown complete")
}


func setupHTTPServer(cfg *config.Config, marketDataService *services.MarketDataService, logger *logrus.Logger) *http.Server {
	router := gin.New()
	router.Use(gin.Recovery())

	healthHandler := handlers.NewHealthHandlerWithConfig(cfg, logger)

	v1 := router.Group("/api/v1")
	{
		v1.GET("/health", healthHandler.Health)
		v1.GET("/ready", healthHandler.Ready)
	}

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: router,
	}
}

