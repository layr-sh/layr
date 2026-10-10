# Layr Multi-Language SDK Ecosystem (`client-sdk/`)

Official client SDKs for Layr, an open-source single-binary Backend-as-a-Service (BaaS) and application platform built on PostgreSQL 18+.

Target SDKs:

- JavaScript/TypeScript (`client-sdk/javascript`)
- Python (`client-sdk/python`)
- Go (`client-sdk/go`)
- Rust (`client-sdk/rust`)
- Swift (`client-sdk/swift`)

---

## 1. Multi-Language Packages & Repositories

| Language | Directory | Package Identifier | Package Manager | Standard Client Import |
| --- | --- | --- | --- | --- |
| JavaScript / TypeScript | `client-sdk/javascript/` | `layr-client` | npm / pnpm / yarn / bun | `import { createClient, useData } from 'layr-client'` |
| Python | `client-sdk/python/` | `layr-client` (`layr_client`) | pip / uv / poetry | `from layr_client import create_client, use_data` |
| Go | `client-sdk/go/` | `github.com/layr-sh/layr/client-sdk/go` | Go Modules | `import layrclient "github.com/layr-sh/layr/client-sdk/go"` |
| Rust | `client-sdk/rust/` | `layr-client` (`layr_client`) | Cargo / crates.io | `use layr_client::{create_client, use_data};` |
| Swift | `client-sdk/swift/` | `LayrClient` | Swift Package Manager | `import LayrClient` |

---

## 2. 4-Tier SDK Architecture

Layr SDKs implement a 4-tier architectural pattern:

```text
┌──────────────────────────────────────────────────────────────────────────────┐
│  Tier 4: Service Account Elevation & Control Plane (.cp)                     │
│  withServiceAccount(service, { serviceAccountKey }) -> ServiceWithCP         │
├──────────────────────────────────────────────────────────────────────────────┤
│  Tier 3: Client Context & Functional Service Hooks                           │
│  createClient(endpointURL, options) -> LayrClient                            │
│  useData(client), useAuth(client), useFileStorage(client), ...               │
├──────────────────────────────────────────────────────────────────────────────┤
│  Tier 2: Fluent Value-Add Builders (Data, File Storage, Image)               │
│  - QueryBuilder: .from('table').select().eq().order().limit()                │
│  - BucketClient: .from('bucket').upload().download().createSignedUrl()       │
│  - ImageURLBuilder: .url('source').resize().format().quality().sign()        │
├──────────────────────────────────────────────────────────────────────────────┤
│  Tier 1: Automated OpenAPI 3.1 Codegen                                       │
│  Auto-generated in client-sdk/<lang>/generated/                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Layer 1: Automated OpenAPI Codegen (`generated/`)

- Exported from Go route controllers via `./scripts/sdk/generate-openapi-specs`.
- Generated into `client-sdk/<language>/generated/` using `./scripts/sdk/generate`.
- Standard API routes are callable on their respective service client (from hooks such as `useAuth(client)`, `useData(client)`, `useCore(client)`) matching their OpenAPI Operation ID (`SDKGroupName` and `SDKMethodName`, e.g. `auth.signIn(...)`, `data.records.list(...)`, `core.healthz(...)`). The central client (`client`) provides shared transport and configuration.
- Generated code in `generated/` is not edited by hand; custom methods belong in service wrappers.

### Layer 2: Fluent Builders (Data, File Storage, Image)

- Fluent builders are provided for Data, File Storage, and Image. Other services are invoked directly through Tier 1 generated methods:
  - Data Service: Query builder (`data.from('todos').select('*').eq(...)`) and CDC subscription manager (`data.subscribe('todos')`).
  - File Storage Service: Bucket client (`fileStorage.from('avatars').upload(...)`, `.createSignedUrl(...)`).
  - Image Service: URL builder (`image.url(source).resize(...).format(...).sign(...)`).

### Layer 3: Client Factory & Service Hooks

- Unified factory function (`createClient` / `layrclient.New`) returning client context.
- Modular functional hooks (`useData`, `useAuth`, `useFileStorage`, `useFunction`, `useImage`, `useTasks`, `useCore`).

### Layer 4: Service Account Elevation & `.cp` Namespace

- Base service clients expose base client APIs.
- Control plane APIs (DDL, schema alterations, user administration, scheduled jobs, event hooks, system configuration) are grouped under `.cp` (Go: `.CP`) after elevation via `withServiceAccount`.
- Inter-service M2M communication uses the OAuth 2.0 / OIDC `client_credentials` flow in `auth` (`auth.oidc.token`), which issues scoped M2M JWT tokens passed via `Authorization: Bearer <token>`. Service account key elevation (`withServiceAccount`) is specifically for control plane (`.cp`) operations.

---

## 3. Core SDK Invariants

### 1. Control Plane Namespace (`.cp`)

Administrative, DDL, and configuration endpoints use `.cp` (Go: `.CP`), attached upon `withServiceAccount` elevation.

### 2. Error Translation

Non-2xx HTTP responses are parsed into standard Layr API error structures.

### 3. Transport Resilience

SDK transports retry transient rate limits (`429`) and server unavailability (`503`). Retries use exponential backoff with full jitter and honor the `Retry-After` header when present.

### 4. Security Headers & Credentials

Outgoing HTTP requests include credentials based on client mode and elevation:

| Header Name | Token Type | Target Security Domain |
| --- | --- | --- |
| `Publishable-Key` (fallback `X-Publishable-Key`) | Publishable API Key | Browser / mobile client; enforces PostgreSQL Row-Level Security (RLS) |
| `Authorization: Bearer <token>` | User Session JWT or M2M JWT | Authenticated user session or M2M client (issued via `auth.oidc.token` `client_credentials`); enforces PostgreSQL RLS |
| `Service-Account-Key` (fallback `X-Service-Account-Key`) | Service Account Secret | Elevated backend service account; bypasses RLS, enables `.cp` operations |

Keys (both publishable keys and service account keys) do not use prefixes like `pk_` or `sa_`; they are raw 32-byte hex strings.

### 5. Identifier & Naming Conventions

Standardized identifiers across languages without abbreviations:

- `endpointURL` / `endpointUrl` (not `baseUrl`, or `url`)
- `accountID` / `accountId`, `serviceAccountKey`, `serviceAccountID` / `serviceAccountId`
- `webhookID` / `webhookId`
- `timeoutMs`, `initialDelayMs`, `maxDelayMs`
- Standard English naming: `is_enabled`, `properties`, `enabled_services`.

### 6. Test Coverage

SDK code achieves 100.0% test coverage across statements, branches, and functions, verified with language-native coverage tools:

- JavaScript / TypeScript: `vitest run --coverage`
- Python: `pytest --cov=layr_client --cov-report=term-missing`
- Go: `go test -cover -coverprofile=...`
- Rust: `cargo llvm-cov`
- Swift: `swift test --enable-code-coverage`

---

## 4. Implementation Checklist

Using JavaScript/TypeScript (`client-sdk/javascript`) as the reference implementation:

### Phase 1: Core Foundation & Resilient Transport

- [ ] Endpoint URL normalization (`normalizeEndpointURL`):
  - Trims trailing slashes and defaults to `http://localhost:5297` if empty.
