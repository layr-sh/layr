# Contributing to Layr

Thank you for your interest in contributing to Layr!

Layr (`layr.sh`) is an open-source developer infrastructure platform and modular control plane for PostgreSQL 18+, engineered to eliminate the compromise between developer velocity and data sovereignty under the **Apache License 2.0**.

---

## 📜 Developer Certificate of Origin (DCO)

All contributors must sign off on the [Developer Certificate of Origin](https://developercertificate.org/) (DCO 1.1). This certifies that you wrote or have the right to submit the code you are contributing.

### How to Sign Off

Add a `Signed-off-by` trailer to every git commit:

```bash
git commit -s -m "feat(auth): add passkey challenge verification"
```

Or manually append the trailer to your commit message:

```text
Signed-off-by: Your Name <your.email@example.com>
```

> **Note:** Commits missing a DCO sign-off cannot be merged into `main`.

---

## 🛠️ Getting Started

### Prerequisites

- **Go**: 1.27+ installed ([golang.org](https://golang.org/dl/))
- **PostgreSQL**: 18+ (or use Layr's embedded native engine for local development)
- **Git**: For version control

### Setup Repository & Git Hooks

1. Clone your fork of the repository:

   ```bash
   git clone https://github.com/<your-username>/layr.git
   cd layr
   ```

2. Initialize and configure the version-controlled Git hooks:

   ```bash
   ./scripts/setup-githooks
   ```

   This configures pre-commit hooks that automatically verify code formatting (`./scripts/fmt --check`) and linting (`./scripts/lint -q`) before committing.

---

## 🚀 Development Scripts Policy

> **Important:** Always execute tests, linting, formatting, and generation tools using the scripts inside the `./scripts/` directory rather than running raw toolchain commands directly.

| Task | Script | Description |
| :--- | :--- | :--- |
| **Formatting** | `./scripts/fmt` | Formats all Go files with `gofumpt`. Run `./scripts/fmt --check` to check without modifying. |
| **Linting** | `./scripts/lint` | Runs `custom-gcl` (golangci-lint with custom linters including `namingclarity`). Supports `--fix` and `-v`. |
| **Testing** | `./scripts/test` | Runs tests with atomic statement coverage. |
| **SDK & Spec Gen** | `./scripts/sdk/generate` | Generates OpenAPI specs and client SDKs across all supported languages. |
| **SDK Drift Check** | `./scripts/sdk/check_drifts` | Validates that generated SDKs and OpenAPI specs match the current route implementations. |

---

## 🧪 Testing & Coverage Invariant

- Run full test suite with coverage enforcement:

  ```bash
  ./scripts/test
  ```

- Run tests for specific packages:

  ```bash
  ./scripts/test ./auth ./data ./core
  ```

- Filter tests while developing:

  ```bash
  ./scripts/test ./auth -run Password
  ```

- Generate visual HTML coverage report:

  ```bash
  ./scripts/test --html
  ```

Any PR that drops test coverage below 100.0% will fail the CI pipeline and pre-commit checks.

---

## 📐 Architecture & Coding Standards

### 1. Invariant Kernel & Modular Services

Layr consists of a minimal core kernel ([`core`](core)) and 7 independent modular services:

- [`auth`](auth)
- [`data`](data)
- [`filestorage`](filestorage)
- [`tasks`](tasks)
- [`image`](image)
- [`function`](function)
- [`console`](console)

Keep shared functionality in `core` and service-specific domain logic scoped within its respective module.

### 2. Error Handling & Security Invariant

Base APIs (`/v1/*`) serve end-users directly and **must never leak internal/technical error messages** in client-facing HTTP responses:

- Client-facing response details must be clean, standardized RFC messages (e.g., `"Invalid request payload"`, `"Access denied"`, `"Resource not found"`, `"Service temporarily unavailable"`).
- Internal Go errors (`err.Error()`), SQL syntax, or internal diagnostics must be forwarded exclusively to server debug logs as the trailing `debugLog` argument to `core.WriteErrorResponse`:

  ```go
  core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", err.Error())
  ```

### 3. Commit Messages & PR Titles

Commit messages and PR titles must follow [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` A new feature
- `fix:` A bug fix
- `refactor:` Code change that neither fixes a bug nor adds a feature
- `test:` Adding or updating tests
- `docs:` Documentation changes
- `chore:` Maintenance, build, or auxiliary tool updates

---

## 📥 Pull Request Workflow

1. Fork the repository and create a feature branch:

   ```bash
   git checkout -b feat/your-feature-name
   ```

2. Make your changes and adhere to the project standards.
3. Validate your code locally using repository scripts:

   ```bash
   ./scripts/fmt
   ./scripts/lint
   ./scripts/test
   ```

4. Commit your changes with Conventional Commits and DCO sign-off:

   ```bash
   git commit -s -m "feat(scope): concise description of change"
   ```

5. Push to your fork and submit a Pull Request to `main`.
6. Fill in all sections of the [Pull Request Template](.github/PULL_REQUEST_TEMPLATE.md).

---

## 💬 Community & Discussions

- **Questions & Ideas**: Join the conversation on [GitHub Discussions](https://github.com/layr-sh/layr/discussions).
- **Bug Reports**: Open an issue using the [Bug Report Template](.github/ISSUE_TEMPLATE/bug-report.yml).
- **Feature Requests**: Propose features using the [Feature Request Template](.github/ISSUE_TEMPLATE/feature-request.yml).
- **Documentation**: Explore guides and references at [layr.sh/docs](https://layr.sh/docs).
