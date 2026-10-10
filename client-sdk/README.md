# Layr Multi-Language SDK Ecosystem (`client-sdk/`)

> **System:** Layr (`layr.sh`)  
> **Status:** Active & Mandatory Standard  
> **Target SDKs:** JavaScript/TypeScript (`client-sdk/javascript`), Python (`client-sdk/python`), Go (`client-sdk/go`), Rust (`client-sdk/rust`), Swift (`client-sdk/swift`)

This directory houses the official client SDKs for **Layr**, the open-source, single-binary Backend-as-a-Service (BaaS) and application platform built on PostgreSQL 18+.

---

## 1. Multi-Language Packages & Repositories

| Language | Directory | Package Identifier | Package Manager | Standard Client Import |
| --- | --- | --- | --- | --- |
| **JavaScript / TypeScript** | `client-sdk/javascript/` | `layr-client` | npm / pnpm / yarn / bun | `import { createClient, useData } from 'layr-client'` |
| **Python** | `client-sdk/python/` | `layr-client` (`layr_client`) | pip / uv / poetry | `from layr_client import create_client, use_data` |
| **Go** | `client-sdk/go/` | `github.com/layr-sh/layr/client-sdk/go` | Go Modules | `import layrclient "github.com/layr-sh/layr/client-sdk/go"` |
| **Rust** | `client-sdk/rust/` | `layr-client` (`layr_client`) | Cargo / crates.io | `use layr_client::{create_client, use_data};` |
| **Swift** | `client-sdk/swift/` | `LayrClient` | Swift Package Manager | `import LayrClient` |

---

## 2. Universal 4-Tier SDK Architectural Pattern

Every official Layr SDK, regardless of language or runtime, **must strictly implement** the 4-tier architectural pattern:

```text
┌──────────────────────────────────────────────────────────────────────────────┐
│  Tier 4: Service Account Elevation & Protected Control Plane (.cp)           │
│  withServiceAccount(service, { serviceAccountKey }) -> ServiceWithCP         │
├──────────────────────────────────────────────────────────────────────────────┤
│  Tier 3: Monolith Context & Functional Service Hooks                         │
│  createClient(endpointURL, options) -> LayrClient                            │
│  useData(client), useAuth(client), useFileStorage(client), ...        │
├──────────────────────────────────────────────────────────────────────────────┤
│  Tier 2: Ergonomic Service Wrappers & Fluent Value-Add Builders              │
│  - QueryBuilder: .from('table').select().eq().order().limit()                │
│  - BucketClient: .from('bucket').upload().download().createSignedUrl()       │
│  - ImageURLBuilder: .url('source').resize().format().quality().sign()        │           │
├──────────────────────────────────────────────────────────────────────────────┤
│  Tier 1: Automated OpenAPI 3.1 Codegen (Layer 1 Base Classes)                │
│  Auto-generated in client-sdk/<lang>/generated/ (Strictly Immutable by Hand) │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Layer 1: Automated OpenAPI 3.1 Codegen (`generated/`)

- Exported directly from Go-Fuego route controllers via `scripts/export_openapi.go`.
- Generated deterministically into `client-sdk/<language>/generated/` using `scripts/sdk-generate`.
- **Inviolable Rule:** Hand-editing code inside `generated/` directories is strictly forbidden. All customization must exist in Layer 2 wrappers or upstream route definitions.

### Layer 2: Ergonomic Service Wrappers & Fluent Builders

- Wraps generated Layer 1 API classes with idiomatic language patterns.
- Implements fluent query builders (`data.from('todos').select('*').eq(...)`), high-level bucket operations (`fileStorage.from('avatars').upload(...)`), HMAC-SHA256 image transformation builders, and WebSocket CDC subscription managers.

### Layer 3: Central Client Factory & Modular Service Hooks

- Provides a unified factory function (`createClient` / `layrclient.New`) returning a lightweight client context.
- Provides modular functional hooks (`useData`, `useAuth`, `useFileStorage`, `useFunction`, `useImage`, `useTasks`, `useCore`) for tree-shaking and clean composition.

### Layer 4: Service Account Elevation & `.cp` Namespace

- Public service clients strictly expose public client APIs.
- Machine-to-machine control plane APIs (DDL, table alterations, user lockouts, cron job schedules, webhook configuration) are strictly segregated under `.cp` (Go: `.CP`) and accessible only after explicit elevation via `withServiceAccount`.

---

## 3. Mandatory SDK Invariants & System Laws

Every current and future SDK implementation **must strictly uphold** these system laws:

### 1. Zero "Admin" Terminology — Strictly `.cp` / `.CP`

- Administrative, DDL, and configuration endpoints must **never** be named `admin`, `master`, or `priv`.
- The canonical property across all languages is strictly `.cp` (Go: `.CP`), attached exclusively upon `withServiceAccount` elevation.

### 2. Problem Details Error Translation

- Every HTTP error (non-2xx response) must be parsed into a typed standard Layr's error response data.

### 3. Transport Resilience: Exponential Backoff, Jitter & Retry-After

- All SDK transports must support automatic retry on transient rate limits (`429 Too Many Requests`) and server unavailability (`503 Service Unavailable`).
- Retry logic must compute exponential backoff with full jitter (e.g., $\pm 20\%$) and strictly honor the `Retry-After` HTTP response header when present.

### 4. Bifurcated Security Headers

Every outgoing HTTP request must inject the correct security headers based on client mode:

| Header Name | Token Type | Target Security Domain |
| --- | --- | --- |
| `X-Layr-Publishable-Key` | Public API Key (`your-publishable-key`) | Browser / mobile client; enforces PostgreSQL Row-Level Security (RLS) |
| `Authorization: Bearer <token>` | User Session JWT | Authenticated user session; enforces PostgreSQL RLS |
| `X-Layr-Service-Account-Key` | Service Account Secret (`your-service-account-key`) | Machine-to-machine backend; bypasses RLS, allows `.cp` operations |

> [!IMPORTANT]
> **Zero Prefix Invariant:** Any key, both publishable key and service account key in Layr, must never contain a prefix (never `pk_`, `pk_dev_`, `pk_live_`, `sa_`, `sak_`, `layr_sec_`, or `layr_sa_`). Keys are always un-prefixed raw 32-byte hex strings.

### 5. Canonical Identifier & Naming Conventions

Follow standardized identifiers across all languages without truncated stems:

- `endpointURL` / `endpointUrl` (never `baseUrl`, or `url`)
- `accountID` / `accountId`, `serviceAccountKey`, `serviceAccountID` / `serviceAccountId` (never `accId`, `saKey`, `saId`)
- `webhookID` / `webhookId` (never `whID`)
- `timeoutMs`, `initialDelayMs`, `maxDelayMs` (never `timeout`, `delay`)
- Standard English naming: `is_enabled`, `properties`, `enabled_services`.

### 6. Strict 100.0% Test Coverage Mandate

- Every non-generated file, function, branch, and statement must achieve **100.0% test coverage**.
- Coverage must be verified in CI using language-native coverage tools:
  - JavaScript / TypeScript: `vitest run --coverage` (V8 engine)
  - Python: `pytest --cov=layr_client --cov-report=term-missing`
  - Go: `go test -cover -coverprofile=...`
  - Rust: `cargo llvm-cov`
  - Swift: `swift test --enable-code-coverage`

---

## 4. Universal SDK Implementation Checklist (Detailed Specification)

Using the **JavaScript/TypeScript SDK (`client-sdk/javascript`) as the canonical reference implementation**, every language SDK must implement and pass the following checklist:

### Phase 1: Core Foundation & Resilient Transport

- [ ] **Endpoint URL Normalization (`normalizeEndpointURL`):**
  - Trims trailing slashes (`/+$`) and defaults to `http://localhost:5297` if empty.
