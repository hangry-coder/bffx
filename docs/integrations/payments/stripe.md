---
title: Stripe Payments Integration
description: Complete step-by-step tutorial on integrating Stripe SDK, webhooks, hooks, and mobile workflows with BFFX.
category: integrations
---

# 💳 Stripe Payments Integration

This integration guide explains how to connect Stripe for subscriptions, payment intents, and transactional flows using BFFX Hooks, webhooks, and the billing addon.

---

## 1. Overview & Stability

*   **Status**: **Beta**
*   **Core Logic Location**: Unified billing operations reside under `pkg/addons/billing`.
*   **Webhooks**: Standard routes listen on `POST /api/v1/billing/stripe/webhook` with signature verification.

---

## 2. Implementation Guide

Follow these steps to integrate Stripe payments within your BFFX application:

### Step 1: Install Stripe Go SDK
Add the Stripe SDK dependency to your project's Go module:
```bash
go get github.com/stripe/stripe-go/v74
```

### Step 2: Define Payment Resource
Create a declarative manifest to track transaction records in `bffx/resources/payment.yaml`:
```yaml
apiVersion: bffx.io/v1alpha1
kind: Resource
metadata:
  name: payment
spec:
  fields:
    - { name: amount, type: int, required: true }
    - { name: currency, type: string, required: true }
    - { name: status, type: string }
    - { name: stripe_id, type: string }
  policy:
    read: owner
    write: owner
```

### Step 3: Implement `BeforeCreate` Hook
In `internal/hooks/payment.go`, intercept creation requests to generate a Stripe `PaymentIntent` synchronously before saving the record locally:

```go
package hooks

import (
    "context"
    "os"
    "github.com/stripe/stripe-go/v74"
    "github.com/stripe/stripe-go/v74/paymentintent"
)

func BeforeCreatePayment(ctx context.Context, data map[string]interface{}) error {
    // Configure Stripe API key from secure environment variable
    stripe.Key = os.Getenv("STRIPE_SECRET_KEY")

    amountVal, _ := data["amount"].(int)
    currencyVal, _ := data["currency"].(string)

    params := &stripe.PaymentIntentParams{
        Amount:   stripe.Int64(int64(amountVal)),
        Currency: stripe.String(currencyVal),
    }

    pi, err := paymentintent.New(params)
    if err != nil {
        return err
    }

    // Attach Stripe transaction token to our local record
    data["stripe_id"] = pi.ID
    data["status"] = "pending"
    
    return nil
}
```

---

## 3. Webhook Integration & Security

Stripe handles asynchronous event notifications (like successful subscriptions or charge failures) via webhooks.

### 3.1 Webhook Path
BFFX registers the following standard endpoint for webhook ingestion:
`POST /api/v1/billing/stripe/webhook`

### 3.2 Secure Webhook Verification
Never trust raw webhook payloads. In production, signature verification is **mandatory**. Configure the webhook signing secret in your environment:

```bash
export BFFX_STRIPE_WEBHOOK_SECRET="whsec_your-signing-secret"
```

When set, the BFFX billing addon automatically validates the `Stripe-Signature` header, rejecting unauthenticated or tampered webhook events.

---

## 4. Mobile Integration Flow

1. **Initiate Payment**: The mobile app calls `POST /api/v1/payments` with `{ amount: 1000, currency: "usd" }`.
2. **Collect Token**: The BFFX API responds with `{ id: "...", stripe_id: "pi_3M...", status: "pending" }`.
3. **Execute Checkout**: The mobile app uses the `stripe_id` (client secret) to present the Stripe Payment Sheet via native Flutter or Swift SDKs.
4. **Fulfill**: Stripe triggers a webhook once the card charge completes, updating the payment record status in BFFX to `completed`.
