package connectpresentation

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/grpc/metadata"

	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/handlers"
	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/proto"
	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/proto/protoconnect"
)

// MarketDataConnectAdapter adapts gRPC MarketDataService to Connect protocol
type MarketDataConnectAdapter struct {
	grpcHandler *handlers.MarketDataGRPCHandler
}

// Ensure adapter implements Connect handler interface
var _ protoconnect.MarketDataServiceHandler = (*MarketDataConnectAdapter)(nil)

// NewMarketDataConnectAdapter creates a new Connect adapter
func NewMarketDataConnectAdapter(grpcHandler *handlers.MarketDataGRPCHandler) *MarketDataConnectAdapter {
	return &MarketDataConnectAdapter{
		grpcHandler: grpcHandler,
	}
}

// GetPrice implements the Connect handler for GetPrice (unary RPC)
func (h *MarketDataConnectAdapter) GetPrice(
	ctx context.Context,
	req *connect.Request[proto.GetPriceRequest],
) (*connect.Response[proto.GetPriceResponse], error) {
	// Call the underlying gRPC handler
	resp, err := h.grpcHandler.GetPrice(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// StreamPrices implements the Connect handler for StreamPrices (server streaming RPC)
func (h *MarketDataConnectAdapter) StreamPrices(
	ctx context.Context,
	req *connect.Request[proto.StreamPricesRequest],
	stream *connect.ServerStream[proto.PriceUpdate],
) error {
	// Create a stream adapter to bridge Connect and gRPC streaming interfaces
	streamAdapter := &priceStreamAdapter{
		stream: stream,
		ctx:    ctx,
	}

	// Call the underlying gRPC handler with adapted stream
	return h.grpcHandler.StreamPrices(req.Msg, streamAdapter)
}

// GenerateSimulation implements the Connect handler for GenerateSimulation (unary RPC)
func (h *MarketDataConnectAdapter) GenerateSimulation(
	ctx context.Context,
	req *connect.Request[proto.SimulationRequest],
) (*connect.Response[proto.SimulationResponse], error) {
	resp, err := h.grpcHandler.GenerateSimulation(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// StreamScenario implements the Connect handler for StreamScenario (server streaming RPC)
func (h *MarketDataConnectAdapter) StreamScenario(
	ctx context.Context,
	req *connect.Request[proto.ScenarioRequest],
	stream *connect.ServerStream[proto.PriceUpdate],
) error {
	streamAdapter := &scenarioStreamAdapter{
		stream: stream,
		ctx:    ctx,
	}

	return h.grpcHandler.StreamScenario(req.Msg, streamAdapter)
}

// HealthCheck implements the Connect handler for HealthCheck (unary RPC)
func (h *MarketDataConnectAdapter) HealthCheck(
	ctx context.Context,
	req *connect.Request[proto.HealthCheckRequest],
) (*connect.Response[proto.HealthCheckResponse], error) {
	resp, err := h.grpcHandler.HealthCheck(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// priceStreamAdapter adapts Connect ServerStream to gRPC streaming interface for PriceUpdate
type priceStreamAdapter struct {
	stream *connect.ServerStream[proto.PriceUpdate]
	ctx    context.Context
}

// Send implements grpc.ServerStream.SendMsg for PriceUpdate
func (s *priceStreamAdapter) Send(msg *proto.PriceUpdate) error {
	return s.stream.Send(msg)
}

// Context implements grpc.ServerStream.Context
func (s *priceStreamAdapter) Context() context.Context {
	return s.ctx
}

// SetHeader implements grpc.ServerStream.SetHeader (no-op for basic usage)
func (s *priceStreamAdapter) SetHeader(md metadata.MD) error {
	// Connect protocol handles headers differently
	return nil
}

// SendHeader implements grpc.ServerStream.SendHeader (no-op for basic usage)
func (s *priceStreamAdapter) SendHeader(md metadata.MD) error {
	return nil
}

// SetTrailer implements grpc.ServerStream.SetTrailer (no-op for basic usage)
func (s *priceStreamAdapter) SetTrailer(md metadata.MD) {
	// Connect protocol handles trailers differently
}

// SendMsg implements grpc.ServerStream.SendMsg (required by interface)
func (s *priceStreamAdapter) SendMsg(m interface{}) error {
	if msg, ok := m.(*proto.PriceUpdate); ok {
		return s.Send(msg)
	}
	return nil
}

// RecvMsg implements grpc.ServerStream.RecvMsg (not used for server streaming)
func (s *priceStreamAdapter) RecvMsg(m interface{}) error {
	return nil
}

// scenarioStreamAdapter adapts Connect ServerStream to gRPC streaming interface for ScenarioRequest
type scenarioStreamAdapter struct {
	stream *connect.ServerStream[proto.PriceUpdate]
	ctx    context.Context
}

// Send implements grpc.ServerStream.SendMsg for scenario stream
func (s *scenarioStreamAdapter) Send(msg *proto.PriceUpdate) error {
	return s.stream.Send(msg)
}

// Context implements grpc.ServerStream.Context
func (s *scenarioStreamAdapter) Context() context.Context {
	return s.ctx
}

// SetHeader implements grpc.ServerStream.SetHeader
func (s *scenarioStreamAdapter) SetHeader(md metadata.MD) error {
	return nil
}

// SendHeader implements grpc.ServerStream.SendHeader
func (s *scenarioStreamAdapter) SendHeader(md metadata.MD) error {
	return nil
}

// SetTrailer implements grpc.ServerStream.SetTrailer
func (s *scenarioStreamAdapter) SetTrailer(md metadata.MD) {
}

// SendMsg implements grpc.ServerStream.SendMsg
func (s *scenarioStreamAdapter) SendMsg(m interface{}) error {
	if msg, ok := m.(*proto.PriceUpdate); ok {
		return s.Send(msg)
	}
	return nil
}

// RecvMsg implements grpc.ServerStream.RecvMsg
func (s *scenarioStreamAdapter) RecvMsg(m interface{}) error {
	return nil
}
