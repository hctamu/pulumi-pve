# Copilot Instructions

## General Guidelines

**🌐 Prefix every response with the 🌐 emoji when answering questions about general guidelines, conventions, or project principles.**

- **Be critical:** Question assumptions, challenge vague requirements, and flag inconsistencies. Don't just accept a task at face value — if the ask is unclear, the scope is ambiguous, or the approach seems wrong, say so.
- **Ask for clarification:** If a request is incomplete, contradicts the guidelines, or lacks context, ask back instead of guessing. Examples: "Is this a bug fix or a refactor?", "Which resource does this apply to?", "What's the success criterion?"
- **Single source of truth:** This file (copilot-instructions.md) is the authoritative reference. All other documentation should defer to it.

This is a **Pulumi provider for Proxmox VE (PVE)** written in Go. It enables infrastructure-as-code management of Proxmox resources (VMs, storage, pools, HA groups, users, groups, roles, ACLs) using the [pulumi-go-provider](https://github.com/pulumi/pulumi-go-provider) infer framework. Multi-language SDKs (Go, Python, Node.js, .NET, Java) are **generated** from a JSON schema — never edit files under `sdk/` directly.

Development must be done inside the devcontainer (VSCode → "Reopen in Container").

---

## Languages & Frameworks

- **Go 1.26** — provider source lives entirely under `provider/`
- **pulumi-go-provider** — infer-based Pulumi resource framework (schema auto-generated from Go struct tags)
- **luthermonson/go-proxmox** — Proxmox HTTP API client
- **testify** — test assertions (`assert`/`require`)
- **mocha/v3** — HTTP mock server for adapter tests
- **golangci-lint v2** — enforced via `.golangci.yml`
- **gofumpt** — stricter Go formatter (used by `make lint`)
- **golines** — enforces 120-char line limit (used by `make lint`)

---

## Build & Validate Commands

**Pre-commit checklist** (all must pass locally before pushing):
```bash
make lint              # golangci-lint with golines (120-char limit)
make test_provider     # all unit tests
make provider          # build provider binary
```

**Full commands:**
```bash
# Build provider binary
make provider

# Run provider unit tests (fast — no Proxmox needed)
make test_provider
# or single package:
cd provider && go test -v -count=1 -cover -timeout 2h -parallel 4 ./pkg/provider/resources/vm
# or single test:
cd provider && go test -v -count=1 -cover -timeout 2h ./pkg/provider/resources/vm -run TestParseCPU

# Lint (must pass before committing)
# IMPORTANT: use 'make lint' not 'golangci-lint' directly — it sets --path-prefix provider
make lint

# Tidy modules (run after adding/removing dependencies)
make tidy

# Regenerate schema.json (fast — after changing struct tags)
make generate_schema

# Regenerate schema + all SDKs (slow — only when exporting a new release)
make build

# Integration testing (requires live Proxmox or integration test env)
export PULUMI_CONFIG_PASSPHRASE=<passphrase>
make up       # first deploy
make update   # re-deploy after changes
make preview  # dry-run
make down     # destroy + cleanup
```

CI runs `make test_provider` and `make lint` on every push/PR. Both must pass.

---

## Project Layout

```
provider/
  cmd/pulumi-resource-pve/   # Entry point (main.go) + schema.json
  pkg/
    config/                  # Provider config struct (URL, user, token, SSH)
    proxmox/                 # Domain layer: interfaces + models (one file per resource)
    adapters/                # Adapter layer: HTTP/SSH implementations + unit tests
    provider/
      provider.go            # Resource registration & wiring
      resources/             # Pulumi resource implementations (vm, ha, pool…)
    testutils/               # Shared mock helpers (MockProxmoxClient, CreateMockServer)
sdk/                         # GENERATED — do not edit
examples/                    # YAML/Go Pulumi programs for manual testing
.golangci.yml                # Lint rules (repo root)
.github/skills/              # Reusable task-specific skills
  - `.github:fix-lint` — fix golangci-lint errors
  - `.github:new-resource` — scaffold a new resource
  - `.github:adapter-testing` — add/complete adapter unit tests
  - `.github:debugging` — find and fix bugs systematically
```

---

## Architecture — Three Layers

1. **Domain** (`provider/pkg/proxmox/`) — interfaces and domain models. No HTTP details. Each resource has its own file (e.g. `ha.go`, `vm.go`).
2. **Adapters** (`provider/pkg/adapters/`) — implement domain interfaces against the real Proxmox HTTP/SSH API. Each adapter has a matching `_test.go`.
3. **Resources** (`provider/pkg/provider/resources/`) — implement `infer.CustomResource` interfaces. Depend **only** on domain interfaces, never on adapters directly. Wired together in `provider.go`.

**Reference implementation:** The `ha` (HA group) resource is the reference for structure, patterns, and style. Check it first when uncertain.

For debugging guidance, see the `.github:debugging` skill.

### Do / Don't

**Critical (breaks build or contracts):**
| ✅ Do | ❌ Don't |
|---|---|
| Add `var _ proxmox.XOps = (*XAdapter)(nil)` compile-time checks | Import adapter packages from resource packages |
| Add `var _ = (infer.CustomResource[...])((*X)(nil))` in resource files | Edit anything under `sdk/` |
| **Nil-check the operations field before every use in resources** (fails silently otherwise) | Use `github.com/golang/protobuf` (use `google.golang.org/protobuf`) |
| Use `fmt.Errorf("...: %w", err)` for error wrapping | Share a mock server across parallel subtests |
| Call `t.Parallel()` in every test function and subtest | Use `NewAPIMock` for new tests (use `CreateMockServer` instead) |

**Style & discipline:**
| ✅ Do | ❌ Don't |
|---|---|
| Keep changes focused to the task at hand | Refactor unrelated code while fixing a bug |
| Use existing abstractions (interfaces, helpers) | Add abstractions for a single caller |
| Validate inputs at system boundaries (user input, external APIs) | Add error handling for impossible cases (trust internal contracts) |
| Ask for clarification when Proxmox API is undocumented or schema/adapter/resource disagree | Guess or assume API behavior |

---

## Code Style

### Copyright header — required on **every** `.go` file

```go
/* Copyright 2025, Pulumi Corporation.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
```

### Import ordering (gci — five groups, blank line between each)

```go
import (
    "context"              // 1. stdlib
    "fmt"

    _ "embed"              // 2. blank imports

    "github.com/stretchr/testify/require"  // 3. third-party

    p "github.com/pulumi/pulumi-go-provider"  // 4. pulumi packages
    "github.com/pulumi/pulumi-go-provider/infer"

    "github.com/hctamu/pulumi-pve/provider/pkg/proxmox"  // 5. local (this module)
)
```

- **Format**: `gofumpt` (stricter than `gofmt`) + `golines` (120-char line limit). Run `make lint` to auto-check both.
- **Line limit:** 120 characters (enforced by golines via `make lint`).
- **Commit message format:** Conventional Commits — `type(scope): description`. Examples: `fix(vm): handle CPU parsing edge case`, `feat(ha): add support for HA group rules`.
- **Method receivers**: Use meaningful names that reflect the type, not single letters or opaque abbreviations. Prefer a short but descriptive word derived from the type name (e.g., `adapter` for `*VMAdapter`, `inputs` for `*RoleInputs`, `mock` for `*mockXOperations`). Avoid `a`, `r`, `p`, `m`, `e`, `s`, and other one-letter receivers.

**Bad → Good example:**
```go
// BAD
func (a *VMAdapter) Create(ctx context.Context, r *CreateInput) (*VM, error) {
    // ...
}

// GOOD
func (adapter *VMAdapter) Create(ctx context.Context, inputs *CreateInput) (*VM, error) {
    // ...
}
```

---

## Testing Rules

- **All tests must be table-driven.** Define a `tests` slice of structs; range with `t.Run`.
- **Always call `t.Parallel()`** in every test function and every subtest.
- **Two test patterns — use the right one:**
  - **Adapter tests** (`*_adapter_test.go`): use `testutils.CreateMockServer` to start a real `httptest.Server`, then wire `NewProxmoxAdapter` → `.Connect()` → `NewXAdapter`. Assert on both the captured HTTP request and the returned domain value.
  - **Resource tests** (`resources/<name>/<name>_test.go`): define a local `mockXOperations` struct with `func` fields; inject it directly into the resource struct. No HTTP server needed.
- Lifecycle/integration tests live in `<name>_lifecycle_test.go` and use `integration.LifeCycleTest`.
- The `-short` flag skips integration tests; unit tests must not require a live Proxmox instance.

```go
func TestExample(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"case one", "a", "A"},
        {"case two", "b", "B"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            require.Equal(t, tt.expected, transform(tt.input))
        })
    }
}
```

---

## Adding a New Resource — Checklist

When adding a new resource, create **all** of the following (in order):

1. `provider/pkg/proxmox/<name>.go` — `<Name>Operations` interface + domain models + API struct
2. `provider/pkg/adapters/<name>_adapter.go` — adapter implementing the interface
3. `provider/pkg/adapters/<name>_adapter_test.go` — httptest-based adapter tests
4. `provider/pkg/provider/resources/<name>/<name>.go` — Pulumi resource (uses interface, not adapter)
5. `provider/pkg/provider/resources/<name>/<name>_test.go` — resource unit tests with mock interface
6. `provider/pkg/provider/provider.go` — add `new<Name>ResourceWithConfig` wiring + register in `Resources` slice

Use the `/new-resource` Copilot skill for the full step-by-step scaffold guide.

---

## CI Checks (must all pass)

| Check | Command |
|---|---|
| Unit tests | `make test_provider` |
| Lint | `make lint` (golangci-lint with `.golangci.yml`) |
| Build | `make provider` |
| Schema diff (PR only) | schema-tools compares generated schema |

