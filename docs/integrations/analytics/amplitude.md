# Integration notes: Amplitude

**Verdict:** **Not native today** — same **bridge** story as Mixpanel.

## Pattern

- Use Amplitude HTTP API from hooks, or forward from PostHog / your own event pipeline.
- Keep secrets on the server; never embed API keys in generated clients.

See [`docs/integrations/mixpanel.md`](mixpanel.md) and [`docs/integrations/posthog.md`](posthog.md).
