# Copilot Instructions

## Repository Summary

This is a **Pulumi provider for Proxmox VE (PVE)** written in Go. It enables infrastructure-as-code management of Proxmox resources (VMs, storage, pools, HA groups, users, groups, roles, ACLs) using the [pulumi-go-provider](https://github.com/pulumi/pulumi-go-provider) infer framework. Multi-language SDKs (Go, Python, Node.js, .NET, Java) are **generated** from a JSON schema — never edit files under `sdk/` directly.

Development must be done inside the devcontainer (VSCode → "Reopen in Container").

---

## Languages & Frameworks

- **Go 1.24** — provider source lives entirely under `provider/`
- **pulumi-go-provider** — infer-based Pulumi resource framework (schema auto-generated from Go struct tags)
- **luthermonson/go-proxmox** — Proxmox HTTP API client
- **testify** — test assertions (`assert`/`require`)
- **mocha/v3** — HTTP mock server for adapter tests
- **golangci-lint v2** — enforced via `.golangci.yml`

---

## Build & Validate Commands

```bash
# Build provider binary
make provider

# Run provider unit tests (fast — no Proxmox needed)
make test_provider
# or directly:
cd provider && go test -v -count=1 -cover -timeout 2h -parallel 4 ./...

# Lint (must pass before committing)
make lint

# Tidy modules (run after adding/removing dependencies)
make tidy

# Generate schema + all SDKs (slow, only needed when schema changes)
make build
```

CI runs `make test_provider` and `make lint` on every push/PR. Both must pass.

---

## Project Layout

```
provider/
  cmd/pulumi-resource-pve/   # Entry point (main.go) + schema.json
  pkg/
    config/                  # Provider config struct (URL, user, token, SSH)
    proxmox/                 # Domain layer: interfaces + models
    adapters/                # Adapter layer: HTTP implementations + unit tests
    provider/
      provider.go            # Resource registration & wiring
      resources/             # Pulumi resource implementations (vm, ha, pool…)
    testutils/               # Shared mock helpers (MockProxmoxClient)
sdk/                         # GENERATED — do not edit
examples/                    # YAML/Go Pulumi programs for manual testing
.golangci.yml                # Lint rules (repo root)
```

---

## Architecture — Three Layers

1. **Domain** (`provider/pkg/proxmox/`) — interfaces (`Client`, `VMOperations`, `HAOperations`, …) and domain models. No HTTP details.
2. **Adapters** (`provider/pkg/adapters/`) — implement domain interfaces against the real Proxmox HTTP API. Each adapter has its own `_test.go` with a mock HTTP server.
3. **Resources** (`provider/pkg/provider/resources/`) — implement `infer.CustomResource` interfaces. Depend only on domain interfaces, never on adapters directly. Wired together in `provider.go`.

---

## Code Style

- **Copyright header** required on every `.go` file (Apache 2.0, year 2025 — see `.golangci.yml` `goheader` template).
- **Imports**: ordered with `gci` — standard → blank → default → `github.com/pulumi/` → local.
- **Format**: `gofumpt` (stricter than `gofmt`).
- **No `github.com/golang/protobuf`** — use `google.golang.org/protobuf`.
- Run `make lint` to catch all of the above automatically.

---

## Testing Rules

- **All tests must be table-driven.** Define a `tests` slice of structs with `name`, inputs, and expected outputs; range over it with `t.Run`.
- Adapter tests use a real `httptest.Server` to mock Proxmox responses.
- Resource tests inject domain interface mocks (see `testutils.MockProxmoxClient`).
- The `-short` flag skips integration tests; unit tests must not require a live Proxmox instance.

```go
func TestExample(t *testing.T) {
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
            require.Equal(t, tt.expected, transform(tt.input))
        })
    }
}
```

---

## CI Checks (must all pass)

| Check | Command |
|---|---|
| Unit tests | `make test_provider` |
| Lint | `make lint` (golangci-lint with `.golangci.yml`) |
| Build | `make provider` |
| Schema diff (PR only) | schema-tools compares generated schema |
