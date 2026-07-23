# Serialization Safe

[![Go Version](https://img.shields.io/badge/Go-1.26-blue)](https://go.dev/)
[![License: GPL-3.0](https://img.shields.io/badge/License-GPL--3.0-blue.svg)](LICENSE)

**Safe content-type serialization provider for the MuxCore fabric.**

A MuxCore sidecar module that provides bidirectional conversion between JSON and MessagePack (msgpack) content types. When a task or message needs serialization in a format that differs from the source, the Serialization Safe module handles the conversion safely and efficiently.

## How It Works

```
Source data (JSON/msgpack) ──→ serialization-safe (gRPC) ──→ Convert()
                                   │
                                   ▼
                            Target data (JSON/msgpack)
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `SERIALIZATION_GRPC_ADDR` | `:9630` | gRPC listen address |
| `MUXCORE_GRPC_ADDR` | – | Core gRPC address |
| `MUXCORE_INSECURE_DISABLE_TLS` | – | Disable TLS when set to `true` (dev mode) |
| `MUXCORE_MODULE_ID` | – | Override module ID (SDK) |

## RPCs

- `Convert(sourceContentType, targetContentType, data)` — convert data between supported serialization formats
- `SupportedTypes()` — return the list of supported content types

Supported types: `application/json`, `application/msgpack`.

## Dependencies

- `github.com/vmihailenco/msgpack/v5` — MessagePack encoding/decoding
- `google.golang.org/grpc` — gRPC service framework
- `github.com/Muxcore-Media/core` — contracts and module SDK

## Build & Develop

```bash
make build   # compile the module binary
make test    # run tests with race detection
make lint    # golangci-lint
make fmt     # format Go source
make docker  # build Docker image
```

## Capabilities

- Registers with capabilities: `serialization`, `serialization.safe`
- Declares contract `SerializationProvider`; serves gRPC `SerializationService` (`Convert`, `SupportedTypes`)

## License

GPL-3.0
