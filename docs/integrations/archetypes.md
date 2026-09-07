# Vertical Archetype Integrations Map

BFFX vertical archetypes come pre-wired with third-party service connections and analytical/monetization batteries. This document maps each archetype to its key external integrations and links to the relevant integration guides.

## Integration Registry by Archetype

| CLI Alias | Key Integrations | Primary Auth Provider | Billing & Monetization | Analytics / Obser. |
|---|---|---|---|---|
| **`superapp`** | FCM, Stripe, PostHog, Sentry | Clerk / WorkOS | Stripe (via Entitlements) | PostHog + Sentry |
| **`fintech`** | Plaid (mock), PostHog | Clerk | — | PostHog |
| **`marketplace`** | Google Maps (mock), FCM | Clerk | — | PostHog |
| **`microlearn`** | PostHog | Clerk | — | PostHog |
| **`dictation`** | Groq (mock) | WorkOS | Stripe | PostHog |
| **`subscriptions`** | Gmail (mock) | Clerk | Stripe | PostHog |
| **`commerce`** | Stripe (mock) | Clerk | Stripe (Checkout API) | PostHog |
| **`web3`** | Alchemy (mock) | Clerk | — | PostHog |
| **`iot`** | MQTT (mock) | Clerk | — | PostHog |
| **`notes`** | — | Builtin | — | — |

---

## Integration Guide Directory

Use the following references to configure production credentials for the pre-wired batteries:

* **Authentication (Clerk / WorkOS):** See [Auth Integration Guide](../integrations/auth/) for setting up `CLERK_API_KEY` or `WORKOS_API_KEY`.
* **Payments & Billing (Stripe):** See [Stripe Payments Integration](../integrations/payments/stripe.md) for configuring webhooks and products.
* **Analytics & Telemetry (PostHog / Mixpanel / Amplitude):** See [PostHog Integration](../integrations/analytics/posthog.md) to route client-side and server-side telemetry events.
* **Push Notifications (FCM):** See FCM documentation to hook mobile push payloads.
