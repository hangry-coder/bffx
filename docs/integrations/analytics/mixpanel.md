# Integration notes: Mixpanel

**Verdict:** **Not native today** — **Planned / bridge** via hooks or outbound analytics.

## Options

1. **PostHog first:** If you already emit via `pkg/comm/analytics` (PostHog), forward or dual-write from a hook.
2. **Direct HTTP:** Call Mixpanel’s HTTP API from an `afterCreate` / action hook with server-held project token.

## Gotchas

- Respect PII: strip identifiers in hooks before sending third-party analytics.

See [`docs/integrations/posthog.md`](posthog.md) for the native analytics path.
