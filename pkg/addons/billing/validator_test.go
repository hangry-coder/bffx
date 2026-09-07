package billing

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAppleValidator_Verify_Mock(t *testing.T) {
	v := &AppleValidator{Secret: "test-secret"}
	ctx := context.Background()

	res, err := v.Verify(ctx, "MOCK_APPLE_RECEIPT_data", "premium_monthly")
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Contains(t, res.TransactionID, "tx_mock_apple_")
	assert.Equal(t, "premium_monthly", res.ProductID)
	assert.True(t, res.IsSubscription)
	assert.Equal(t, "active", res.Status)
	assert.True(t, res.ExpiresAt.After(time.Now()))
}

func TestGoogleValidator_Verify_Mock(t *testing.T) {
	v := &GoogleValidator{ConfigJSON: ""}
	ctx := context.Background()

	res, err := v.Verify(ctx, "MOCK_GOOGLE_RECEIPT_data", "premium_yearly")
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Contains(t, res.TransactionID, "tx_mock_google_")
	assert.Equal(t, "premium_yearly", res.ProductID)
	assert.True(t, res.IsSubscription)
	assert.Equal(t, "active", res.Status)
}
