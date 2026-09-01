package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Muxcore-Media/core/pkg/contracts"
	"github.com/vmihailenco/msgpack/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maxDecodeDepth = 64

func normalizeContentType(ct string) string {
	ct = strings.TrimSpace(ct)
	if idx := strings.Index(ct, ";"); idx >= 0 {
		ct = strings.TrimSpace(ct[:idx])
	}
	return ct
}

func validateContentType(ct string) (string, error) {
	norm := normalizeContentType(ct)
	if norm == contracts.SafeContentTypeProtobuf {
		return norm, status.Error(codes.Unimplemented, "application/x-protobuf conversion is not supported (schema-less Convert cannot encode or decode protobuf)")
	}
	if !isSupported(norm) {
		return norm, status.Errorf(codes.InvalidArgument, "%s: %s", errUnsupportedType, ct)
	}
	return norm, nil
}

func isSupported(ct string) bool {
	norm := normalizeContentType(ct)
	for _, s := range supportedTypes {
		if s == norm {
			return true
		}
	}
	return false
}

func convert(src, tgt string, data []byte) ([]byte, error) {
	src, err := validateContentType(src)
	if err != nil {
		return nil, err
	}
	tgt, err = validateContentType(tgt)
	if err != nil {
		return nil, err
	}
	if src == tgt {
		return data, nil
	}

	switch {
	case src == contracts.SafeContentTypeJSON && tgt == contracts.SafeContentTypeMsgpack:
		return jsonToMsgpack(data)
	case src == contracts.SafeContentTypeMsgpack && tgt == contracts.SafeContentTypeJSON:
		return msgpackToJSON(data)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "%s: %s -> %s", errUnsupportedConversion, src, tgt)
	}
}

func jsonToMsgpack(data []byte) ([]byte, error) {
	v, err := decodeJSON(data)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "json decode: %v", err)
	}
	out, err := msgpack.Marshal(v)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "msgpack encode: %v", err)
	}
	return out, nil
}

func msgpackToJSON(data []byte) ([]byte, error) {
	var v any
	if err := msgpack.Unmarshal(data, &v); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "msgpack decode: %v", err)
	}
	if err := checkDepth(v, 1); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	v = normalizeNumbers(v)
	out, err := json.Marshal(v)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "json encode: %v", err)
	}
	return out, nil
}

func decodeJSON(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if err := checkDepth(v, 1); err != nil {
		return nil, err
	}
	return normalizeNumbers(v), nil
}

func normalizeNumbers(v any) any {
	switch x := v.(type) {
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		if f, err := x.Float64(); err == nil {
			return f
		}
		return x
	case map[string]any:
		for k, val := range x {
			x[k] = normalizeNumbers(val)
		}
		return x
	case []any:
		for i, val := range x {
			x[i] = normalizeNumbers(val)
		}
		return x
	default:
		return v
	}
}

func checkDepth(v any, depth int) error {
	if depth > maxDecodeDepth {
		return fmt.Errorf("nesting exceeds max depth %d", maxDecodeDepth)
	}
	switch x := v.(type) {
	case map[string]any:
		for _, val := range x {
			if err := checkDepth(val, depth+1); err != nil {
				return err
			}
		}
	case []any:
		for _, val := range x {
			if err := checkDepth(val, depth+1); err != nil {
				return err
			}
		}
	case map[interface{}]interface{}:
		for _, val := range x {
			if err := checkDepth(val, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func marshalValue(contentType string, v any) ([]byte, error) {
	ct, err := validateContentType(contentType)
	if err != nil {
		return nil, err
	}
	switch ct {
	case contracts.SafeContentTypeJSON:
		out, err := json.Marshal(v)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "json encode: %v", err)
		}
		return out, nil
	case contracts.SafeContentTypeMsgpack:
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "json encode: %v", err)
		}
		return jsonToMsgpack(raw)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "%s: %s", errUnsupportedType, contentType)
	}
}

func unmarshalValue(contentType string, data []byte, v any) error {
	if v == nil {
		return status.Error(codes.InvalidArgument, "target must be a non-nil pointer")
	}
	ct, err := validateContentType(contentType)
	if err != nil {
		return err
	}
	switch ct {
	case contracts.SafeContentTypeJSON:
		if err := json.Unmarshal(data, v); err != nil {
			return status.Errorf(codes.InvalidArgument, "json decode: %v", err)
		}
		return nil
	case contracts.SafeContentTypeMsgpack:
		raw, err := msgpackToJSON(data)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, v); err != nil {
			return status.Errorf(codes.InvalidArgument, "json decode: %v", err)
		}
		return nil
	default:
		return status.Errorf(codes.InvalidArgument, "%s: %s", errUnsupportedType, contentType)
	}
}