- [ ] **Bifurcated Header Injection (`buildHeaders`):**
  - Injects `X-Layr-Publishable-Key` for public client requests.
  - Injects `Authorization: Bearer <token>` for user sessions (defensively strips redundant leading `Bearer` prefix if passed).
  - Injects `X-Layr-Service-Account-Key` for elevated machine-to-machine calls.
  - Merges custom user headers without overwriting security headers unless explicitly configured.
- [ ] **Problem Details Error Handling (`LayrAPIError`):**
  - Parses standard Layr's error JSON responses.
  - Provides graceful fallbacks when response body is non-JSON or missing.
- [ ] **Resilient Transport & Retry Middleware (`createFetchWithRetry`):**
  - Intercepts transient HTTP `429 (Too Many Requests)` and `503 (Service Unavailable)`.
  - Parses `Retry-After` response header in both integer seconds and RFC 1123 HTTP-Date format.
  - Applies exponential backoff with full $\pm 20\%$ jitter when `Retry-After` is omitted (`initialDelayMs`, `maxDelayMs`, `maxRetries`).
  - Drains/discards response bodies prior to retry to prevent memory leaks and unconsumed stream locks.
  - Implements request timeout cancellation via `timeoutMs`.
- [ ] **Dynamic Session Token Mutability (`setSession` / `clearSession`):**
  - Updating session token on the central client mutates the active HTTP configuration in-place so all instantiated service clients immediately reflect the new session.

---

### Phase 2: Layer 2 Ergonomic Service Wrappers & Fluent Builders

#### 1. Data Service (`useData` / `UseData`)

...

####

---

### Phase 3: Layer 3 Central Client & Modular Service Hooks

- [ ] **Central Client Factory (`createClient` / `layrclient.New`):**
  - Accepts `endpointURL` / `endpointUrl` and `options: ClientOptions`.
  - Holds `config`, `endpointURL` / `endpointUrl`, `options`, and cached singleton services for data realtime service.
  - Provides `setSession(token)` and `clearSession()` with in-place mutation of active HTTP headers.
