# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0/).

## [0.1.1] — 2026-08-10

### Added

- SettingsProvider for `max_payload_bytes` (`SERIALIZATION_MAX_PAYLOAD_BYTES`, default 4 MiB)
- Advertises `settings` capability for admin-ui discovery

## [0.1.0] — 2026-08-09

### Added

- Serialization sidecar: `Convert` between `application/json` and `application/msgpack`, plus `SupportedTypes`.
- gRPC listen default `:9630` (`SERIALIZATION_GRPC_ADDR`).