- [ ] Header injection (`buildHeaders`):
  - Injects `Publishable-Key` for base client requests.
  - Injects `Authorization: Bearer <token>` for user or M2M sessions (strips redundant leading `Bearer` if present).
  - Injects `Service-Account-Key` for elevated control plane (`.cp`) calls.
  - Merges custom headers without overwriting security headers unless explicitly configured.
- [ ] Error handling (`LayrAPIError`):
  - Parses standard Layr error JSON responses with fallback when body is non-JSON or missing.
- [ ] Resilient transport (`createFetchWithRetry`):
  - Retries HTTP 429 and 503 responses.
  - Parses `Retry-After` header (integer seconds and RFC 1123 HTTP-Date).
  - Applies exponential backoff with jitter when `Retry-After` is omitted (`initialDelayMs`, `maxDelayMs`, `maxRetries`).
  - Drains response bodies prior to retry.
  - Implements request timeout cancellation via `timeoutMs`.
- [ ] Session token mutability (`setSession` / `clearSession`):
  - Mutates active HTTP headers so instantiated service clients reflect the updated session.

---

### Phase 2: Tier 1 Standard Base APIs (Direct Operation ID Mapping)

Standard base APIs are callable on their service client instance (from hooks such as `useAuth(client)`, `useData(client)`, etc.) matching their OpenAPI Operation ID (`<service>.<subgroups>.<method>()`).

<!-- BEGIN_GENERATED_BASE_METHODS -->

#### `auth` (Auth Service — via `useAuth(client)`)

- Root methods (`auth.*`):
  - [ ] `auth.sign_in()`: Authenticate with email/phone and password (`POST /v1/auth/sign-in` | `auth__sign_in`)
  - [ ] `auth.sign_out()`: Revoke active session and refresh token (`POST /v1/auth/sign-out` | `auth__sign_out`)
  - [ ] `auth.sign_up()`: Register a new user with email/phone and password (`POST /v1/auth/sign-up` | `auth__sign_up`)

- Subgroup `auth.anonymous.*`:
  - [ ] `auth.anonymous.sign_in()`: Sign in or initialize an anonymous guest user account (`POST /v1/auth/anonymous` | `auth__anonymous__sign_in`)

- Subgroup `auth.mfa.*`:
  - [ ] `auth.mfa.challenge()`: Verify MFA challenge ticket with TOTP code (`POST /v1/auth/mfa/challenge` | `auth__mfa__challenge`)
  - [ ] `auth.mfa.disable()`: Disable multi-factor authentication on user account (`DELETE /v1/auth/mfa` | `auth__mfa__disable`)
  - [ ] `auth.mfa.setup()`: Generate TOTP secret and setup URI for authenticator apps (`POST /v1/auth/mfa` | `auth__mfa__setup`)
  - [ ] `auth.mfa.verify()`: Verify 2FA TOTP code and enable MFA on account (`POST /v1/auth/mfa/verify` | `auth__mfa__verify`)

- Subgroup `auth.oidc.*`:
  - [ ] `auth.oidc.token()`: Exchange authorization code or refresh token for OpenID Connect tokens (`POST /v1/auth/oauth/token` | `auth__oidc__token`)
  - [ ] `auth.oidc.userinfo()`: Retrieve OpenID Connect Core 1.0 user claims (`GET /v1/auth/oauth/userinfo` | `auth__oidc__userinfo`)

- Subgroup `auth.otp.*`:
  - [ ] `auth.otp.send()`: Request a 6-digit one-time password code via email or SMS (`POST /v1/auth/otp` | `auth__otp__send`)
  - [ ] `auth.otp.verify()`: Verify 6-digit OTP code and issue authenticated session (`POST /v1/auth/otp/verify` | `auth__otp__verify`)

- Subgroup `auth.passkeys.sign_in.*`:
  - [ ] `auth.passkeys.sign_in.begin()`: Begin WebAuthn passkey authentication ceremony (`POST /v1/auth/passkeys/sign-in` | `auth__passkeys__sign_in__begin`)
  - [ ] `auth.passkeys.sign_in.verify()`: Verify WebAuthn passkey assertion signature (`POST /v1/auth/passkeys/sign-in/verify` | `auth__passkeys__sign_in__verify`)

- Subgroup `auth.passkeys.sign_up.*`:
  - [ ] `auth.passkeys.sign_up.begin()`: Begin WebAuthn passkey registration ceremony (`POST /v1/auth/passkeys/sign-up` | `auth__passkeys__sign_up__begin`)
  - [ ] `auth.passkeys.sign_up.verify()`: Verify WebAuthn passkey registration attestation (`POST /v1/auth/passkeys/sign-up/verify` | `auth__passkeys__sign_up__verify`)

- Subgroup `auth.password_reset.*`:
  - [ ] `auth.password_reset.confirm()`: Confirm password reset with verification code and new password (`POST /v1/auth/password-reset/confirm` | `auth__password_reset__confirm`)
  - [ ] `auth.password_reset.request()`: Request a password reset verification code via email or SMS (`POST /v1/auth/password-reset/request` | `auth__password_reset__request`)