- [ ] **Modular Functional Hooks:**
  - `useData(client)`, `useAuth(client)`, `useFileStorage(client)`, `useImage(client)`, `useFunction(client)`, `useTasks(client)`, `useCore(client)`.

---

### Phase 4: Layer 4 Service Account Elevation & `.cp` Control Plane

- [ ] **Elevation Helper (`withServiceAccount` / `WithServiceAccount`):**
  - Accepts a service instance and `options: { serviceAccountKey }`.
  - Injects `X-Layr-Service-Account-Key` header and attaches the `.cp` (Go: `.CP`) namespace.
- [ ] **Protected Control Plane Services:**
  - **Data `.cp`:**
    ...
  - **Auth `.cp`:**
    ...
  - **File Storage `.cp`:**
    ...
  - **Image `.cp`:**
    ...
  - **Tasks `.cp`:**
    ...
  - **Function `.cp`:**
    ...
  - **Core `.cp`:**
    ...

---

### Phase 5: Test Coverage & Verification Mandate

- [ ] **Strict 100.0% Test Coverage:** All non-generated statements, branches, functions, and lines must achieve 100.0% test coverage.
- [ ] **Error & Fallback Coverage:** All HTTP 4xx/5xx responses, network failures, missing optional parameters, and null returns must have dedicated unit tests.
- [ ] **Canonical Testing Specification & Checklist:** Comprehensive Unit, Integration, and E2E testing standard codified in [client-sdk/TESTS.md (TBD)](file:///Users/hrdtr/Workspace/Personal/layr/client-sdk/TESTS.md).

---

## 5. Multi-Language Implementation Status Matrix

| Language | Directory | Core Transport & Retry | Fluent QueryBuilder | File Storage BucketClient | Image HMAC Builder | Realtime WebSocket | Elevation `.cp` | 100% Test Coverage | Status |
| --- | --- | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: |
| **JavaScript / TypeScript** | `client-sdk/javascript` | - | - | - | - | - | - | - (0%) | **Pending** |
| **Python** | `client-sdk/python` | - | - | - | - | - | - | - (0%) | **Pending** |
| **Go** | `client-sdk/go` | - | - | - | - | - | - | - (0%) | **Pending** |
| **Rust** | `client-sdk/rust` | - | - | - | - | - | - | - (0%) | **Pending** |
| **Swift** | `client-sdk/swift` | - | - | - | - | - | - | - (0%) | **Pending** |

Legend: ✅ Complete & Verified (100.0% Coverage) | 🔄 In Active Harmonization | ⏳ Planned

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
go run scripts/export_openapi.go --output-dir=generated/openapi
```

### Step 2: Multi-Language Code Generation

Run the unified generator script to regenerate Layer 1 base SDKs:

```bash
# Generate all SDKs
./scripts/sdk-generate --all

# Or generate a single target:
./scripts/sdk-generate --js
./scripts/sdk-generate --python
./scripts/sdk-generate --go
./scripts/sdk-generate --rust
./scripts/sdk-generate --swift
```

### Step 3: Run SDK Test Suites & Verify Coverage

Ensure all unit and integration tests pass with 100.0% coverage:

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

Layr enforces 3 independent drift boundaries to ensure zero divergence between route controllers, exported specs, client SDKs, and architectural documentation:

1. **OpenAPI Specification Drift Gate:** Asserts committed `generated/openapi/` matches fresh export from Go route controllers:

   ```bash
   go run scripts/export_openapi.go --output-dir=generated/openapi
   git diff --exit-code generated/openapi/
   ```

2. **SDK Codegen Drift Gate:** Asserts generated code in `client-sdk/*/generated/` matches fresh codegen output:

   ```bash
   ./scripts/sdk-generate --all
   git diff --exit-code client-sdk/*/generated/
   ```

3. **All-in-One Drift Verification Gate:** Executes all 2 verification gates in sequence:

   ```bash
   ./scripts/sdk-check-drifts
   ```

---

## 7. Rules for Adding a New Language SDK

When adding support for a new language (e.g. Java, Kotlin, C#, Dart, PHP):

1. **Add Target Directory:** Create `client-sdk/<language>/` with standard package layout and manifests.
2. **Add Codegen Target:** Add `--<language>` flag and openapi-generator configuration in `scripts/sdk-generate`.
3. **Implement Layer 2 Wrappers:** Implement fluent builders (`QueryBuilder`, `BucketClient`, `ImageURLBuilder`, ...).
4. **Implement Layer 3 & 4:** Implement `create_client`, modular `use_<service>` hooks, and `with_service_account` elevation.
5. **Implement Error Mapping & Retry:** Include typed error mapping and exponential backoff retry.
6. **Achieve 100.0% Test Coverage:** Write comprehensive test suites covering all statements, branches, and error paths.
7. **Create Package README:** Provide complete quickstarts and service documentation matching this standard.
