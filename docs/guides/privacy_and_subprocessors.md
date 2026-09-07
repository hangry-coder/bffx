---
title: Privacy, Data Retention & Subprocessors
description: Comprehensive playbook for GDPR/CCPA compliance, self-hosted data taxonomy, and third-party egress profiles.
category: guides
---

# 🛡️ Privacy, Data Retention & Subprocessors

This document provides a comprehensive overview of how **BFFX** collects, processes, stores, and transfers data. As a self-hosted framework, **you** (the developer/enterprise deploying BFFX) are the **Data Controller**, and the infrastructure you run BFFX on (AWS, GCP, Hetzner, etc.) hosts the physical data stores.

Use this guide as your blueprint for understanding the default privacy posture of BFFX, enabling you to answer security questionnaires, draft privacy policies, and execute compliance playbooks (GDPR, CCPA/CPRA, etc.).

---

## 1. Core Architecture & Data Controllership

By default, BFFX runs as a **fully self-contained, self-hosted service**. It does not call home to any centralized BFFX telemetry or management servers.

- **Data Controller**: You (the developer or company building the application).
- **Data Processor**: The infrastructure/SaaS platforms you integrate with (e.g. AWS, Resend, PostHog).
- **Subprocessors**: Any opt-in third-party service wired via environment variables (detailed in Section 5).

```
┌────────────────────────────────────────────────────────┐
│                   YOUR HOSTED BOUNDARY                 │
│                                                        │
│  ┌──────────────┐      ┌──────────────┐                │
│  │  BFFX API    │ ───> │  Postgres    │                │
│  │  (Go Engine) │      │  (Core Data) │                │
│  └──────────────┘      └──────────────┘                │
│         │                                              │
│         ▼ (Opt-in Outbound HTTPS)                      │
└─────────┼──────────────────────────────────────────────┘
          │
          ├───> PostHog (Analytics)
          ├───> FCM (Push notifications)
          └───> Resend/SMTP (Transactional emails)
```

---

## 2. Default Data Taxonomy & Storage

The core BFFX engine creates several database tables and in-memory caches. Below is the detailed taxonomy of personal data (PII) and system metadata collected by default.

### 2.1 User Credentials and Identity
*   **Stored In**: `user` table (Postgres/SQLite).
*   **PII Category**: Personal identifiers (Email, Name).
*   **Security Guardrails**:
    *   Password hashes are stored using **bcrypt** with a default work factor cost of 10. Raw passwords are never written to disk, logged, or exposed in API payloads.
    *   OTP verification codes are hashed using **bcrypt** before storage. Once successfully validated, the database field `otp_code` is cleared (`""`) immediately to minimize lingering risk.

### 2.2 Device Tracking and Association
*   **Stored In**: `device` table, access logs.
*   **PII Category**: Device identifiers (`device_id`, client user-agent strings, OS type).
*   **Purpose**: Association of guest accounts with specific hardware, mapping active push notification tokens, and rate-limit tracking.

### 2.3 Authentication Sessions and Tokens
*   **Stored In**: JWT payloads (client-side), `refreshtoken` table (database), and Redis cache (if enabled).
*   **PII Category**: Cryptographic session identifiers, JTI (JWT ID), token rotation histories.
*   **Purpose**: Secure access control, refresh token rotation, and single-click global logout.
*   **Security Guardrails**:
    *   Token Revocation utilizes Redis for constant-time checks of blacklisted JTIs, allowing immediate invalidation of credentials.
    *   Refresh tokens are rotated on every use; reuse of an old refresh token instantly invalidates the entire family tree, protecting against token theft.

### 2.4 Background Worker and Skill Jobs
*   **Stored In**: `task` or `job` tables.
*   **PII Category**: Execution payloads, user-associated metadata, failure stack traces.
*   **Purpose**: Orchestrator scheduling for asynchronous work (e.g. emailing reports, syncing data).
*   **Privacy Warning**: If asynchronous tasks handle PII (e.g., passing raw email addresses as arguments), this data resides in the job database until garbage collected. Ensure that stack traces do not leak passwords or payment tokens.