- Subgroup `auth.sessions.*`:
  - [ ] `auth.sessions.list()`: List active sessions for current user (`GET /v1/auth/sessions` | `auth__sessions__list`)
  - [ ] `auth.sessions.revoke()`: Revoke specific user session (`DELETE /v1/auth/sessions/{session_id}` | `auth__sessions__revoke`)
  - [ ] `auth.sessions.revoke_others()`: Sign out of all other devices (`POST /v1/auth/sessions/revoke-others` | `auth__sessions__revoke_others`)

- Subgroup `auth.token.*`:
  - [ ] `auth.token.refresh()`: Exchange refresh token for fresh JWT access token (`POST /v1/auth/token/refresh` | `auth__token__refresh`)

- Subgroup `auth.user.*`:
  - [ ] `auth.user.delete()`: Permanently delete authenticated user account (`DELETE /v1/auth/user` | `auth__user__delete`)
  - [ ] `auth.user.export()`: Export comprehensive current user account data graph (`POST /v1/auth/user/export` | `auth__user__export`)
  - [ ] `auth.user.get()`: Get authenticated user (`GET /v1/auth/user` | `auth__user__get`)

- Subgroup `auth.user.email.*`:
  - [ ] `auth.user.email.update()`: Request to update authenticated user email (`PATCH /v1/auth/user/email` | `auth__user__email__update`)

- Subgroup `auth.user.email.verification.*`:
  - [ ] `auth.user.email.verification.confirm()`: Confirm email verification with code (`POST /v1/auth/user/email/verification/confirm` | `auth__user__email__verification__confirm`)
  - [ ] `auth.user.email.verification.request()`: Request an email verification code (`POST /v1/auth/user/email/verification/request` | `auth__user__email__verification__request`)

- Subgroup `auth.user.passkeys.*`:
  - [ ] `auth.user.passkeys.delete()`: Revoke a registered passkey (`DELETE /v1/auth/user/passkeys/{passkey_id}` | `auth__user__passkeys__delete`)
  - [ ] `auth.user.passkeys.list()`: List registered passkeys for current user (`GET /v1/auth/user/passkeys` | `auth__user__passkeys__list`)

- Subgroup `auth.user.password.*`:
  - [ ] `auth.user.password.update()`: Change or set account password (`PATCH /v1/auth/user/password` | `auth__user__password__update`)

- Subgroup `auth.user.phone.*`:
  - [ ] `auth.user.phone.update()`: Request to update authenticated user phone number (`PATCH /v1/auth/user/phone` | `auth__user__phone__update`)

- Subgroup `auth.user.phone.verification.*`:
  - [ ] `auth.user.phone.verification.confirm()`: Confirm phone verification with code (`POST /v1/auth/user/phone/verification/confirm` | `auth__user__phone__verification__confirm`)
  - [ ] `auth.user.phone.verification.request()`: Request a phone verification code (`POST /v1/auth/user/phone/verification/request` | `auth__user__phone__verification__request`)

- Subgroup `auth.user.properties.*`:
  - [ ] `auth.user.properties.update()`: Update user personal properties (`PATCH /v1/auth/user/properties` | `auth__user__properties__update`)

#### `core` (Core Service — via `useCore(client)`)

- Root methods (`core.*`):
  - [ ] `core.manifest()`: Get dynamic cluster manifest and enabled services (`GET /v1/manifest` | `core__manifest`)

#### `data` (Data Service — via `useData(client)`)

- Root methods (`data.*`):
  - [ ] `data.graphql()`: Execute GraphQL query or mutation (`POST /v1/graphql` | `data__graphql`)

- Subgroup `data.kv.*`:
  - [ ] `data.kv.delete()`: Delete an ephemeral KV entry (`DELETE /v1/data/kv/{key}` | `data__kv__delete`)
  - [ ] `data.kv.get()`: Get an ephemeral KV entry by key (`GET /v1/data/kv/{key}` | `data__kv__get`)
  - [ ] `data.kv.increment()`: Atomically increment or decrement a KV key (`POST /v1/data/kv/increment` | `data__kv__increment`)
  - [ ] `data.kv.mget()`: Multi-get KV entries by list of keys (`POST /v1/data/kv/mget` | `data__kv__mget`)
  - [ ] `data.kv.mset()`: Batch set multiple KV entries (`POST /v1/data/kv/mset` | `data__kv__mset`)
  - [ ] `data.kv.put()`: Put raw ephemeral KV entry (`PUT /v1/data/kv/{key}` | `data__kv__put`)
  - [ ] `data.kv.set()`: Set an ephemeral KV entry with envelope (`POST /v1/data/kv/{key}` | `data__kv__set`)
  - [ ] `data.kv.touch()`: Refresh TTL of an ephemeral KV entry (`PATCH /v1/data/kv/{key}` | `data__kv__touch`)

- Subgroup `data.records.*`:
  - [ ] `data.records.create()`: Insert single or bulk rows with RETURNING * (`POST /v1/data/{schema_name}/{table_name}` | `data__records__create`)
  - [ ] `data.records.delete()`: Delete a row by primary key (`DELETE /v1/data/{schema_name}/{table_name}/{record_id}` | `data__records__delete`)
  - [ ] `data.records.get()`: Get a single row by primary key (`GET /v1/data/{schema_name}/{table_name}/{record_id}` | `data__records__get`)
  - [ ] `data.records.list()`: Query multiple rows with pagination, filtering, and sorting (`GET /v1/data/{schema_name}/{table_name}` | `data__records__list`)
  - [ ] `data.records.put()`: Update a row by primary key (`PUT /v1/data/{schema_name}/{table_name}/{record_id}` | `data__records__put`)
  - [ ] `data.records.update()`: Partially update a row by primary key (`PATCH /v1/data/{schema_name}/{table_name}/{record_id}` | `data__records__update`)

- Subgroup `data.rpc.*`:
  - [ ] `data.rpc.mutation()`: Execute database stored function or procedure (`POST /v1/data/{schema_name}/rpc/{function_name}` | `data__rpc__mutation`)
  - [ ] `data.rpc.query()`: Execute database stored function or procedure (read-only) (`GET /v1/data/{schema_name}/rpc/{function_name}` | `data__rpc__query`)

