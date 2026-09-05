package internal

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/Muxcore-Media/core/pkg/contracts"
	serializationv1 "github.com/Muxcore-Media/core/proto/gen/muxcore/serialization/v1"
	"github.com/vmihailenco/msgpack/v5"
)

func newTestModule(t *testing.T, cfg Config) *Module {
	t.Helper()
	m, err := NewModule(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestModuleInfo(t *testing.T) {
	m := newTestModule(t, Config{})
	info := m.Info()
	if info.ID == "" {
		t.Error("module ID must not be empty")
	}
	if info.Version != "0.1.2" {
		t.Errorf("version = %q", info.Version)
	}
	if info.MinCoreVersion == "" {
		t.Error("MinCoreVersion must not be empty")
	}
	if len(info.Contracts) == 0 {
		t.Fatal("Contracts must not be empty")
	}
	if info.Contracts[0].Interface != "SerializationProvider" {
		t.Errorf("expected SerializationProvider contract, got %s", info.Contracts[0].Interface)
	}
	if info.HTTPAddr != "" {
		t.Errorf("HTTPAddr must be empty for gRPC-only module, got %q", info.HTTPAddr)
	}
	foundSettings := false
	for _, c := range info.Capabilities {
		if c == "settings" {
			foundSettings = true
		}
	}
	if !foundSettings {
		t.Error("expected settings capability")
	}
}

func TestDefaultGRPCAddr(t *testing.T) {
	m := newTestModule(t, Config{})
	if m.grpcAddr != defaultGRPCAddr {
		t.Fatalf("grpcAddr=%q want %q", m.grpcAddr, defaultGRPCAddr)
	}
}

func TestNewModuleInvalidMaxPayloadEnv(t *testing.T) {
	t.Setenv("SERIALIZATION_MAX_PAYLOAD_BYTES", "nope")
	if _, err := NewModule(Config{}); err == nil {
		t.Fatal("expected error for invalid env")
	}
	t.Setenv("SERIALIZATION_MAX_PAYLOAD_BYTES", "0")
	if _, err := NewModule(Config{}); err == nil {
		t.Fatal("expected error for zero env")
	}
}

func TestNewModuleEnvBootstrap(t *testing.T) {
	t.Setenv("SERIALIZATION_MAX_PAYLOAD_BYTES", "8192")
	t.Setenv("SERIALIZATION_GRPC_ADDR", "127.0.0.1:19635")
	m, err := NewModule(Config{})
	if err != nil {
		t.Fatal(err)
	}
	if m.maxPayloadBytes != 8192 {
		t.Fatalf("maxPayloadBytes=%d", m.maxPayloadBytes)
	}
	if m.grpcAddr != "127.0.0.1:19635" {
		t.Fatalf("grpcAddr=%q", m.grpcAddr)
	}
}

func TestSettings_MaxPayloadBytes(t *testing.T) {
	m := newTestModule(t, Config{MaxPayloadBytes: 100})
	if err := m.UpdateSetting("max_payload_bytes", "50"); err != nil {
		t.Fatal(err)
	}
	if m.Settings()[0].Value != "50" {
		t.Fatalf("value=%q", m.Settings()[0].Value)
	}
	ctx := context.Background()
	_, err := m.convertRPC(ctx, &serializationv1.ConvertRequest{
		SourceContentType: contracts.SafeContentTypeJSON,
		TargetContentType: contracts.SafeContentTypeJSON,
		Data:              make([]byte, 51),
	})
	if err == nil {
		t.Fatal("expected payload too large")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("code=%v err=%v", status.Code(err), err)
	}
}

func TestSettingsUnknownKey(t *testing.T) {
	m := newTestModule(t, Config{})
	if err := m.UpdateSetting("bogus", "1"); err == nil {
		t.Fatal("expected unknown setting error")
	}
}

func TestSettingsEnvAlias(t *testing.T) {
	m := newTestModule(t, Config{MaxPayloadBytes: 100})
	if err := m.UpdateSetting("SERIALIZATION_MAX_PAYLOAD_BYTES", "80"); err != nil {
		t.Fatal(err)
	}
	if m.Settings()[0].Value != "80" {
		t.Fatalf("value=%q", m.Settings()[0].Value)
	}
}

func TestSettingsInvalidUpdate(t *testing.T) {
	m := newTestModule(t, Config{})
	if err := m.UpdateSetting("max_payload_bytes", "-1"); err == nil {
		t.Fatal("expected validation error")
	}
	if err := m.UpdateSetting("max_payload_bytes", "abc"); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestSerializationProviderContract(t *testing.T) {
	m := newTestModule(t, Config{})
	types := m.SupportedTypes()
	if len(types) != 2 {
		t.Fatalf("types=%v", types)
	}

	type payload struct {
		Name  string `json:"name"`
		Count int64  `json:"count"`
	}
	in := payload{Name: "x", Count: 7}
	raw, err := m.Marshal(contracts.SafeContentTypeMsgpack, in)
	if err != nil {
		t.Fatal(err)
	}
	var out payload
	if err := m.Unmarshal(contracts.SafeContentTypeMsgpack, raw, &out); err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Fatalf("got %+v want %+v", out, in)
	}
}

func TestSupportedTypes(t *testing.T) {
	m := newTestModule(t, Config{})
	srv := &grpcServer{mod: m}
	resp, err := srv.SupportedTypes(context.Background(), &serializationv1.SupportedTypesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{contracts.SafeContentTypeJSON, contracts.SafeContentTypeMsgpack}
	if len(resp.ContentTypes) != len(expected) {
		t.Fatalf("expected %d types, got %d: %v", len(expected), len(resp.ContentTypes), resp.ContentTypes)
	}
	for i, ct := range resp.ContentTypes {
		if ct != expected[i] {
			t.Errorf("type %d: expected %s, got %s", i, expected[i], ct)
		}
	}
}

func TestJSONToMsgpack(t *testing.T) {
	m := newTestModule(t, Config{})
	ctx := context.Background()

	input := []byte(`{"name":"test","count":42,"active":true,"tags":["a","b"]}`)
	resp, err := m.convertRPC(ctx, &serializationv1.ConvertRequest{
		SourceContentType: contracts.SafeContentTypeJSON,
		TargetContentType: contracts.SafeContentTypeMsgpack,
		Data:              input,
	})
	if err != nil {
		t.Fatal(err)
	}

	var v any
	if err := msgpack.Unmarshal(resp.Result, &v); err != nil {
		t.Fatalf("msgpack decode result: %v", err)
	}
	obj, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", v)
	}
	if obj["name"] != "test" {
		t.Errorf("expected name=test, got %v", obj["name"])
	}
	if obj["count"] != int64(42) {
		t.Errorf("expected count=int64(42), got %v (%T)", obj["count"], obj["count"])
	}
	if obj["active"] != true {
		t.Errorf("expected active=true, got %v", obj["active"])
	}
}

func TestMsgpackToJSON(t *testing.T) {
	m := newTestModule(t, Config{})
	ctx := context.Background()

	original := map[string]any{"name": "test", "value": 3.14}
	mp, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := m.convertRPC(ctx, &serializationv1.ConvertRequest{
		SourceContentType: contracts.SafeContentTypeMsgpack,
		TargetContentType: contracts.SafeContentTypeJSON,
		Data:              mp,
	})
	if err != nil {
		t.Fatal(err)
	}

	var v map[string]any
	if err := json.Unmarshal(resp.Result, &v); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if v["name"] != "test" {
		t.Errorf("expected name=test, got %v", v["name"])
	}
	if v["value"] != 3.14 {
		t.Errorf("expected value=3.14, got %v", v["value"])
	}
}

func TestRoundTrip(t *testing.T) {
	m := newTestModule(t, Config{})
	ctx := context.Background()

	original := []byte(`{"a":1,"b":"two","c":[1,2,3]}`)

	toMsgpack, err := m.convertRPC(ctx, &serializationv1.ConvertRequest{
		SourceContentType: contracts.SafeContentTypeJSON,
		TargetContentType: contracts.SafeContentTypeMsgpack,
		Data:              original,
	})
	if err != nil {
		t.Fatal(err)
	}

	backToJSON, err := m.convertRPC(ctx, &serializationv1.ConvertRequest{
		SourceContentType: contracts.SafeContentTypeMsgpack,
		TargetContentType: contracts.SafeContentTypeJSON,
		Data:              toMsgpack.Result,
	})
	if err != nil {
		t.Fatal(err)
	}

	var expected, got map[string]any
	if err := json.Unmarshal(original, &expected); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(backToJSON.Result, &got); err != nil {
		t.Fatal(err)
	}
	if expected["a"] != got["a"] {
		t.Errorf("a: expected %v, got %v", expected["a"], got["a"])
	}
	if expected["b"] != got["b"] {
		t.Errorf("b: expected %v, got %v", expected["b"], got["b"])
	}
}

func TestPassthrough(t *testing.T) {
	m := newTestModule(t, Config{})
	ctx := context.Background()

	input := []byte(`{"unchanged":true}`)
	resp, err := m.convertRPC(ctx, &serializationv1.ConvertRequest{
		SourceContentType: contracts.SafeContentTypeJSON,
		TargetContentType: contracts.SafeContentTypeJSON,
		Data:              input,
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.Result) != string(input) {
		t.Errorf("passthrough: expected %s, got %s", input, resp.Result)
	}
}

func TestCharsetContentType(t *testing.T) {
	m := newTestModule(t, Config{})
	ctx := context.Background()
	input := []byte(`{"ok":true}`)
	_, err := m.convertRPC(ctx, &serializationv1.ConvertRequest{
		SourceContentType: "application/json; charset=utf-8",
		TargetContentType: contracts.SafeContentTypeMsgpack,
		Data:              input,
	})
	if err != nil {
		t.Fatalf("charset source: %v", err)
	}
}

func TestProtobufRejected(t *testing.T) {
	m := newTestModule(t, Config{})
	ctx := context.Background()
	_, err := m.convertRPC(ctx, &serializationv1.ConvertRequest{
		SourceContentType: contracts.SafeContentTypeProtobuf,
		TargetContentType: contracts.SafeContentTypeJSON,
		Data:              []byte{1, 2, 3},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("code=%v err=%v", status.Code(err), err)
	}
}

func TestUnsupportedSource(t *testing.T) {
	m := newTestModule(t, Config{})
	ctx := context.Background()

	_, err := m.convertRPC(ctx, &serializationv1.ConvertRequest{
		SourceContentType: "application/yaml",
		TargetContentType: contracts.SafeContentTypeJSON,
		Data:              []byte("key: value\n"),
	})
	if err == nil {
		t.Fatal("expected error for unsupported source type")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code=%v", status.Code(err))
	}
}

func TestUnsupportedTarget(t *testing.T) {
	m := newTestModule(t, Config{})
	ctx := context.Background()

	_, err := m.convertRPC(ctx, &serializationv1.ConvertRequest{
		SourceContentType: contracts.SafeContentTypeJSON,
		TargetContentType: "application/gob",
		Data:              []byte(`{}`),
	})
	if err == nil {
		t.Fatal("expected error for unsupported target type")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code=%v", status.Code(err))
	}
}

func TestJSONArray(t *testing.T) {
	m := newTestModule(t, Config{})
	ctx := context.Background()

	input := []byte(`[1,"two",{"three":3}]`)
	resp, err := m.convertRPC(ctx, &serializationv1.ConvertRequest{
		SourceContentType: contracts.SafeContentTypeJSON,
		TargetContentType: contracts.SafeContentTypeMsgpack,
		Data:              input,
	})
	if err != nil {
		t.Fatal(err)
	}

	var v []any
	if err := msgpack.Unmarshal(resp.Result, &v); err != nil {
		t.Fatalf("msgpack decode: %v", err)
	}
	if len(v) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(v))
	}
}

func TestInvalidJSON(t *testing.T) {
	m := newTestModule(t, Config{})
	ctx := context.Background()

	_, err := m.convertRPC(ctx, &serializationv1.ConvertRequest{
		SourceContentType: contracts.SafeContentTypeJSON,
		TargetContentType: contracts.SafeContentTypeMsgpack,
		Data:              []byte(`{invalid}`),
	})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code=%v", status.Code(err))
	}
}

func TestDecodeDepthLimit(t *testing.T) {
	m := newTestModule(t, Config{})
	ctx := context.Background()
	var b strings.Builder
	for i := 0; i < maxDecodeDepth+2; i++ {
		b.WriteString(`{"n":`)
	}
	b.WriteString(`1`)
	for i := 0; i < maxDecodeDepth+2; i++ {
		b.WriteByte('}')
	}
	_, err := m.convertRPC(ctx, &serializationv1.ConvertRequest{
		SourceContentType: contracts.SafeContentTypeJSON,
		TargetContentType: contracts.SafeContentTypeMsgpack,
		Data:              []byte(b.String()),
	})
	if err == nil {
		t.Fatal("expected depth limit error")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code=%v err=%v", status.Code(err), err)
	}
}

func TestLifecycle(t *testing.T) {
	t.Setenv("MUXCORE_INSECURE_DISABLE_TLS", "true")
	m, err := NewModule(Config{GRPCAddr: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Health(ctx); err != nil {
		t.Fatal("expected health to pass after start")
	}

	addr := m.ListenerAddr()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	rpc := serializationv1.NewSerializationServiceClient(conn)
	typesResp, err := rpc.SupportedTypes(ctx, &serializationv1.SupportedTypesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(typesResp.GetContentTypes()) != 2 {
		t.Fatalf("types=%v", typesResp.GetContentTypes())
	}

	convResp, err := rpc.Convert(ctx, &serializationv1.ConvertRequest{
		SourceContentType: contracts.SafeContentTypeJSON,
		TargetContentType: contracts.SafeContentTypeMsgpack,
		Data:              []byte(`{"live":true}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(convResp.GetResult()) == 0 {
		t.Fatal("expected convert result")
	}

	if err := m.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Health(ctx); err == nil {
		t.Fatal("expected health to fail after stop")
	}
}

func TestHealthBeforeInit(t *testing.T) {
	m := newTestModule(t, Config{})
	if err := m.Health(context.Background()); err == nil {
		t.Fatal("expected health to fail before init")
	}
}

func TestModuleLifecycleTLS(t *testing.T) {
	t.Setenv("MUXCORE_INSECURE_DISABLE_TLS", "")
	t.Setenv("MUXCORE_GRPC_INSECURE", "")

	dir := t.TempDir()
	t.Setenv("SERIALIZATION_TLS_DIR", dir)

	m, err := NewModule(Config{GRPCAddr: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := m.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := m.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestStopWithoutStartClosesListener(t *testing.T) {
	m, err := NewModule(Config{GRPCAddr: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if m.lis != nil {
		t.Fatal("listener should be nil after stop")
	}
}