---

## 3. Logs & Observability Egress

Standard structured logging in production can lead to **accidental data leakage (PII) to log aggregators** (e.g. Datadog, AWS CloudWatch, Axiom). BFFX implements built-in safeguards to reduce this risk.

### 3.1 HTTP Access Logs
*   Access logs emit method, path, HTTP status, request duration, and IP address.
*   **Automatic Scrubbing**: The `X-App-Secret`, `Authorization` headers, and cookies are automatically excluded from logs.
*   **IP Address Anonymization**: Client IP addresses (`ip` field) are logged by default for rate-limiting verification. If you require full GDPR IP-masking compliance, terminate client traffic at your reverse proxy (e.g. Caddy/Nginx) and apply subnet masking before passing headers.

### 3.2 Sentry Crash Reporting (Opt-In Tag)
When building BFFX with `-tags=sentry`, any unhandled panic or crash captures the runtime context.
*   **Safe Capture**: Sentry hooks in `pkg/observability/sentry_sdk.go` explicitly strip Cookie headers from event requests before sending them.
*   **Data Minimization**: Traces sample rates default to `0` to prevent bulk data transfer of routine customer transactions.

---

## 4. GDPR/CCPA Compliance Playbooks

As the Data Controller, you are legally responsible for fulfilling data subject access requests (DSARs).

### 4.1 Right to Erasure ("Right to be Forgotten")
To completely erase a user's data from a default BFFX deployment, execute the following SQL script:

```sql
-- 1. Identify the target User ID
-- SELECT id FROM "user" WHERE email = 'target-user@domain.com';
BEGIN;

-- 2. Delete active session tokens and family trees
DELETE FROM "refreshtoken" WHERE user_id = 'USER_UUID_HERE';

-- 3. Dissociate devices (preserves device identity but breaks link to personal profile)
UPDATE "device" SET user_id = NULL WHERE user_id = 'USER_UUID_HERE';

-- 4. Delete core user row (cascade delete on associated resources must be handled per-app)
DELETE FROM "user" WHERE id = 'USER_UUID_HERE';

COMMIT;
```

> [!IMPORTANT]
> If you have custom resources (e.g. `Task`, `Post`, `Note`), ensure your manifest relationship rules or database schema define `ON DELETE CASCADE`, or manually scrub associated rows prior to final user deletion.

> [!TIP]
> For a comprehensive, automated technical implementation playbook including custom Go hook code examples, tagged Redis cache invalidation, blob purges, and background job cancellations, refer to the [GDPR User Erasure Playbook](../compliance/gdpr_erasure.md).

### 4.2 Right to Portability ("Right to Know")
To export a user's entire profile for portability requests, query the database to output JSON:

```sql
SELECT json_build_object(
    'user', (SELECT row_to_json(u) FROM "user" u WHERE u.id = 'USER_UUID_HERE'),
    'devices', (SELECT json_agg(row_to_json(d)) FROM "device" d WHERE d.user_id = 'USER_UUID_HERE')
) AS export_payload;
```

---

## 5. Third-Party Egress & Subprocessors Matrix

BFFX provides native, **opt-in integrations** to support common production features. When activated via environment variables, data will egress your self-hosted boundary to the following subprocessors:

| Integration / Subprocessor | Purpose | PII Types Egressed | Activation Env Trigger | Locality Option |
|:---|:---|:---|:---|:---|
| **Resend** | Transactional Emails | Email address, name, message body | `RESEND_API_KEY` | SaaS only |
| **Firebase (FCM)** | Mobile Push Notifications | Device tokens, notification payloads | `BFFX_FCM_SERVICE_ACCOUNT_JSON` | SaaS only |
| **Stripe** | Payment & Billing | User IDs, email addresses, billing metadata | `BFFX_STRIPE_WEBHOOK_SECRET` | SaaS only |
| **PostHog** | Product Analytics | Device IDs, custom event parameters | `BFFX_POSTHOG_API_KEY` | SaaS / Self-Host |
| **S3 Compatible Store**| File & Asset Storage | Raw uploaded files (images, audio, logs) | `BFFX_S3_ACCESS_KEY` | SaaS / Self-Host |
| **Sentry** | Crash & Error Reporting | Runtime stack traces, request URLs, User ID | `BFFX_SENTRY_DSN` (sentry tag build) | SaaS / Self-Host |