#### `fileStorage` (File Storage Service — via `useFileStorage(client)`)

- Subgroup `fileStorage.objects.*`:
  - [ ] `fileStorage.objects.delete()`: Delete a file object (`DELETE /v1/file-storage/objects/{bucket}/{key...}` | `file_storage__objects__delete`)
  - [ ] `fileStorage.objects.download()`: Download a file object (`GET /v1/file-storage/objects/{bucket}/{key...}` | `file_storage__objects__download`)
  - [ ] `fileStorage.objects.head()`: Inspect file object metadata headers (`HEAD /v1/file-storage/objects/{bucket}/{key...}` | `file_storage__objects__head`)
  - [ ] `fileStorage.objects.upload_post()`: Upload a file object via POST (`POST /v1/file-storage/objects/{bucket}/{key...}` | `file_storage__objects__upload_post`)
  - [ ] `fileStorage.objects.upload_put()`: Upload a file object via PUT (`PUT /v1/file-storage/objects/{bucket}/{key...}` | `file_storage__objects__upload_put`)

- Subgroup `fileStorage.url.*`:
  - [ ] `fileStorage.url.presign()`: Mint a presigned capability URL (`POST /v1/file-storage/url/presign` | `file_storage__url__presign`)

#### `image` (Image Service — via `useImage(client)`)

- Subgroup `image.info.*`:
  - [ ] `image.info.get()`: Inspect image metadata by path (`GET /v1/image/info/{signature}/{path...}` | `image__info__get`)
  - [ ] `image.info.post()`: Inspect image metadata from binary upload (`POST /v1/image/info` | `image__info__post`)

- Subgroup `image.transform.*`:
  - [ ] `image.transform.get()`: Transform and optimize an image (`GET /v1/image/{signature}/{path...}` | `image__transform__get`)

#### `function` (Function Service — via `useFunction(client)`)

The Function service has no base plane endpoints. All function operations exist on the control plane via `function.cp.*` upon service account elevation (`withServiceAccount(useFunction(client), ...)`).

#### `tasks` (Tasks Service — via `useTasks(client)`)

The Tasks service has no base plane endpoints. All tasks operations exist on the control plane via `tasks.cp.*` upon service account elevation (`withServiceAccount(useTasks(client), ...)`).

#### `console` (Console Service — via `useConsole(client)`)

The Console service has no base plane endpoints. All console operations exist on the control plane via `console.cp.*` upon service account elevation (`withServiceAccount(useConsole(client), ...)`).
<!-- END_GENERATED_BASE_METHODS -->

---

### Phase 3: Tier 2 Fluent Builders (Data, File Storage, Image)

Fluent builder APIs are provided for Data, File Storage, and Image. Other services are invoked directly via generated methods.

#### 1. Data Service (`useData` / `UseData`) — Query Builder & CDC

- [ ] Query builder (`from(table)`):
  - Projections: `.select(columns)`.
  - Filter Predicates: `.eq(col, val)`, `.neq(col, val)`, `.gt(col, val)`, `.gte(col, val)`, `.lt(col, val)`, `.lte(col, val)`, `.like(col, pattern)`, `.ilike(col, pattern)`, `.in(col, values)`, `.is(col, val)`.
  - Ordering & Pagination: `.order(col, { ascending, nullsFirst })`, `.limit(count)`, `.offset(count)`.
  - Execution Modifiers: `.single()` (fails unless exactly 1 record returned), `.maybeSingle()` (returns null if 0 records).
- [ ] CRUD operations (`from(table)`):
  - `.insert(records, options)`: Single or batch insertion.
  - `.update(values)`: Constrained mutation applied to records matching filter chain.
  - `.delete()`: Removes records satisfying filter chain.
  - `.upsert(records, options)`: Conflict-aware upsert against unique keys.
- [ ] Realtime CDC subscriptions (`subscribe`):
  - Connects to `/v1/realtime` WebSocket with auto-reconnection and heartbeat ping/pong.
  - Per-table subscription dispatchers (`onInsert`, `onUpdate`, `onDelete`).

#### 2. File Storage Service (`useFileStorage` / `UseFileStorage`) — Bucket Client

- [ ] Bucket client (`from(bucket)`):
  - `.download(key)`: Streams binary object bytes.
  - `.head(key)`: Returns object metadata headers.
  - `.upload(key, file, options)`: Uploads binary payload with automatic MIME detection.
  - `.delete(key)`: Removes file object.
  - `.createSignedUrl(key, expiresInSeconds)`: Mints time-limited pre-signed download URL.

#### 3. Image Service (`useImage` / `UseImage`) — Image URL Builder

- [ ] URL transformation builder (`url(source)`):
  - Geometry: `.resize({ width, height, fit, position })`.
  - Format & Compression: `.format(format)` (webp, avif, png, jpeg), `.quality(1-100)`.
  - Filters: `.blur(sigma)`, `.rotate(degrees)`, `.flip()`, `.flop()`.
  - Cryptographic Signing: `.sign(hmacKey)` (appends canonical HMAC-SHA256 signature to URL query params).

---

### Phase 4: Tier 3 Central Client & Modular Service Hooks

- [ ] Central client factory (`createClient` / `layrclient.New`):
  - Accepts `endpointURL` / `endpointUrl` and `options: ClientOptions`.
  - Holds `config`, `endpointURL` / `endpointUrl`, `options`, and cached singleton services for data realtime service.
  - Provides `setSession(token)` and `clearSession()` with in-place mutation of active HTTP headers.
- [ ] Modular functional hooks:
  - `useData(client)`, `useAuth(client)`, `useFileStorage(client)`, `useImage(client)`, `useFunction(client)`, `useTasks(client)`, `useCore(client)`.

---

### Phase 5: Tier 4 Service Account Elevation & Control Plane (`.cp`)

- [ ] Elevation helper (`withServiceAccount` / `WithServiceAccount`):
  - Accepts a service instance and `options: { serviceAccountKey }`.
  - Injects `Service-Account-Key` header and attaches the `.cp` (Go: `.CP`) namespace.
- [ ] Control plane operation mapping:
  - Every control plane operation across all services is directly callable on an elevated service client instance (returned from `withServiceAccount(use<Service>(client), ...)`) based on its OpenAPI Operation ID under `.cp` (`<service>.cp.<subgroups>.<method>()`).

