package internal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"google.golang.org/grpc"

	"github.com/Muxcore-Media/core/pkg/contracts"
	serializationv1 "github.com/Muxcore-Media/core/proto/gen/muxcore/serialization/v1"
	modulesdk "github.com/Muxcore-Media/core/sdk/go/module"
)

const (
	defaultMaxPayloadBytes = 4 << 20 // 4 MiB
	defaultGRPCAddr        = "127.0.0.1:9635"
)

// Version is injected at link time via -X main.version.
var Version = "0.1.2"

var (
	supportedTypes = []string{
		contracts.SafeContentTypeJSON,
		contracts.SafeContentTypeMsgpack,
	}
	errUnsupportedType       = errors.New("unsupported content type")
	errUnsupportedConversion = errors.New("unsupported conversion between source and target types")
	errPayloadTooLarge       = errors.New("payload exceeds max_payload_bytes")
)

type Module struct {
	mu              sync.RWMutex
	id              string
	grpcAddr        string
	grpcSrv         *grpc.Server
	lis             net.Listener
	maxPayloadBytes int
}

type Config struct {
	ID              string
	GRPCAddr        string
	MaxPayloadBytes int
}

func NewModule(cfg Config) (*Module, error) {
	if cfg.ID == "" {
		cfg.ID = "serialization-safe"
	}
	if cfg.GRPCAddr == "" {
		cfg.GRPCAddr = defaultGRPCAddr
	}
	if v := strings.TrimSpace(os.Getenv("SERIALIZATION_GRPC_ADDR")); v != "" {
		cfg.GRPCAddr = v
	}
	maxBytes := cfg.MaxPayloadBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxPayloadBytes
	}
	if v := strings.TrimSpace(os.Getenv("SERIALIZATION_MAX_PAYLOAD_BYTES")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid SERIALIZATION_MAX_PAYLOAD_BYTES %q", v)
		}
		maxBytes = n
	}
	return &Module{
		id:              cfg.ID,
		grpcAddr:        cfg.GRPCAddr,
		maxPayloadBytes: maxBytes,
	}, nil
}

func (m *Module) Info() contracts.ModuleInfo {
	ver := Version
	if ver == "" {
		ver = "0.0.0-dev"
	}
	return contracts.ModuleInfo{
		ID:           m.id,
		Name:         "Serialization Safe",
		Version:      ver,
		Roles:        []string{"infrastructure"},
		Description:  "Safe content-type serialization provider supporting JSON and msgpack bidirectional conversion",
		Author:       "MuxCore",
		Capabilities: []string{contracts.CapabilitySerialization, "serialization.safe", "settings"},
		Contracts: []contracts.ContractDeclaration{
			{
				Repo:      "github.com/Muxcore-Media/core/pkg/contracts",
				Interface: "SerializationProvider",
				Version:   "v0.5.0",
			},
		},
		MinCoreVersion: "0.5.0",
	}
}

func (m *Module) Init(ctx context.Context) error {
	lis, err := net.Listen("tcp", m.grpcAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", m.grpcAddr, err)
	}
	m.lis = lis
	m.mu.RLock()
	maxBytes := m.maxPayloadBytes
	m.mu.RUnlock()
	slog.Info("serialization-safe initialized", "addr", m.grpcAddr, "max_payload_bytes", maxBytes)
	return nil
}

func (m *Module) Start(ctx context.Context) error {
	m.mu.RLock()
	maxBytes := m.maxPayloadBytes
	m.mu.RUnlock()

	m.grpcSrv = grpc.NewServer(
		grpc.MaxRecvMsgSize(maxBytes),
		grpc.MaxSendMsgSize(maxBytes),
	)
	serializationv1.RegisterSerializationServiceServer(m.grpcSrv, &grpcServer{mod: m})
	modulesdk.RegisterSettings(m.grpcSrv, m.id, m)

	go func() {
		slog.Info("serialization-safe gRPC service started", "addr", m.grpcAddr)
		if err := m.grpcSrv.Serve(m.lis); err != nil {
			slog.Error("serialization-safe gRPC serve error", "error", err)
		}
	}()
	return nil
}

func (m *Module) Stop(ctx context.Context) error {
	if m.grpcSrv != nil {
		m.grpcSrv.GracefulStop()
		m.grpcSrv = nil
	}
	if m.lis != nil {
		_ = m.lis.Close()
		m.lis = nil
	}
	slog.Info("serialization-safe stopped")
	return nil
}

func (m *Module) Health(ctx context.Context) error {
	m.mu.RLock()
	lis := m.lis
	srv := m.grpcSrv
	m.mu.RUnlock()
	if lis == nil {
		return errors.New("listener not initialized")
	}
	if srv == nil {
		return errors.New("gRPC server not serving")
	}
	return nil
}

// Marshal serializes v using the requested content type.
func (m *Module) Marshal(contentType string, v any) ([]byte, error) {
	m.mu.RLock()
	maxBytes := m.maxPayloadBytes
	m.mu.RUnlock()

	out, err := marshalValue(contentType, v)
	if err != nil {
		return nil, err
	}
	if maxBytes > 0 && len(out) > maxBytes {
		return nil, fmt.Errorf("%s: %d > %d", errPayloadTooLarge, len(out), maxBytes)
	}
	return out, nil
}

// Unmarshal deserializes data into v using the requested content type.
func (m *Module) Unmarshal(contentType string, data []byte, v any) error {
	m.mu.RLock()
	maxBytes := m.maxPayloadBytes
	m.mu.RUnlock()
	if maxBytes > 0 && len(data) > maxBytes {
		return fmt.Errorf("%s: %d > %d", errPayloadTooLarge, len(data), maxBytes)
	}
	return unmarshalValue(contentType, data, v)
}

// SupportedTypes returns supported content types for SerializationProvider.
func (m *Module) SupportedTypes() []string {
	out := make([]string, len(supportedTypes))
	copy(out, supportedTypes)
	return out
}

// SetListener attaches a net.Listener before Start (tests).
func (m *Module) SetListener(lis net.Listener) {
	m.lis = lis
}

// ListenerAddr returns the bound listen address (tests).
func (m *Module) ListenerAddr() string {
	if m.lis == nil {
		return ""
	}
	return m.lis.Addr().String()
}

var (
	_ contracts.Module                = (*Module)(nil)
	_ contracts.SerializationProvider = (*Module)(nil)
)
