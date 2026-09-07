# Billing & Stripe webhooks

## Stripe webhooks

When `BFFX_STRIPE_WEBHOOK_SECRET` is set to your endpoint signing secret (`whsec_...`), BFFX registers:

- `POST /api/v1/billing/stripe/webhook`

The handler:

1. Reads the **raw** request body (required for signature verification).
2. Validates the `Stripe-Signature` header (HMAC v1, ±300s clock skew).
3. Dedupes by Stripe `event.id` (Redis `SET NX` when `BFFX_REDIS_URL` is configured; otherwise in-memory — single-instance only).
4. Dispatches to `billing.StripeEventHooks` (hook struct with optional `OnEvent` callback).

### Durable idempotency hardening

- Set `BFFX_STRIPE_STRICT_DURABLE_IDEMPOTENCY=true` in production to require a non-memory dedupe backend.
- In strict mode, webhook requests return `503` when running with memory-only dedupe (prevents silent replay risk on restarts).
- Pair strict mode with Redis so Stripe event dedupe survives process restarts.

### Event mapping

Implement `OnEvent` and switch on `eventType`, for example:

- `customer.subscription.created`
- `customer.subscription.updated`
- `customer.subscription.deleted`
- `invoice.paid`

The `data` argument is the raw JSON `data` object from the Stripe envelope (typically includes `object` for the nested resource).

### Operational notes

- Return **2xx** quickly; Stripe retries on non-2xx.
- Replays with the same `event.id` return **200** with `duplicate: true` and do **not** invoke `OnEvent` again.
