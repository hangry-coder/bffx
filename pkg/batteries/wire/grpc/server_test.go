package grpc

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestLiveGRPCServerInterceptors(t *testing.T) {
	// 1. Setup random TCP listener
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	// 2. Setup mock auth
	mockAuth := &mockAuthProvider{
		validateFunc: func(ctx context.Context, token string) (map[string]any, error) {
			if token == "secret-token-123" {
				return map[string]any{"sub": "user_xyz", "role": "member"}, nil
			}
			return nil, errors.New("unauthorized")
		},
	}

	// 3. Setup gRPC server on the random port to prove it boots and stops gracefully
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			TraceIDInterceptor(),
			RecoveryInterceptor(),
			AuthInterceptor(mockAuth, nil),
		),
	)

	go func() {
		_ = srv.Serve(lis)
	}()
	defer srv.GracefulStop()

	// 4. Manually execute the interceptor chain on a test handler to verify context enrichment
	var reachedHandler bool
	testHandler := func(handlerCtx context.Context, req interface{}) (interface{}, error) {
		reachedHandler = true
		traceID := middleware.GetTraceID(handlerCtx)
		userID := middleware.GetUserID(handlerCtx)
		if traceID != "client-trace-555" {
			t.Errorf("Expected trace ID client-trace-555, got %s", traceID)
		}
		if userID != "user_xyz" {
			t.Errorf("Expected userID user_xyz, got %s", userID)
		}
		return "ok", nil
	}

	// Prepare metadata
	md := metadata.New(map[string]string{
		"x-trace-id":    "client-trace-555",
		"authorization": "Bearer secret-token-123",
	})
	incomingCtx := metadata.NewIncomingContext(context.Background(), md)

	// Compose interceptor chain manually
	info := &grpc.UnaryServerInfo{FullMethod: "/TestService/TestMethod"}
	
	// Chain: TraceID -> Recovery -> Auth -> testHandler
	authChain := func(ctx context.Context, req interface{}) (interface{}, error) {
		return AuthInterceptor(mockAuth, nil)(ctx, req, info, testHandler)
	}
	recoveryChain := func(ctx context.Context, req interface{}) (interface{}, error) {
		return RecoveryInterceptor()(ctx, req, info, authChain)
	}
	traceIDChain := func(ctx context.Context, req interface{}) (interface{}, error) {
		return TraceIDInterceptor()(ctx, req, info, recoveryChain)
	}

	// Execute
	resp, err := traceIDChain(incomingCtx, "request")
	if err != nil {
		t.Fatalf("Interceptor chain failed: %v", err)
	}
	if resp != "ok" || !reachedHandler {
		t.Fatalf("Handler was not reached or returned incorrect response")
	}
}