---

## 6. In-Depth Subprocessor Profiles

### 6.1 Resend (Transactional Email Delivery)
*   **Framework Package**: `pkg/comm/email`
*   **Data Transferred**: Recipient email address (`To`), recipient name (optional), and message body.
*   **PII Sensitivity**: **High** (direct communication channels).
*   **Best Practices**: Ensure the body payloads of your transactional emails do not contain permanent passwords, sensitive health data, or unmasked credit card details.

### 6.2 Firebase Cloud Messaging (FCM - Push Notifications)
*   **Framework Package**: `pkg/comm/notifications`
*   **Data Transferred**: FCM registration device tokens, notification titles, bodies, and custom payload maps.
*   **PII Sensitivity**: **Medium** (handles device identifiers and message teasers).
*   **Data Minimization**: Push notifications are inherently exposed to APNs and Google Play services. BFFX recommends keeping notification payloads generic (e.g. `"You have a new message"`) and fetching the actual private content securely via API on app launch, rather than sending full PII in the push payload.

### 6.3 Stripe (Payment & Billing)
*   **Framework Package**: `pkg/addons/billing`
*   **Data Transferred**: Customer email address, unique User UUID (passed as `client_reference_id` or `metadata.user_id`), and active subscription identifiers.
*   **PII Sensitivity**: **High** (billing, payment status, customer identification).
*   **PCI Compliance**: BFFX does **not** process, store, or transmit credit card numbers. All card handling is deferred entirely to Stripe Elements/SDKs. BFFX only handles secure webhook callbacks via constant-time signature checks.

### 6.4 PostHog (Native Product Analytics)
*   **Framework Package**: `pkg/batteries/analytics`
*   **Data Transferred**: User UUID or anonymous `device_id`, custom event labels, and properties.
*   **PII Sensitivity**: **Low-Medium** (behavioral analytics, event trails).
*   **SaaS vs Self-Host**: PostHog can be self-hosted. By configuring `BFFX_POSTHOG_HOST` to your own self-hosted PostHog instance (e.g. `https://posthog.yourdomain.com`), you can maintain full network locality and keep behavioral analytics fully private.

### 6.5 S3-Compatible Storage (AWS S3, MinIO, Cloudflare R2)
*   **Framework Package**: `pkg/storage/blob`
*   **Data Transferred**: Raw binary files uploaded by mobile clients (user profile pictures, PDFs, files).
*   **PII Sensitivity**: **Variable** (depends entirely on what files users upload).
*   **Minimizing Egress**: If you require 100% data locality, you can point BFFX to a self-hosted **MinIO** cluster or activate local uploads on local disk, keeping all binary files within your physical boundary.

### 6.6 Sentry (Error Tracking & Crash Reporting)
*   **Framework Package**: `pkg/observability`
*   **Data Transferred**: Server stack traces, error messages, and HTTP request metadata.
*   **PII Sensitivity**: **Low-Medium** (internal stack frames and metadata).
*   **Privacy Protections**: BFFX automatically strips cookie strings and standard auth tokens before dispatching panic payloads to Sentry. If you require maximum data control, you can choose to self-host **GlitchTip** or Sentry on-premise and direct reports there.

---

## 7. Compliance Best Practices for Developers

To comply with global regulations (GDPR Article 28 / CCPA service provider requirements), ensure you execute the following:
1. **Data Processing Agreements (DPAs)**: Sign DPAs with any of the SaaS providers listed above that you activate in your production build.
2. **Opt-Out Mechanism**: Provide clients with simple options to disable optional tracking (e.g. turning off PostHog analytics from the mobile settings page).
3. **Data Retention Policies**: Configure automatic expiry on S3 buckets and log rotation in CloudWatch/Axiom to ensure customer metadata is routinely purged.
