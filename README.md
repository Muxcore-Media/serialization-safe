# Serialization Safe

[![CI](https://git.zem.systems/muxcore/serialization-safe/actions/workflows/ci.yml/badge.svg)](https://git.zem.systems/muxcore/serialization-safe/actions)
[![Go Version](https://img.shields.io/badge/Go-1.26-blue)](https://go.dev/)
[![License: GPL-3.0](https://img.shields.io/badge/License-GPL--3.0-blue.svg)](LICENSE)

**Safe content-type serialization provider for the MuxCore fabric.**

A MuxCore sidecar module that provides bidirectional conversion between JSON and MessagePack (msgpack) content types. When a task or message needs serialization in a format that differs from the source, the Serialization Safe module handles the conversion safely and efficiently.

## How It Works

```
Source data (JSON/msgpack) ──→ serialization-safe (gRPC :9635) ──→ Convert()
                                   │
                                   ▼
                            Target data (JSON/msgpack)
```

Core or another module discovers this sidecar by capability (`serialization` / `serialization.safe`), dials its gRPC address from mesh registration, and calls `SerializationService` (`Convert`, `SupportedTypes`). For in-process callers, use `github.com/Muxcore-Media/serialization-safe/pkg/client` to obtain a `contracts.SerializationProvider` adapter over the gRPC service.

Example discovery + dial (Go mesh client):

```go
providers, err := meshClient.Discovery.FindByCapability(ctx, contracts.CapabilitySerialization)
// pick serialization-safe, dial its registered gRPC address, then:
ser := client.New(conn)
out, err := ser.Marshal(contracts.SafeContentTypeMsgpack, payload)
```

> **Security:** gRPC listens on **`127.0.0.1:9635`** by default with no auth on the serialization RPCs. Treat it as a LAN decode endpoint; use mesh TLS in production.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `SERIALIZATION_GRPC_ADDR` | `127.0.0.1:9635` | gRPC listen address |
| `SERIALIZATION_MAX_PAYLOAD_BYTES` | `4194304` (4 MiB) | Reject `Convert` payloads larger than this size |
| `MVP_ENABLE_SERIALIZATION_SAFE` | `0` | Set to `1` in `_mvp/.env` to start the sidecar via `run-host.sh` |
| `MUXCORE_GRPC_ADDR` | – | Core gRPC address |
| `MUXCORE_INSECURE_DISABLE_TLS` | – | Disable TLS when set to `true` (dev mode) |
| `MUXCORE_MODULE_ID` | `serialization-safe` | Override module ID (SDK) |

Runtime limits are also exposed via the **`settings`** capability (`max_payload_bytes` / `SERIALIZATION_MAX_PAYLOAD_BYTES` alias) for admin-ui.

## RPCs

- `Convert(sourceContentType, targetContentType, data)` — convert data between supported serialization formats
- `SupportedTypes()` — return the list of supported content types

Supported types: `application/json`, `application/msgpack`. Content types may include `; charset=…` suffixes. `application/x-protobuf` is rejected (`Unimplemented`) — schema-less Convert cannot encode or decode protobuf.

## Dependencies

- `github.com/vmihailenco/msgpack/v5` — MessagePack encoding/decoding
- `google.golang.org/grpc` — gRPC service framework
- `github.com/Muxcore-Media/core` v0.5.8 — contracts and module SDK

## Build & Develop

```bash
make build   # compile the module binary
make test    # run tests with race detection
make lint    # golangci-lint
make fmt     # format Go source
make docker  # build Docker image
```

## Capabilities

- Registers with capabilities: `serialization`, `serialization.safe`, `settings`
- Declares contract `SerializationProvider`; serves gRPC `SerializationService` (`Convert`, `SupportedTypes`)

## License

GPL-3.0