<!-- BEGIN_GENERATED_CONTROL_PLANE_METHODS -->

#### `auth.cp` (Auth Control Plane — via `withServiceAccount(useAuth(client), ...)`)

- Subgroup `auth.cp.config.*`:
  - [ ] `auth.cp.config.get()`: Retrieve dynamic runtime auth configuration (`GET /v1/_/auth/config` | `auth__config__get`)
  - [ ] `auth.cp.config.update()`: Update dynamic runtime auth configuration with write-only secrets (`PUT /v1/_/auth/config` | `auth__config__update`)

- Subgroup `auth.cp.users.*`:
  - [ ] `auth.cp.users.create()`: Create a new application user account (`POST /v1/_/auth/users` | `auth__users__create`)
  - [ ] `auth.cp.users.delete()`: Delete application user and cascade related sessions/identities (`DELETE /v1/_/auth/users/{user_id}` | `auth__users__delete`)
  - [ ] `auth.cp.users.get()`: Get detailed user record by UUID (`GET /v1/_/auth/users/{user_id}` | `auth__users__get`)
  - [ ] `auth.cp.users.list()`: List registered application users with filtering and pagination (`GET /v1/_/auth/users` | `auth__users__list`)
  - [ ] `auth.cp.users.lock()`: Lock application user and revoke active sessions (`POST /v1/_/auth/users/{user_id}/lock` | `auth__users__lock`)
  - [ ] `auth.cp.users.unlock()`: Unlock application user and restore access (`DELETE /v1/_/auth/users/{user_id}/lock` | `auth__users__unlock`)

- Subgroup `auth.cp.users.sessions.*`:
  - [ ] `auth.cp.users.sessions.list()`: List active sessions for an application user (`GET /v1/_/auth/users/{user_id}/sessions` | `auth__users__sessions__list`)
  - [ ] `auth.cp.users.sessions.revoke()`: Revoke all active sessions for an application user (`POST /v1/_/auth/users/{user_id}/sessions/revoke` | `auth__users__sessions__revoke`)

#### `console.cp` (Console Control Plane — via `withServiceAccount(useConsole(client), ...)`)

- Subgroup `console.cp.config.*`:
  - [ ] `console.cp.config.get()`: Get runtime configuration (`GET /v1/_/console/config` | `console__config__get`)
  - [ ] `console.cp.config.update()`: Update runtime configuration (`PUT /v1/_/console/config` | `console__config__update`)

- Subgroup `console.cp.users.*`:
  - [ ] `console.cp.users.create()`: Create console operator account (`POST /v1/_/console/users` | `console__users__create`)
  - [ ] `console.cp.users.delete()`: Delete console operator account (`DELETE /v1/_/console/users/{user_id}` | `console__users__delete`)
  - [ ] `console.cp.users.get()`: Get console operator account details (`GET /v1/_/console/users/{user_id}` | `console__users__get`)
  - [ ] `console.cp.users.list()`: List console operator accounts (`GET /v1/_/console/users` | `console__users__list`)
  - [ ] `console.cp.users.password()`: Reset console operator password (`POST /v1/_/console/users/{user_id}/password` | `console__users__password`)
  - [ ] `console.cp.users.update()`: Update console operator account (`PUT /v1/_/console/users/{user_id}` | `console__users__update`)

- Subgroup `console.cp.users.sessions.*`:
  - [ ] `console.cp.users.sessions.delete()`: Revoke specific operator session (`DELETE /v1/_/console/users/{user_id}/sessions/{session_id}` | `console__users__sessions__delete`)
  - [ ] `console.cp.users.sessions.list()`: List active operator sessions (`GET /v1/_/console/users/{user_id}/sessions` | `console__users__sessions__list`)
  - [ ] `console.cp.users.sessions.revoke()`: Revoke all operator sessions (`POST /v1/_/console/users/{user_id}/sessions/revoke` | `console__users__sessions__revoke`)

#### `core.cp` (Core Control Plane — via `withServiceAccount(useCore(client), ...)`)

- Subgroup `core.cp.event_hooks.*`:
  - [ ] `core.cp.event_hooks.create()`: Create a new event hook (`POST /v1/_/core/event-hooks` | `core__event_hooks__create`)
  - [ ] `core.cp.event_hooks.delete()`: Delete event hook (`DELETE /v1/_/core/event-hooks/{event_hook_id}` | `core__event_hooks__delete`)
  - [ ] `core.cp.event_hooks.get()`: Get event hook by ID (`GET /v1/_/core/event-hooks/{event_hook_id}` | `core__event_hooks__get`)
  - [ ] `core.cp.event_hooks.list()`: List all event hooks (`GET /v1/_/core/event-hooks` | `core__event_hooks__list`)
  - [ ] `core.cp.event_hooks.update()`: Update event hook (`PUT /v1/_/core/event-hooks/{event_hook_id}` | `core__event_hooks__update`)

- Subgroup `core.cp.event_hooks.deliveries.*`:
  - [ ] `core.cp.event_hooks.deliveries.list()`: List event hook deliveries (`GET /v1/_/core/event-hooks/{event_hook_id}/deliveries` | `core__event_hooks__deliveries__list`)
  - [ ] `core.cp.event_hooks.deliveries.retry()`: Retry event hook delivery (`POST /v1/_/core/event-hooks/{event_hook_id}/deliveries/{delivery_id}/retry` | `core__event_hooks__deliveries__retry`)

- Subgroup `core.cp.events.*`:
  - [ ] `core.cp.events.get()`: Get event by ID (`GET /v1/_/core/events/{event_id}` | `core__events__get`)
  - [ ] `core.cp.events.list()`: List events (`GET /v1/_/core/events` | `core__events__list`)

