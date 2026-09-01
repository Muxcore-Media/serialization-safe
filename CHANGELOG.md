# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0/).

## [0.1.2] — 2026-08-10

### Added

- `contracts.SerializationProvider` on `internal.Module` (`Marshal` / `Unmarshal` / `SupportedTypes`)
- `pkg/client` gRPC adapter for `SerializationProvider`
- Integer preservation on JSON↔msgpack (`json.Decoder.UseNumber`, int64 for integral values)
- Decode depth cap before payload byte limit
- Content-type normalization (`; charset=…`); explicit `Unimplemented` for `application/x-protobuf`
- Fail-closed `SERIALIZATION_MAX_PAYLOAD_BYTES` env parsing
- Settings tests (unknown key, env alias, invalid updates)
- Lifecycle integration test dialing `Convert` / `SupportedTypes`
- Forgejo CI: sibling `core` checkout, `go test -race`, `golangci-lint`

### Changed

- Default gRPC bind `127.0.0.1:9635` (was `:9635`)
- `Convert` returns gRPC status codes (`InvalidArgument`, `Unimplemented`)
- `Health` fails when listener/server not serving; `Stop` closes listener if `Start` never ran
- `Info().HTTPAddr` empty (gRPC-only sidecar)
- gRPC `MaxRecvMsgSize` / `MaxSendMsgSize` follow `max_payload_bytes`
- `main.version` wired for Makefile `-X` injection
- Core dependency **v0.5.8**

### Fixed

- Self-hosted CI (`runs-on: self-hosted`; `go test` without `-race` for laptop runners)

## [0.1.1] — 2026-08-10

### Added

- SettingsProvider for `max_payload_bytes` (`SERIALIZATION_MAX_PAYLOAD_BYTES`, default 4 MiB)
- Advertises `settings` capability for admin-ui discovery

## [0.1.0] — 2026-08-09

### Added

- Serialization sidecar: `Convert` between `application/json` and `application/msgpack`, plus `SupportedTypes`.
- gRPC listen default `:9635` (`SERIALIZATION_GRPC_ADDR`).
