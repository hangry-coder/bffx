# Integration notes: Firebase

**Verdict:** **FCM (push)** — **Beta** in BFFX (HTTP v1). Firebase Auth, Firestore, and Cloud Functions are **client-side or hook-bridge** concerns, not generated first-class APIs.

## What works today

- **Push:** Configure `BFFX_FCM_SERVICE_ACCOUNT_JSON` (or path) per [`docs/guides/operations.md`](../../guides/operations.md) and [`pkg/comm/notifications`](../../pkg/comm/notifications).

## What you bridge yourself

- **Auth / Firestore:** Use Firebase SDKs on device, or call Firebase Admin APIs from BFFX hooks with a service account stored as env/secret — manifests do not model Firestore collections.

## Gotchas

- Do not expose service account JSON to generated mobile clients; keep FCM and Admin credentials on the server only.

See also: [`docs/getting-started/state_of_the_project.md`](../../getting-started/state_of_the_project.md).
