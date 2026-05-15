# Copilot Instructions

## Repository Summary

This is a **Pulumi provider for Proxmox VE (PVE)** written in Go. It enables infrastructure-as-code management of Proxmox resources (VMs, storage, pools, HA groups, users, groups, roles, ACLs) using the [pulumi-go-provider](https://github.com/pulumi/pulumi-go-provider) infer framework. Multi-language SDKs (Go, Python, Node.js, .NET, Java) are **generated** from a JSON schema — never edit files under `sdk/` directly.

---

## Environment

All build tools (Go 1.26.0, golangci-lint, pulumi, golines, gofumpt, dlv, Java/Gradle for SDK gen) live exclusively inside the devcontainer. **Nothing is installed on the host.**

The devcontainer is defined by `Dockerfile` (repo root) + `.devcontainer/devcontainer.json`. The container is named `pulumi-pve`; the repo is mounted at `/workspaces/pulumi-pve` inside it.

### GitHub Copilot — runs inside the devcontainer

Copilot works from within the container (VSCode → "Reopen in Container"). All commands run directly in the integrated terminal with no extra setup.

### OpenCode — runs on the host, shells into the container for commands

OpenCode runs on the host machine, not inside the container. To execute any build/test/lint command, prefix it with `docker exec`:

```bash
# Start the container if it isn't running (idempotent)
docker start pulumi-pve

# Run any make/go command inside the container
docker exec -w /workspaces/pulumi-pve pulumi-pve make provider
docker exec -w /workspaces/pulumi-pve pulumi-pve make lint
docker exec -w /workspaces/pulumi-pve pulumi-pve make test_provider

# Interactive shell (for debugging)
docker exec -it -w /workspaces/pulumi-pve pulumi-pve bash
```

Source files are edited on the host (they are bind-mounted into the container), so file edits take effect immediately for the next `docker exec` command.

**`go.mod` lives at `provider/go.mod`**, not the repo root — all direct `go` commands must be run from `provider/` (or via `docker exec ... bash -c "cd provider && go ..."`).

### On-save formatter (inside the container / VS Code only)

The devcontainer VS Code config runs `golines ${file} -w -m 120 -t 2` on every `.go` save. Outside VS Code, run it manually before committing if lines exceed 120 chars.

---

## Languages & Frameworks

- **Go 1.26** — provider source lives entirely under `provider/`
- **pulumi-go-provider** — infer-based Pulumi resource framework (schema auto-generated from Go struct tags)
- **luthermonson/go-proxmox** — Proxmox HTTP API client
- **testify** — test assertions (`assert`/`require`)
- **golangci-lint v2** — enforced via `.golangci.yml`

---

## Build & Validate Commands

```bash
make provider            # Build provider binary only (fast)
make provider_debug      # Build with debug symbols (for dlv)
make build               # Build provider + all SDKs (slow)
make lint                # Run golangci-lint (run before committing)
make test_provider       # Run all provider unit tests
make generate_schema     # Regenerate schema.json from provider binary
make sdk/go              # Regenerate a single SDK (go/nodejs/python/dotnet/java)
make codegen             # generate_schema + all five SDKs (full regeneration)
make tidy                # go mod tidy — run after adding/removing dependencies
```

Run a single package's tests (from repo root):
```bash
cd provider && go test -v -count=1 -cover -timeout 2h -parallel 4 ./pkg/provider/resources/vm
```

Run a single test:
```bash
cd provider && go test -v -count=1 -cover -timeout 2h ./pkg/provider/resources/vm -run TestParseCPU
```

`make lint` uses `--path-prefix provider` — do not invoke golangci-lint directly without that flag or paths will be wrong.

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
    utils/                   # Utility functions (StringSliceChanged, etc.)
sdk/                         # GENERATED — do not edit
examples/                    # YAML/Go Pulumi programs for manual testing
.golangci.yml                # Lint rules (repo root)
.github/prompts/             # Reusable task-specific prompt files
```

---

## Architecture — Three Layers

1. **Domain** (`provider/pkg/proxmox/`) — interfaces and domain models. No HTTP details. Each resource has its own file (e.g. `ha.go`, `vm.go`).
2. **Adapters** (`provider/pkg/adapters/`) — implement domain interfaces against the real Proxmox HTTP/SSH API. Each adapter has a matching `_test.go`.
3. **Resources** (`provider/pkg/provider/resources/`) — implement `infer.CustomResource` interfaces. Depend **only** on domain interfaces, never on adapters directly. Wired together in `provider.go`.

The HA resource is the reference implementation of the adapter pattern.

### Do / Don't

| ✅ Do | ❌ Don't |
|---|---|
| Add `var _ proxmox.XOps = (*XAdapter)(nil)` compile-time checks | Import adapter packages from resource packages |
| Add `var _ = (infer.CustomResource[...])((*X)(nil))` in resource files | Edit anything under `sdk/` |
| Nil-check the operations field before every use in resources | Use `github.com/golang/protobuf` (use `google.golang.org/protobuf`) |
| Use `fmt.Errorf("...: %w", err)` for error wrapping | Share a mock server across parallel subtests |
| Call `t.Parallel()` in every test function and subtest | Use `NewAPIMock` for new tests (use `testutils.CreateMockServer` instead) |
| Check `request.DryRun` and return early in every CRUD method | Hand-edit `.github/workflows/*.yml` (auto-generated from `.ci-mgmt.yaml`) |

Resource-to-adapter wiring lives in `provider/pkg/provider/provider.go` (`NewProviderWithConfig`). Pass `nil` config to read credentials from Pulumi context at runtime; pass a non-nil `*config.Config` in tests.

---

## Code Style

### Copyright header — required on **every** `.go` file

Place as a `/* ... */` block comment immediately before `package` with **no blank line** between comment and `package`:

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

- **Formatter**: `gofumpt` (stricter than `gofmt`). Line length limit: 120 chars (`lll` linter). The devcontainer runs `golines ${file} -w -m 120 -t 2` on every `.go` save; outside VS Code run `golines -w -m 120 -t 2 <file>` manually.
- **`gocritic`**: enable-all except `hugeParam` and `importShadow`. Extra `govet` checks enabled: `nilness`, `reflectvaluecompare`, `sortslice`, `unusedwrite`.
- **Method receivers**: Use meaningful names derived from the type name (e.g., `adapter` for `*VMAdapter`). Single-letter receivers (`a`, `r`, `p`, `m`, `s`, etc.) are rejected by the linter.

---

## Schema and SDKs

Schema (`provider/cmd/pulumi-resource-pve/schema.json`) is generated from the compiled provider binary — it is not hand-edited. Generation strips the version field via `jq 'del(.version)'`. After changing resource inputs/outputs, run `make generate_schema` then regenerate the affected SDK(s).

---

## Testing Rules

- **All tests must be table-driven.** Define a `tests` slice of structs; range with `t.Run`.
- **Always call `t.Parallel()`** in every test function and every subtest.
- **Two test patterns — use the right one:**
  - **Adapter tests** (`*_adapter_test.go`, `package adapters_test`): use `testutils.CreateMockServer` to start a real `httptest.Server`, then wire `NewProxmoxAdapter` → `.Connect(ctx)` → `NewXAdapter`. Assert on both the captured HTTP request and the returned domain value.
  - **Resource tests** (`resources/<name>/<name>_test.go`): define a local `mockXOperations` struct with `func` fields; inject it directly into the resource struct. No HTTP server needed.
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

### Test helper conventions

Shared helpers live in `provider/pkg/testutils/`:
- `testutils.CreateMockServer(t, handler)` — wraps `httptest.NewServer`; returns server + captured-request store.
- `testutils.NewMockAdapter(url)` — creates a `ProxmoxAdapter` pointing at the given `httptest` URL; call `.Connect(ctx)` before use.
- `testutils.MockProxmoxClient{DefaultNode, DefaultVMID}` — lightweight stub for resource-layer tests.
- `testutils.Ptr[T](v)` — generic helper to take a pointer to a literal value.

Each parallel subtest must create its **own** mock server — never share one across subtests.

### Mock server response format

`go-proxmox` expects the Proxmox API envelope even in tests:
```json
{"data": null}
{"data": {"key": "value"}}
```
A bare string or non-enveloped object causes silent misparsing.

---

## Key Implementation Quirks

- **Clone field** is never returned by the Proxmox API. It must always be carried forward from the user's inputs or from prior state (`preserveInputs` in `vm.go` and `readCurrentOutput`). Do not attempt to read it from the Proxmox response.
- **Tags order**: Proxmox returns tags sorted alphabetically regardless of submission order. Use `utils.StringSliceChanged` (order-insensitive) for comparison; `preserveInputs` restores the user's original order when content matches.
- **UpdateConfig guard**: `vm_adapter.go` no-ops when the options list is empty. Proxmox returns HTTP 500 for a `Config()` call with zero options — do not remove this guard.
- **TLS**: `InsecureSkipVerify: true` is intentional (Proxmox self-signed certs). The `//nolint:gosec` comment is required or lint will fail.

---

## Adding a New Resource — Checklist

When adding a new resource, create **all** of the following (in order):

1. `provider/pkg/proxmox/<name>.go` — `<Name>Operations` interface + domain models + API struct
2. `provider/pkg/adapters/<name>_adapter.go` — adapter implementing the interface
3. `provider/pkg/adapters/<name>_adapter_test.go` — httptest-based adapter tests
4. `provider/pkg/provider/resources/<name>/<name>.go` — Pulumi resource (uses interface, not adapter)
5. `provider/pkg/provider/resources/<name>/<name>_test.go` — resource unit tests with mock interface
6. `provider/pkg/provider/provider.go` — add `new<Name>ResourceWithConfig` wiring + register in `Resources` slice

See `.github/prompts/new-resource.prompt.md` for the full scaffold guide. Adapter test conventions are in `.github/prompts/new-adapter-test.prompt.md`. Verify with:

```bash
cd provider && go build ./...   # must compile
make test_provider               # all tests must pass
make lint                        # zero warnings
```

---

## Integration Testing Against the Real Cluster

Prerequisites: `PULUMI_CONFIG_PASSPHRASE` must be set. Build the binary first (`Pulumi.yaml` loads the plugin from `../../bin`).

```bash
export PULUMI_CONFIG_PASSPHRASE=<passphrase>
make provider          # compile binary into bin/
make up                # pulumi login --local + stack init dev + pulumi up -y
make preview           # dry-run with diff; safe to run anytime
make update            # re-deploy after code changes (skips stack init)
make refresh           # reconcile state with real Proxmox state
make down              # pulumi destroy -y + remove stack (clean slate)
```

`make up` will fail if the `dev` stack already exists — use `make update` after the first deploy.

The Makefile exports `PULUMI_IGNORE_AMBIENT_PLUGINS=true` automatically; set it manually when running `pulumi` CLI directly.

Alternative swap-in program files in `examples/yaml/`: `Pulumi.vm.yaml` (VM only), `Pulumi.empty.yaml` (connectivity test only).

---

## Provider Configuration

```yaml
config:
  pve:pveUrl: "https://proxmox-host:8006"
  pve:pveUser: "user@pam"
  pve:pveToken: "token-id=secret"
  pve:sshUser: "root"     # optional, for SSH operations
  pve:sshPass: "password" # optional, for SSH operations
```

`PVE_API_URL` environment variable overrides `pveUrl` if set.

---

## CI Workflows

`.github/workflows/*.yml` are auto-generated from `.ci-mgmt.yaml` — do not hand-edit them. To regenerate: `make ci-mgmt`.

---

## Release

Tag `v*.*.*` on GitHub → CI publishes to NPM, NuGet, and PyPI automatically.
