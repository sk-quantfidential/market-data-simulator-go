package infrastructure

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"

	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/config"
	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/services"
)

type MarketDataGRPCServer struct {
	config            *config.Config
	logger            *logrus.Logger
	marketDataService *services.MarketDataService
	grpcServer        *grpc.Server
	healthServer      *health.Server
	metrics           *ServerMetrics
	startTime         time.Time
	listener          net.Listener
}

type ServerMetrics struct {
	requestCount     int64
	connectionCount  int64
	streamingClients int64
	mu               sync.RWMutex
	responseTimes    []time.Duration
}

func NewMarketDataGRPCServer(cfg *config.Config, marketDataService *services.MarketDataService, logger *logrus.Logger) *MarketDataGRPCServer {
	server := &MarketDataGRPCServer{
		config:            cfg,
		logger:            logger,
		marketDataService: marketDataService,
		metrics:           &ServerMetrics{},
		startTime:         time.Now(),
	}

	// Create gRPC server with interceptors
	server.grpcServer = grpc.NewServer(
		grpc.UnaryInterceptor(server.unaryInterceptor),
		grpc.StreamInterceptor(server.streamInterceptor),
	)

	// Setup health service
	server.healthServer = health.NewServer()
	grpc_health_v1.RegisterHealthServer(server.grpcServer, server.healthServer)
	server.healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	server.healthServer.SetServingStatus("market-data", grpc_health_v1.HealthCheckResponse_SERVING)

	return server
}

func (s *MarketDataGRPCServer) GetGRPCServer() *grpc.Server {
	return s.grpcServer
}

func (s *MarketDataGRPCServer) Start() error {
	var err error
	s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.config.GRPCPort))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %v", s.config.GRPCPort, err)
	}

	s.logger.WithField("port", s.config.GRPCPort).Info("Starting gRPC server")
	return s.grpcServer.Serve(s.listener)
}

func (s *MarketDataGRPCServer) Stop() {
	s.logger.Info("Gracefully stopping gRPC server")
	s.healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	s.healthServer.SetServingStatus("market-data", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	s.grpcServer.GracefulStop()
}

func (s *MarketDataGRPCServer) GetMetrics() map[string]interface{} {
	s.metrics.mu.RLock()
	defer s.metrics.mu.RUnlock()

	var avgResponseTime float64
	if len(s.metrics.responseTimes) > 0 {
		var total time.Duration
		for _, rt := range s.metrics.responseTimes {
			total += rt
		}
		avgResponseTime = float64(total) / float64(len(s.metrics.responseTimes)) / float64(time.Millisecond)
	}

	return map[string]interface{}{
		"uptime_seconds":         time.Since(s.startTime).Seconds(),
		"request_count":          atomic.LoadInt64(&s.metrics.requestCount),
		"connection_count":       atomic.LoadInt64(&s.metrics.connectionCount),
		"streaming_clients":      atomic.LoadInt64(&s.metrics.streamingClients),
		"avg_response_time_ms":   avgResponseTime,
		"service_name":           s.config.ServiceName,
		"service_version":        s.config.ServiceVersion,
		"health_status":          "SERVING",
	}
}

func (s *MarketDataGRPCServer) unaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()
	atomic.AddInt64(&s.metrics.requestCount, 1)
	atomic.AddInt64(&s.metrics.connectionCount, 1)
	defer atomic.AddInt64(&s.metrics.connectionCount, -1)

	// Log request details
	if p, ok := peer.FromContext(ctx); ok {
		s.logger.WithFields(logrus.Fields{
			"method":     info.FullMethod,
			"client_ip":  p.Addr.String(),
			"start_time": start,
		}).Info("Handling gRPC request")
	}

	resp, err := handler(ctx, req)

	// Record response time
	responseTime := time.Since(start)
	s.metrics.mu.Lock()
	s.metrics.responseTimes = append(s.metrics.responseTimes, responseTime)
	// Keep only last 100 response times
	if len(s.metrics.responseTimes) > 100 {
		s.metrics.responseTimes = s.metrics.responseTimes[1:]
	}
	s.metrics.mu.Unlock()

	s.logger.WithFields(logrus.Fields{
		"method":        info.FullMethod,
		"response_time": responseTime,
		"error":         err != nil,
	}).Info("Completed gRPC request")

	return resp, err
}

func (s *MarketDataGRPCServer) streamInterceptor(
	srv interface{},
	stream grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	start := time.Now()
	atomic.AddInt64(&s.metrics.streamingClients, 1)
	defer atomic.AddInt64(&s.metrics.streamingClients, -1)

	// Log stream start
	if p, ok := peer.FromContext(stream.Context()); ok {
		s.logger.WithFields(logrus.Fields{
			"method":     info.FullMethod,
			"client_ip":  p.Addr.String(),
			"start_time": start,
		}).Info("Starting gRPC stream")
	}

	err := handler(srv, stream)

	// Log stream completion
	duration := time.Since(start)
	s.logger.WithFields(logrus.Fields{
		"method":   info.FullMethod,
		"duration": duration,
		"error":    err != nil,
	}).Info("Completed gRPC stream")

	return err
}

// Wrapper for server stream to add metadata
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

func newWrappedServerStream(stream grpc.ServerStream) grpc.ServerStream {
	ctx := stream.Context()
	// Add server metadata
	md := metadata.Pairs(
		"server-name", "market-data-simulator",
		"server-version", "1.0.0",
	)
	ctx = metadata.NewOutgoingContext(ctx, md)

	return &wrappedServerStream{
		ServerStream: stream,
		ctx:          ctx,
	}
}