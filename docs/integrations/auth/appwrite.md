# Integration notes: Appwrite

**Verdict:** **Not supported** as a native `Store` driver. Use HTTP from hooks if you need Appwrite as a backing service.

## Pattern

- Define a BFFX resource backed by your own logic in `hooks/*.go`, calling Appwrite REST with API keys or JWT from env.
- Keep manifests as the contract; treat Appwrite as an upstream dependency, not the persistence layer for core CRUD unless you fully own consistency.

## Gotchas

- No query translation from BFFX filters to Appwrite queries — you implement list/read semantics yourself.

See also: [`docs/getting-started/state_of_the_project.md`](../../getting-started/state_of_the_project.md).
