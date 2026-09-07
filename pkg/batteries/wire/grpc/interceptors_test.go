package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hangry-coder/bffx/pkg/api/middleware"
	cachebattery "github.com/hangry-coder/bffx/pkg/batteries/cache"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

// Mock Auth Provider
type mockAuthProvider struct {
	validateFunc func(ctx context.Context, token string) (map[string]any, error)
}

func (m *mockAuthProvider) ValidateToken(ctx context.Context, token string) (map[string]any, error) {
	if m.validateFunc != nil {
		return m.validateFunc(ctx, token)
	}
	return nil, errors.New("unauthorized")
}

func (m *mockAuthProvider) UserIDFromClaims(claims map[string]any) string {
	if sub, ok := claims["sub"].(string); ok {
		return sub
	}
	return ""
}

func (m *mockAuthProvider) Issuer() string {
	return "mock"
}

func (m *mockAuthProvider) Type() string {
	return "mock"
}

func TestRecoveryInterceptor(t *testing.T) {
	interceptor := RecoveryInterceptor()

	// Handler that panics
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		panic("boom")
	}

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Expected gRPC status error, got %v", err)
	}

	if st.Code() != codes.Internal {
		t.Errorf("Expected Internal status code, got %v", st.Code())
	}
}

func TestTraceIDInterceptor(t *testing.T) {
	interceptor := TraceIDInterceptor()

	// 1. With existing metadata
	md := metadata.New(map[string]string{
		"x-trace-id":   "existing-trace-123",
		"x-request-id": "existing-req-456",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		traceID := middleware.GetTraceID(ctx)
		reqID := middleware.GetRequestID(ctx)
		if traceID != "existing-trace-123" {
			t.Errorf("Expected trace ID 'existing-trace-123', got '%s'", traceID)
		}
		if reqID != "existing-req-456" {
			t.Errorf("Expected request ID 'existing-req-456', got '%s'", reqID)
		}
		return nil, nil
	}

	_, _ = interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)

	// 1b. With correlation alias metadata only
	mdCorrelation := metadata.New(map[string]string{
		"x-correlation-id": "corr-trace-789",
	})
	ctxCorrelation := metadata.NewIncomingContext(context.Background(), mdCorrelation)
	handlerCorrelation := func(ctx context.Context, req interface{}) (interface{}, error) {
		traceID := middleware.GetTraceID(ctx)
		reqID := middleware.GetRequestID(ctx)
		if traceID != "corr-trace-789" {
			t.Errorf("Expected trace ID 'corr-trace-789', got '%s'", traceID)
		}
		if reqID != "corr-trace-789" {
			t.Errorf("Expected request ID 'corr-trace-789', got '%s'", reqID)
		}
		return nil, nil
	}
	_, _ = interceptor(ctxCorrelation, nil, &grpc.UnaryServerInfo{}, handlerCorrelation)

	// 1c. x-trace-id takes precedence over x-correlation-id
	mdBoth := metadata.New(map[string]string{
		"x-trace-id":       "trace-preferred-001",
		"x-correlation-id": "corr-secondary-001",
	})
	ctxBoth := metadata.NewIncomingContext(context.Background(), mdBoth)
	handlerBoth := func(ctx context.Context, req interface{}) (interface{}, error) {
		traceID := middleware.GetTraceID(ctx)
		if traceID != "trace-preferred-001" {
			t.Errorf("Expected trace ID 'trace-preferred-001', got '%s'", traceID)
		}
		return nil, nil
	}
	_, _ = interceptor(ctxBoth, nil, &grpc.UnaryServerInfo{}, handlerBoth)

	// 2. Without metadata (should generate new trace ID)
	handlerGenerate := func(ctx context.Context, req interface{}) (interface{}, error) {
		traceID := middleware.GetTraceID(ctx)
		reqID := middleware.GetRequestID(ctx)
		if traceID == "" || traceID == "unknown" {
			t.Errorf("Expected generated trace ID, got '%s'", traceID)
		}
		if reqID == "" || reqID == "unknown" {
			t.Errorf("Expected generated request ID, got '%s'", reqID)
		}
		return nil, nil
	}

	_, _ = interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handlerGenerate)
}