- Subgroup `core.cp.service_accounts.*`:
  - [ ] `core.cp.service_accounts.create()`: Create a new machine service account (`POST /v1/_/core/service-accounts` | `core__service_accounts__create`)
  - [ ] `core.cp.service_accounts.delete()`: Delete service account (`DELETE /v1/_/core/service-accounts/{service_account_id}` | `core__service_accounts__delete`)
  - [ ] `core.cp.service_accounts.get()`: Get service account by ID (`GET /v1/_/core/service-accounts/{service_account_id}` | `core__service_accounts__get`)
  - [ ] `core.cp.service_accounts.list()`: List all service accounts (`GET /v1/_/core/service-accounts` | `core__service_accounts__list`)
  - [ ] `core.cp.service_accounts.update()`: Update service account (`PUT /v1/_/core/service-accounts/{service_account_id}` | `core__service_accounts__update`)

#### `data.cp` (Data Control Plane — via `withServiceAccount(useData(client), ...)`)

- Subgroup `data.cp.cache.*`:
  - [ ] `data.cp.cache.flush()`: Flush all global query and schema reflection caches (`POST /v1/_/data/cache/flush` | `data__cache__flush`)
  - [ ] `data.cp.cache.invalidate()`: Programmatically invalidate specific table or catalog caches (`POST /v1/_/data/cache/invalidate` | `data__cache__invalidate`)

- Subgroup `data.cp.config.*`:
  - [ ] `data.cp.config.get()`: Retrieve dynamic data service configuration (`GET /v1/_/data/config` | `data__config__get`)
  - [ ] `data.cp.config.update()`: Update dynamic data service configuration (`PUT /v1/_/data/config` | `data__config__update`)

- Subgroup `data.cp.tables.*`:
  - [ ] `data.cp.tables.create()`: Create a new PostgreSQL table with UUIDv7 primary key (`POST /v1/_/data/tables` | `data__tables__create`)
  - [ ] `data.cp.tables.delete()`: Drop a table (`DELETE /v1/_/data/tables/{schema_name}/{table_name}` | `data__tables__delete`)
  - [ ] `data.cp.tables.get()`: Inspect table schema, columns, indexes, and constraints (`GET /v1/_/data/tables/{schema_name}/{table_name}` | `data__tables__get`)
  - [ ] `data.cp.tables.list()`: List all user tables across configured schemas (`GET /v1/_/data/tables` | `data__tables__list`)

- Subgroup `data.cp.tables.columns.*`:
  - [ ] `data.cp.tables.columns.create()`: Add a new column to a table (`POST /v1/_/data/tables/{schema_name}/{table_name}/columns` | `data__tables__columns__create`)
  - [ ] `data.cp.tables.columns.delete()`: Drop a column from a table (`DELETE /v1/_/data/tables/{schema_name}/{table_name}/columns/{column_name}` | `data__tables__columns__delete`)
  - [ ] `data.cp.tables.columns.update()`: Alter column type, default, nullable, or rename (`PATCH /v1/_/data/tables/{schema_name}/{table_name}/columns/{column_name}` | `data__tables__columns__update`)

- Subgroup `data.cp.tables.indexes.*`:
  - [ ] `data.cp.tables.indexes.create()`: Create an index on a table (`POST /v1/_/data/tables/{schema_name}/{table_name}/indexes` | `data__tables__indexes__create`)
  - [ ] `data.cp.tables.indexes.delete()`: Drop an index from a table (`DELETE /v1/_/data/tables/{schema_name}/{table_name}/indexes/{index_name}` | `data__tables__indexes__delete`)
  - [ ] `data.cp.tables.indexes.list()`: List indexes on a table (`GET /v1/_/data/tables/{schema_name}/{table_name}/indexes` | `data__tables__indexes__list`)

- Subgroup `data.cp.tables.policies.*`:
  - [ ] `data.cp.tables.policies.create()`: Create a Row-Level Security policy on a table (`POST /v1/_/data/tables/{schema_name}/{table_name}/policies` | `data__tables__policies__create`)
  - [ ] `data.cp.tables.policies.delete()`: Drop an RLS policy from a table (`DELETE /v1/_/data/tables/{schema_name}/{table_name}/policies/{policy_name}` | `data__tables__policies__delete`)
  - [ ] `data.cp.tables.policies.list()`: List RLS policies on a table (`GET /v1/_/data/tables/{schema_name}/{table_name}/policies` | `data__tables__policies__list`)

- Subgroup `data.cp.tables.rls.*`:
  - [ ] `data.cp.tables.rls.update()`: Toggle Row-Level Security (ENABLE / DISABLE / FORCE) (`PATCH /v1/_/data/tables/{schema_name}/{table_name}/rls` | `data__tables__rls__update`)

#### `fileStorage.cp` (File Storage Control Plane — via `withServiceAccount(useFileStorage(client), ...)`)

- Subgroup `fileStorage.cp.buckets.*`:
  - [ ] `fileStorage.cp.buckets.create()`: Create a new storage bucket (`POST /v1/_/file-storage/buckets` | `file_storage__buckets__create`)
  - [ ] `fileStorage.cp.buckets.delete()`: Delete a storage bucket (`DELETE /v1/_/file-storage/buckets/{bucket}` | `file_storage__buckets__delete`)
  - [ ] `fileStorage.cp.buckets.get()`: Get bucket details by name (`GET /v1/_/file-storage/buckets/{bucket}` | `file_storage__buckets__get`)
  - [ ] `fileStorage.cp.buckets.list()`: List all storage buckets (`GET /v1/_/file-storage/buckets` | `file_storage__buckets__list`)
  - [ ] `fileStorage.cp.buckets.list_objects()`: List objects in a bucket (`GET /v1/_/file-storage/buckets/{bucket}/objects` | `file_storage__buckets__list_objects`)
  - [ ] `fileStorage.cp.buckets.update()`: Update bucket settings (`PATCH /v1/_/file-storage/buckets/{bucket}` | `file_storage__buckets__update`)

- Subgroup `fileStorage.cp.config.*`:
  - [ ] `fileStorage.cp.config.get()`: Get runtime file storage configuration (`GET /v1/_/file-storage/config` | `file_storage__config__get`)
  - [ ] `fileStorage.cp.config.update()`: Update runtime file storage configuration (`PUT /v1/_/file-storage/config` | `file_storage__config__update`)

#### `function.cp` (Function Control Plane — via `withServiceAccount(useFunction(client), ...)`)

