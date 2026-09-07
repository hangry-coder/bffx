# Integration notes: PostHog

**Verdict:** **Beta** — native batching driver in `pkg/comm/analytics`; wired through `ActionContext.Analytics` (Week 8 OSS track).

## What works today

- Server-side product analytics with env-configured PostHog host + key per [`docs/guides/operations.md`](../../guides/operations.md) and state of the project.

## Gotchas

- Flush on shutdown is tied to server lifecycle; long CLI-only processes should call flush explicitly if you add custom entrypoints.

See also: [`docs/getting-started/state_of_the_project.md`](../../getting-started/state_of_the_project.md), [`docs/guides/billing.md`](../../guides/billing.md) (separate concern from analytics).