func TestAuthInterceptor(t *testing.T) {
	// Mock Auth Provider returns valid claim for token "valid-token"
	mockAuth := &mockAuthProvider{
		validateFunc: func(ctx context.Context, token string) (map[string]any, error) {
			if token == "valid-token" {
				return map[string]any{"sub": "user_99", "role": "admin"}, nil
			}
			return nil, errors.New("invalid token")
		},
	}

	interceptor := AuthInterceptor(mockAuth, nil)

	// 1. Valid token
	md := metadata.New(map[string]string{
		"authorization": "Bearer valid-token",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		userID := middleware.GetUserID(ctx)
		if userID != "user_99" {
			t.Errorf("Expected userID 'user_99', got '%s'", userID)
		}
		return nil, nil
	}

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// 2. Invalid token
	mdInvalid := metadata.New(map[string]string{
		"authorization": "Bearer wrong-token",
	})
	ctxInvalid := metadata.NewIncomingContext(context.Background(), mdInvalid)

	_, err = interceptor(ctxInvalid, nil, &grpc.UnaryServerInfo{}, handler)
	if err == nil {
		t.Fatal("Expected authentication failure error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		t.Errorf("Expected Unauthenticated status, got %v", err)
	}
}

func TestUnaryCacheInterceptor(t *testing.T) {
	// 1. Setup Memory Cache Provider
	provider := cachebattery.NewMemoryProvider()

	// 2. Define Cache TTL policy
	fullMethod := "/testapp.wire.v1.MealService/ListMeals"
	ttls := map[string]time.Duration{
		fullMethod: 10 * time.Second,
	}

	interceptor := UnaryCacheInterceptor(provider, ttls)

	// Create test request and response
	reqStruct, _ := structpb.NewStruct(map[string]any{"limit": float64(10)})
	resStruct, _ := structpb.NewStruct(map[string]any{"data": []any{"pizza", "pasta"}})

	var callCount int
	handler := func(ctx context.Context, r interface{}) (interface{}, error) {
		callCount++
		return resStruct, nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: fullMethod}
	ctx := context.Background()

	// 3. First execution: Cache Miss
	resp1, err := interceptor(ctx, reqStruct, info, handler)
	if err != nil {
		t.Fatalf("First interceptor call failed: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected handler call count 1, got %d", callCount)
	}

	resStruct1 := resp1.(*structpb.Struct)
	if resStruct1.Fields["data"].GetListValue().Values[0].GetStringValue() != "pizza" {
		t.Errorf("Expected pizza, got %v", resStruct1)
	}

	// 4. Second execution: Cache Hit (Handler should NOT be reached)
	resp2, err := interceptor(ctx, reqStruct, info, handler)
	if err != nil {
		t.Fatalf("Second interceptor call failed: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected handler call count to remain 1 (cache hit), got %d", callCount)
	}

	resStruct2 := resp2.(*structpb.Struct)
	if resStruct2.Fields["data"].GetListValue().Values[0].GetStringValue() != "pizza" {
		t.Errorf("Expected pizza on cache hit, got %v", resStruct2)
	}

	// 5. Invalidation test: Invalidate the cache pattern
	// Since both REST and gRPC share prefix "bffx:action-cache:",
	// calling InvalidatePattern with "bffx:action-cache:*" must clear it.
	deleted, err := provider.InvalidatePattern(ctx, "bffx:action-cache:*")
	if err != nil {
		t.Fatalf("Invalidation failed: %v", err)
	}
	if deleted == 0 {
		t.Error("Expected at least one cached entry to be deleted")
	}

	// 6. Third execution: Cache Miss again
	resp3, err := interceptor(ctx, reqStruct, info, handler)
	if err != nil {
		t.Fatalf("Third interceptor call failed: %v", err)
	}
	if callCount != 2 {
		t.Errorf("Expected handler call count to increment to 2 (cache miss after invalidation), got %d", callCount)
	}

	resStruct3 := resp3.(*structpb.Struct)
	if resStruct3.Fields["data"].GetListValue().Values[0].GetStringValue() != "pizza" {
		t.Errorf("Expected pizza, got %v", resStruct3)
	}
}