- Subgroup `function.cp.config.*`:
  - [ ] `function.cp.config.get()`: Get runtime configuration (`GET /v1/_/function/config` | `function__config__get`)
  - [ ] `function.cp.config.update()`: Update runtime configuration (`PUT /v1/_/function/config` | `function__config__update`)

- Subgroup `function.cp.deployments.*`:
  - [ ] `function.cp.deployments.create()`: Deploy endpoint (`POST /v1/_/function/endpoints/{endpoint_id}/deploy` | `function__deployments__create`)
  - [ ] `function.cp.deployments.list()`: List deployments (`GET /v1/_/function/endpoints/{endpoint_id}/deployments` | `function__deployments__list`)
  - [ ] `function.cp.deployments.rollback()`: Rollback deployment (`POST /v1/_/function/endpoints/{endpoint_id}/rollback` | `function__deployments__rollback`)

- Subgroup `function.cp.domains.*`:
  - [ ] `function.cp.domains.create()`: Create custom domain (`POST /v1/_/function/domains` | `function__domains__create`)
  - [ ] `function.cp.domains.delete()`: Delete custom domain (`DELETE /v1/_/function/domains/{domain_id}` | `function__domains__delete`)
  - [ ] `function.cp.domains.list()`: List custom domains (`GET /v1/_/function/domains` | `function__domains__list`)

- Subgroup `function.cp.endpoint_domains.*`:
  - [ ] `function.cp.endpoint_domains.create()`: Create custom domain route (`POST /v1/_/function/endpoints/{endpoint_id}/domains` | `function__endpoint_domains__create`)
  - [ ] `function.cp.endpoint_domains.delete()`: Delete custom domain route (`DELETE /v1/_/function/endpoints/{endpoint_id}/domains/{route_id}` | `function__endpoint_domains__delete`)
  - [ ] `function.cp.endpoint_domains.list()`: List custom domain routes (`GET /v1/_/function/endpoints/{endpoint_id}/domains` | `function__endpoint_domains__list`)

- Subgroup `function.cp.endpoints.*`:
  - [ ] `function.cp.endpoints.create()`: Create endpoint (`POST /v1/_/function/endpoints` | `function__endpoints__create`)
  - [ ] `function.cp.endpoints.delete()`: Delete endpoint (`DELETE /v1/_/function/endpoints/{endpoint_id}` | `function__endpoints__delete`)
  - [ ] `function.cp.endpoints.executions()`: List endpoint execution logs (`GET /v1/_/function/endpoints/{endpoint_id}/executions` | `function__endpoints__executions`)
  - [ ] `function.cp.endpoints.get()`: Get endpoint (`GET /v1/_/function/endpoints/{endpoint_id}` | `function__endpoints__get`)
  - [ ] `function.cp.endpoints.list()`: List endpoints (`GET /v1/_/function/endpoints` | `function__endpoints__list`)
  - [ ] `function.cp.endpoints.update()`: Update endpoint (`PUT /v1/_/function/endpoints/{endpoint_id}` | `function__endpoints__update`)

- Subgroup `function.cp.executions.*`:
  - [ ] `function.cp.executions.list()`: List execution history logs (`GET /v1/_/function/executions` | `function__executions__list`)

- Subgroup `function.cp.stats.*`:
  - [ ] `function.cp.stats.get()`: Get runtime statistics (`GET /v1/_/function/stats` | `function__stats__get`)

#### `image.cp` (Image Control Plane — via `withServiceAccount(useImage(client), ...)`)

- Subgroup `image.cp.config.*`:
  - [ ] `image.cp.config.get()`: Get runtime image configuration (`GET /v1/_/image/config` | `image__config__get`)
  - [ ] `image.cp.config.update()`: Update runtime image configuration (`PUT /v1/_/image/config` | `image__config__update`)

- Subgroup `image.cp.presets.*`:
  - [ ] `image.cp.presets.create()`: Create an image preset (`POST /v1/_/image/presets` | `image__presets__create`)
  - [ ] `image.cp.presets.delete()`: Delete an image preset (`DELETE /v1/_/image/presets/{preset_id}` | `image__presets__delete`)
  - [ ] `image.cp.presets.get()`: Get image preset by ID (`GET /v1/_/image/presets/{preset_id}` | `image__presets__get`)
  - [ ] `image.cp.presets.list()`: List image presets (`GET /v1/_/image/presets` | `image__presets__list`)
  - [ ] `image.cp.presets.update()`: Update an image preset (`PUT /v1/_/image/presets/{preset_id}` | `image__presets__update`)

- Subgroup `image.cp.stats.*`:
  - [ ] `image.cp.stats.get()`: Get image service telemetry statistics (`GET /v1/_/image/stats` | `image__stats__get`)

- Subgroup `image.cp.url.*`:
  - [ ] `image.cp.url.sign()`: Sign an image transformation path (`POST /v1/_/image/url/sign` | `image__url__sign`)

#### `tasks.cp` (Tasks Control Plane — via `withServiceAccount(useTasks(client), ...)`)

- Subgroup `tasks.cp.config.*`:
  - [ ] `tasks.cp.config.get()`: Get runtime tasks configuration (`GET /v1/_/tasks/config` | `tasks__config__get`)
  - [ ] `tasks.cp.config.update()`: Update runtime tasks configuration (`PUT /v1/_/tasks/config` | `tasks__config__update`)

- Subgroup `tasks.cp.dlq.*`:
  - [ ] `tasks.cp.dlq.list()`: List failed tasks in DLQ (`GET /v1/_/tasks/dlq` | `tasks__dlq__list`)
  - [ ] `tasks.cp.dlq.purge()`: Purge a task from DLQ (`DELETE /v1/_/tasks/dlq/{execution_id}` | `tasks__dlq__purge`)
  - [ ] `tasks.cp.dlq.retry()`: Re-queue a failed task from DLQ (`POST /v1/_/tasks/dlq/{execution_id}/retry` | `tasks__dlq__retry`)

- Subgroup `tasks.cp.executions.*`:
  - [ ] `tasks.cp.executions.create()`: Trigger an immediate execution (`POST /v1/_/tasks/executions` | `tasks__executions__create`)
  - [ ] `tasks.cp.executions.list()`: Query execution audit logs (`GET /v1/_/tasks/executions` | `tasks__executions__list`)

