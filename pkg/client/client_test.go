package client_test

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/Muxcore-Media/core/pkg/contracts"
	serializationv1 "github.com/Muxcore-Media/core/proto/gen/muxcore/serialization/v1"
	"github.com/Muxcore-Media/serialization-safe/internal"
	"github.com/Muxcore-Media/serialization-safe/pkg/client"
)

const bufSize = 4 << 20

func startBufconnModule(t *testing.T) (*internal.Module, *bufconn.Listener) {
	t.Helper()
	mod, err := internal.NewModule(internal.Config{MaxPayloadBytes: bufSize})
	if err != nil {
		t.Fatal(err)
	}
	lis := bufconn.Listen(bufSize)
	mod.SetListener(lis)
	ctx := context.Background()
	if err := mod.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = mod.Stop(ctx)
		_ = lis.Close()
	})
	return mod, lis
}

func dialClient(t *testing.T, lis *bufconn.Listener) *client.Client {
	t.Helper()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return client.New(conn)
}

func TestClientMarshalUnmarshal(t *testing.T) {
	_, lis := startBufconnModule(t)
	cl := dialClient(t, lis)

	types := cl.SupportedTypes()
	if len(types) != 2 {
		t.Fatalf("types=%v", types)
	}

	type payload struct {
		Name  string `json:"name"`
		Count int64  `json:"count"`
	}
	original := payload{Name: "test", Count: 42}

	for _, ct := range []string{contracts.SafeContentTypeJSON, contracts.SafeContentTypeMsgpack} {
		data, err := cl.Marshal(ct, original)
		if err != nil {
			t.Fatalf("marshal %s: %v", ct, err)
		}
		var got payload
		if err := cl.Unmarshal(ct, data, &got); err != nil {
			t.Fatalf("unmarshal %s: %v", ct, err)
		}
		if got != original {
			t.Fatalf("%s: got %+v want %+v", ct, got, original)
		}
	}
}

func TestClientConvertRoundTrip(t *testing.T) {
	_, lis := startBufconnModule(t)
	cl := dialClient(t, lis)

	input := []byte(`{"count":42}`)
	mp, err := cl.Marshal(contracts.SafeContentTypeMsgpack, map[string]any{"count": 42})
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := cl.Unmarshal(contracts.SafeContentTypeMsgpack, mp, &v); err != nil {
		t.Fatal(err)
	}
	if _, ok := v["count"].(float64); !ok {
		if n, ok := v["count"].(int64); !ok || n != 42 {
			t.Fatalf("count=%v (%T)", v["count"], v["count"])
		}
	}
	_ = input
}

var _ contracts.SerializationProvider = (*client.Client)(nil)

func TestClientSupportedTypesRPC(t *testing.T) {
	_, lis := startBufconnModule(t)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	rpc := serializationv1.NewSerializationServiceClient(conn)
	resp, err := rpc.SupportedTypes(context.Background(), &serializationv1.SupportedTypesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetContentTypes()) != 2 {
		t.Fatalf("types=%v", resp.GetContentTypes())
	}
}
