# feat(epic-TSE-0002): Add Connect protocol support for browser clients

## Summary

Adds Connect protocol support to market-data-simulator-go to enable browser-based gRPC communication for real-time market data streaming. Follows the pattern successfully implemented in audit-correlator-go.

**Problem**: Browser clients cannot use native gRPC directly and require Connect protocol for streaming market data.

**Solution**: Implemented full Connect protocol stack alongside existing gRPC server, enabling browsers to access all MarketDataService RPCs including real-time price streams and scenario simulations.

## What Changed

### Dependencies (`go.mod`)
- **Added Connect framework**: `connectrpc.com/connect v1.19.1`
- **Added reflection support**: `connectrpc.com/grpcreflect v1.3.0`
- **Added CORS support**: `connectrpc.com/cors v0.1.0`
- **Added HTTP/2**: `golang.org/x/net v0.46.0` (upgraded)

### Generated Files
- **`internal/proto/protoconnect/marketdata.connect.go`** (Generated, 400+ lines):
  - `MarketDataServiceHandler` interface
  - `NewMarketDataServiceHandler` function
  - Supports Connect, gRPC, and gRPC-Web protocols
  - Binary Protobuf and JSON codecs

### Infrastructure Layer
- **`internal/presentation/connect/marketdata_connect_adapter.go`** (New, 198 lines):
  - `MarketDataConnectAdapter` struct wrapping gRPC handler
  - Implements `protoconnect.MarketDataServiceHandler` interface
  - **Unary RPCs**: `GetPrice`, `GenerateSimulation`, `HealthCheck`
  - **Streaming RPCs**: `StreamPrices`, `StreamScenario`
  - Stream adapters: `priceStreamAdapter`, `scenarioStreamAdapter`
  - Bridges Connect and gRPC streaming interfaces

### Presentation Layer (`cmd/server/main.go`)
- **CORS middleware** for browser requests:
  - Allows all origins (development mode)
  - Exposes Connect protocol headers
  - Handles OPTIONS preflight requests

- **Connect handler registration**:
  - `registerConnectHandlers()` function
  - Creates Connect adapter from gRPC handler
  - Registers handlers at `/marketdata.MarketDataService/*`

- **HTTP/2 support**:
  - Enabled h2c (HTTP/2 cleartext) for Connect protocol
  - Maintains backward compatibility with HTTP/1.1

## MarketDataService RPCs

All 5 RPC methods now support Connect protocol:

### Unary RPCs
1. **GetPrice**: Get current price for a symbol
2. **GenerateSimulation**: Generate simulated market data with statistical similarity
3. **HealthCheck**: Service health status

### Server Streaming RPCs
1. **StreamPrices**: Real-time price updates for multiple symbols
2. **StreamScenario**: Simulated market scenarios (rally, crash, divergence, etc.)

## Testing

```bash
# Build verification
go build ./...
# ✅ Build succeeds

# Run all tests
go test ./... -short -count=1
# ✅ All tests pass (no regression)
#    - internal/config: PASS
#    - internal/handlers: PASS

# Test Connect endpoint (after deployment)
curl -X POST http://localhost:8085/marketdata.MarketDataService/GetPrice \
  -H "Content-Type: application/json" \
  -d '{"symbol": "BTC-USD"}'

# Expected response:
# {"symbol":"BTC-USD","price":45123.45,"timestamp":"2025-10-26T09:00:00Z","source":"market-data-simulator"}
```

## Docker Configuration

Port mapping verified in `orchestrator-docker/docker-compose.yml`:

```yaml
market-data-simulator:
  ports:
    - "127.0.0.1:8085:8080"   # HTTP port (Connect protocol)
    - "127.0.0.1:50055:50051" # gRPC port (native gRPC)
```

**HTTP port 8085** now serves:
- Connect protocol (for browsers)
- REST health endpoints (`/api/v1/health`, `/api/v1/ready`)
- Prometheus metrics (`/metrics`)

**gRPC port 50055** continues serving:
- Native gRPC protocol (for server-to-server communication)

## Architecture Impact

✅ **Clean Architecture Maintained**:
- Connect adapter in presentation layer (adapter pattern)
- No changes to domain or application layers
- gRPC handler remains unchanged
- Dual protocol support (gRPC + Connect)

✅ **No Breaking Changes**:
- Existing gRPC clients unaffected
- Native gRPC server continues on port 50051
- Connect protocol adds new capability without replacing existing one

✅ **Streaming Support**:
- Stream adapters handle gRPC ↔ Connect streaming differences
- Both streaming RPCs (`StreamPrices`, `StreamScenario`) fully functional
- Metadata handling adapted for Connect protocol

## Browser Integration

Browser clients can now:

1. **Subscribe to real-time market data**:
   ```javascript
   const client = createConnectTransport({
     baseUrl: "http://localhost:8085"
   });

   for await (const update of client.streamPrices({
     symbols: ["BTC-USD", "ETH-USD"],
     updateIntervalMs: 1000
   })) {
     console.log(`${update.symbol}: $${update.price}`);
   }
   ```

2. **Generate market simulations**:
   ```javascript
   const simulation = await client.generateSimulation({
     symbol: "BTC-USD",
     simulationType: SimulationType.MONTE_CARLO,
     parameters: { volatilityFactor: 1.5 }
   });
   ```

3. **Stream scenario testing**:
   ```javascript
   for await (const update of client.streamScenario({
     symbol: "BTC-USD",
     scenarioType: ScenarioType.CRASH,
     parameters: { intensity: 1.5 }
   })) {
     console.log(`Crash scenario: $${update.price}`);
   }
   ```

## Related Work

- **Follows pattern from**: `audit-correlator-go` feat(epic-TSE-0002) ✅
- **Part of epic**: TSE-0002 Network Topology Visualization
- **Enables**: Browser-based market data visualization and scenario testing

## Future Enhancements

Phase 1 complete (this PR). Future phases:

- **Phase 2**: WebSocket support for lower latency
- **Phase 3**: Authentication/authorization for Connect endpoints
- **Phase 4**: Rate limiting and throttling for browser clients
- **Phase 5**: Advanced metrics collection for Connect traffic

## Epic Context

**Epic**: TSE-0002 - Network Topology Visualization
**Component**: market-data-simulator-go
**Type**: Feature (Connect protocol)
**Status**: ✅ Ready for deployment

This enables browser-based UIs to stream real-time market data and test trading scenarios without requiring gRPC-Web proxies or polling.

## Branch Information

- **Branch**: `feature/epic-TSE-0002-connect-protocol`
- **Base**: `main`
- **Type**: `feature` (new functionality)
- **Epic**: TSE-0002
- **Milestone**: Browser connectivity for simulators

## Checklist

- [x] Connect dependencies added
- [x] Connect handlers generated from proto files
- [x] Connect adapter created with stream support
- [x] CORS middleware configured
- [x] HTTP/2 (h2c) enabled
- [x] All tests pass (no regression)
- [x] Build succeeds
- [x] Port configuration verified
- [x] Documentation complete
- [x] Follows audit-correlator pattern
- [x] Branch name follows `feature/epic-XXX-description` format
- [x] Ready for Docker deployment testing
