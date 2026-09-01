package client

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc"

	"github.com/Muxcore-Media/core/pkg/contracts"
	serializationv1 "github.com/Muxcore-Media/core/proto/gen/muxcore/serialization/v1"
)

// Client implements contracts.SerializationProvider over SerializationService gRPC.
type Client struct {
	rpc serializationv1.SerializationServiceClient
}

// New returns a SerializationProvider client backed by conn.
func New(conn grpc.ClientConnInterface) *Client {
	return &Client{rpc: serializationv1.NewSerializationServiceClient(conn)}
}

var _ contracts.SerializationProvider = (*Client)(nil)

// Marshal serializes v to the requested content type (JSON intermediate, then Convert when needed).
func (c *Client) Marshal(contentType string, v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("json encode: %w", err)
	}
	if normalizeContentType(contentType) == contracts.SafeContentTypeJSON {
		return raw, nil
	}
	resp, err := c.rpc.Convert(context.Background(), &serializationv1.ConvertRequest{
		SourceContentType: contracts.SafeContentTypeJSON,
		TargetContentType: contentType,
		Data:              raw,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetResult(), nil
}

// Unmarshal deserializes data into v (Convert to JSON when needed, then json.Unmarshal).
func (c *Client) Unmarshal(contentType string, data []byte, v any) error {
	if v == nil {
		return fmt.Errorf("target must be a non-nil pointer")
	}
	var raw []byte
	if normalizeContentType(contentType) == contracts.SafeContentTypeJSON {
		raw = data
	} else {
		resp, err := c.rpc.Convert(context.Background(), &serializationv1.ConvertRequest{
			SourceContentType: contentType,
			TargetContentType: contracts.SafeContentTypeJSON,
			Data:              data,
		})
		if err != nil {
			return err
		}
		raw = resp.GetResult()
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("json decode: %w", err)
	}
	return nil
}

// SupportedTypes returns content types advertised by the sidecar.
func (c *Client) SupportedTypes() []string {
	resp, err := c.rpc.SupportedTypes(context.Background(), &serializationv1.SupportedTypesRequest{})
	if err != nil {
		return nil
	}
	return append([]string(nil), resp.GetContentTypes()...)
}

func normalizeContentType(ct string) string {
	ct = trimSpace(ct)
	if idx := indexSemicolon(ct); idx >= 0 {
		ct = trimSpace(ct[:idx])
	}
	return ct
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func indexSemicolon(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ';' {
			return i
		}
	}
	return -1
}