- Subgroup `tasks.cp.jobs.*`:
  - [ ] `tasks.cp.jobs.create()`: Register a new recurring cron job (`POST /v1/_/tasks/jobs` | `tasks__jobs__create`)
  - [ ] `tasks.cp.jobs.delete()`: Delete a cron job (`DELETE /v1/_/tasks/jobs/{job_id}` | `tasks__jobs__delete`)
  - [ ] `tasks.cp.jobs.get()`: Get cron job details by ID (`GET /v1/_/tasks/jobs/{job_id}` | `tasks__jobs__get`)
  - [ ] `tasks.cp.jobs.list()`: List all recurring cron jobs (`GET /v1/_/tasks/jobs` | `tasks__jobs__list`)
  - [ ] `tasks.cp.jobs.update()`: Update an existing cron job (`PATCH /v1/_/tasks/jobs/{job_id}` | `tasks__jobs__update`)

- Subgroup `tasks.cp.stats.*`:
  - [ ] `tasks.cp.stats.get()`: Get operational telemetry statistics (`GET /v1/_/tasks/stats` | `tasks__stats__get`)

<!-- END_GENERATED_CONTROL_PLANE_METHODS -->

---

### Phase 6: Test Coverage & Verification

- [ ] Test coverage: all non-generated statements, branches, functions, and lines achieve 100% test coverage.
- [ ] Error and fallback coverage: all HTTP 4xx/5xx responses, network failures, missing optional parameters, and null returns have unit tests.
- [ ] Testing specification: unit, integration, and E2E standards documented in [client-sdk/TESTS.md (TBD)](file:///Users/hrdtr/Workspace/Personal/layr/client-sdk/TESTS.md).

---

## 5. Multi-Language Implementation Status Matrix

| Language | Directory | Core Transport & Retry | Fluent QueryBuilder | File Storage BucketClient | Image HMAC Builder | Realtime WebSocket | Elevation `.cp` | 100% Test Coverage | Status |
| --- | --- | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: |
| JavaScript / TypeScript | `client-sdk/javascript` | - | - | - | - | - | - | - (0%) | Pending |
| Python | `client-sdk/python` | - | - | - | - | - | - | - (0%) | Pending |
| Go | `client-sdk/go` | - | - | - | - | - | - | - (0%) | Pending |
| Rust | `client-sdk/rust` | - | - | - | - | - | - | - (0%) | Pending |
| Swift | `client-sdk/swift` | - | - | - | - | - | - | - (0%) | Pending |

Legend: Complete (100% Coverage) | In Progress | Planned

---

## 6. Development & Code Generation Workflow

### Step 1: Route Annotation & Spec Export

Every REST route controller in Layr must declare explicit SDK annotations before export:

```go
fuego.Get(s, "/api/v1/data/{schema}/{table}", c.ListRecords,
    server.SDKGroupName("data", "records"),
    server.SDKMethodName("list"),
)
```

Export OpenAPI 3.1 specifications:

```bash
./scripts/sdk/generate-openapi-specs --output-dir=generated/openapi
```

### Step 2: Multi-Language Code Generation

Run the unified generator script to regenerate Layer 1 base SDKs:

```bash
# Generate all SDKs
./scripts/sdk/generate --all

# Or generate a single target:
./scripts/sdk/generate --js
./scripts/sdk/generate --python
./scripts/sdk/generate --go
./scripts/sdk/generate --rust
./scripts/sdk/generate --swift
```

### Step 3: Run SDK Test Suites & Verify Coverage

Ensure all unit and integration tests pass with 100% coverage:

```bash
# JavaScript
cd client-sdk/javascript \
  && pnpm typecheck \
  && pnpm test:coverage

# Python
cd client-sdk/python \
  && uv run --extra dev ruff check layr_client tests \
  && uv run --extra dev mypy layr_client && uv run --extra dev pytest --cov=layr_client --cov-report=term-missing

# Go
cd client-sdk/go \
  && go test -v -cover ./...

# Rust
cd client-sdk/rust \
  && cargo clippy --all-targets -- -D warnings \
  && cargo llvm-cov --package layr-client --ignore-filename-regex 'generated' --summary-only

# Swift
cd client-sdk/swift \
  && swift test --enable-code-coverage \
  && xcrun llvm-cov report \
    $(swift build --show-bin-path)/LayrClientPackageTests.xctest/Contents/MacOS/LayrClientPackageTests \
    -instr-profile=$(swift build --show-bin-path)/codecov/default.profdata \
    -ignore-filename-regex='generated|Tests'
```

### Step 4: Run Continuous Drift Verification

Layr enforces drift verification to ensure consistency between route controllers, exported specs, client SDKs, and documentation:

1. OpenAPI specification drift: verifies committed `generated/openapi/` matches fresh export from Go route controllers:

   ```bash
   ./scripts/sdk/generate-openapi-specs --output-dir=generated/openapi
   git diff --exit-code generated/openapi/
   ```

2. SDK documentation drift: verifies `client-sdk/README.md` methods match Go route controllers:

   ```bash
   ./scripts/sdk/generate-dev-references --check
   ```

3. SDK codegen drift: verifies generated code in `client-sdk/*/generated/` matches fresh codegen output:

   ```bash
   ./scripts/sdk/generate --all
   git diff --exit-code client-sdk/*/generated/
   ```

4. Complete drift check: runs all verification checks in sequence:

   ```bash
   ./scripts/sdk/check_drifts
   ```

---

## 7. Adding a New Language SDK

When adding support for a new language (such as Java, Kotlin, C#, Dart, or PHP):

1. Target directory: create `client-sdk/<language>/` with standard package layout and manifests.
2. Codegen target: add `--<language>` flag and openapi-generator configuration in `scripts/sdk/generate`.
3. Fluent builders: implement fluent builders for Data (`QueryBuilder`), File Storage (`BucketClient`), and Image (`ImageURLBuilder`).
4. Client and hooks: implement `create_client`, modular `use_<service>` hooks, and `with_service_account` elevation.
5. Error mapping and retry: include typed error mapping and exponential backoff retry.
6. Test coverage: write test suites covering statements, branches, and error paths.
7. Package README: provide quickstarts and service documentation matching this standard.
