package internal

import (
	"context"

	serializationv1 "github.com/Muxcore-Media/core/proto/gen/muxcore/serialization/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type grpcServer struct {
	serializationv1.UnimplementedSerializationServiceServer
	mod *Module
}

func (s *grpcServer) Convert(ctx context.Context, req *serializationv1.ConvertRequest) (*serializationv1.ConvertResponse, error) {
	return s.mod.convertRPC(ctx, req)
}

func (s *grpcServer) SupportedTypes(ctx context.Context, req *serializationv1.SupportedTypesRequest) (*serializationv1.SupportedTypesResponse, error) {
	return &serializationv1.SupportedTypesResponse{
		ContentTypes: s.mod.SupportedTypes(),
	}, nil
}

func (m *Module) convertRPC(_ context.Context, req *serializationv1.ConvertRequest) (*serializationv1.ConvertResponse, error) {
	src := req.GetSourceContentType()
	tgt := req.GetTargetContentType()
	data := req.GetData()

	m.mu.RLock()
	maxBytes := m.maxPayloadBytes
	m.mu.RUnlock()
	if maxBytes > 0 && len(data) > maxBytes {
		return nil, status.Errorf(codes.InvalidArgument, "%s: %d > %d", errPayloadTooLarge, len(data), maxBytes)
	}

	if _, err := validateContentType(src); err != nil {
		return nil, err
	}
	if _, err := validateContentType(tgt); err != nil {
		return nil, err
	}

	srcNorm := normalizeContentType(src)
	tgtNorm := normalizeContentType(tgt)
	if srcNorm == tgtNorm {
		return &serializationv1.ConvertResponse{Result: data}, nil
	}

	result, err := convert(src, tgt, data)
	if err != nil {
		return nil, err
	}
	return &serializationv1.ConvertResponse{Result: result}, nil
}
