# Layr

The Single-Binary, Modular Developer Infrastructure & Control Plane for PostgreSQL 18+

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
![Zero Lock-in](https://img.shields.io/badge/lock--in-Zero%20(5--min%20exit)-success.svg)

Layr (`layr.sh`) is an open-source developer infrastructure platform engineered to eliminate the compromise between developer velocity and data sovereignty. It acts as an intelligent control plane and driver over standard **PostgreSQL 18+**, requiring **zero proprietary database extensions** and **zero mandatory external infrastructure** (no Redis, Kafka, or S3 required).

---

## 🏛️ Core Principles

- **User-Owned Data Sovereignty:** You always own your database and underlying data. Layr operates directly on standard PostgreSQL schemas using ANSI SQL, native `DEFAULT uuidv7()` primary keys, and standard Argon2id hashes.
- **5-Minute Exit Guarantee:** Zero vendor lock-in. Migrating away requires no data conversion—just run `pg_dump`.
- **Invariant Kernel + Modular Services:** A lightweight core runtime kernel (`layr/core`) with 8 opt-in, unbundled services (`data`, `auth`, `file-storage`, `tasks`, `notification`, `analytics`, `image`, `console`).
- **Dual Database Engine:** Connect to an external PostgreSQL 18+ cluster (`postgres://...`) or use the embedded native engine (`.layr/data`) for instant, zero-config local development in $<3$ seconds.
- **Monolith to Microservice Elasticity:** Run all enabled services in a single binary on a single machine or scale dedicated service pods horizontally (`layr start auth`, `layr start data`, `layr start file-storage`) with zero code changes.
- **Zero Taste Forcing:** No hardcoded rate limits, fixed cache TTLs, or unalterable lockout policies. Every policy is 100% configurable via the control plane.

---

## 🧩 Modular Services

Layr provides unbundled primitives that can be enabled independently or combined freely:

| Service                 | Replaces                            | Capabilities                                                                                                                                        |
| ----------------------- | ----------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| **`layr/data`**         | Hasura • PostGraphile • PostgREST   | Instant CRUD REST & GraphQL AST APIs, WebSocket Change Data Capture (CDC) subscriptions, and external JWT/OIDC Row-Level Security.                  |
| **`layr/auth`**         | Clerk • Auth0 • Logto • Keycloak    | Full OIDC/OAuth2 Identity Provider discovery (`/.well-known/openid-configuration`), Passkeys (WebAuthn), TOTP MFA, OTP, Argon2id, and Social OAuth. |
| **`layr/file-storage`** | MinIO • AWS S3 • Cloudflare R2      | S3-compatible REST API, presigned URLs, and zero-infra streamable PostgreSQL BYTEA chunking fallback.                                               |
| **`layr/tasks`**        | Temporal • BullMQ • AWS EventBridge | Distributed cron, decentralized transactional `SKIP LOCKED` task queues and job polling, and resilient HTTP webhook dispatchers.                    |
| **`layr/notification`** | Novu • Courier • Resend • Knock     | Multi-channel delivery engine (Email/SMTP, Twilio SMS, Push via FCM/APNs, Webhooks) and dynamic templating.                                         |
| **`layr/analytics`**    | Umami • Plausible • PostHog         | Privacy-first cookieless tracking script (`<script data-site="...">`), 46-field telemetry store, and monthly partitioned PostgreSQL tables.         |
| **`layr/image`**        | imgproxy • Cloudinary • Imgix       | 100% imgproxy-compatible API, HMAC-SHA256 URL signing, dynamic resizing, gravity cropping, and WebP/AVIF transcoding.                               |
| **`layr/console`**      | Retool • TablePlus • Prisma Studio  | Embedded web dashboard (`/console`), visual SQL editor, interactive schema explorer, and dynamic runtime configuration manager.                     |

---

## ⚡ Quickstart

### 1. Run Layr

```bash
# Install
# TBD

# Start with zero config (auto-generates layr.yaml and initializes embedded DB)
layr start
```

### 2. Access the Console

Open [http://localhost:8080/console](http://localhost:8080/console) to access the embedded control plane.

---

## 📦 Client SDKs

Layr provides official multi-language SDKs with fluent query builders, automatic retries, and symmetric service account elevation (`.cp`):

- 🌐 **[JavaScript / TypeScript SDK](sdks/javascript/README.md)** (`layr-client`)
- 🐍 **[Python SDK](sdks/python/README.md)** (`layr-client`)
- 🐹 **[Go SDK](sdks/go/README.md)** (`github.com/layr-sh/layr/sdks/go`)
- 🦀 **[Rust SDK](sdks/rust/README.md)** (`layr-client`)
- 🍎 **[Swift SDK](sdks/swift/README.md)** (`LayrClient`)

For architectural details and usage across all languages, see the **[SDK Ecosystem Overview](sdks/README.md)**.

---

## 📄 License

Layr core is open-source software licensed under the **Apache License 2.0**.
