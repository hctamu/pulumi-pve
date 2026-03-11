# New Resource Scaffold

You are adding a new resource to this Pulumi provider for Proxmox VE.
Follow the three-layer architecture exactly. Work through each step in order.

## Step 0 — Determine the resource name

Ask the user (or infer from context) the resource name in PascalCase (e.g. `Firewall`).
All paths below use `<Name>` as a placeholder — replace it throughout.

---

## Step 1 — Domain layer: `provider/pkg/proxmox/<name>.go`

Create this file. It must contain:

1. **`<Name>Operations` interface** with `Create`, `Get`, `Update`, `Delete` methods.
   Each method takes `context.Context` as the first arg.
2. **Domain model structs**: `<Name>Inputs` and `<Name>Outputs` (outputs embeds inputs).
   - `<Name>Inputs` fields carry `pulumi:"..."` and optional `provider:"replaceOnChanges"` tags.
   - Add an `Annotate(a infer.Annotator)` method on `*<Name>Inputs`.
3. **API-level struct** (e.g. `<Name>Resource`) used only for JSON HTTP serialisation — no pulumi tags.
4. Add a compile-time check if there are any enum-like string types (see `HAState` for the pattern).

Copyright header **must** be the first thing in the file:

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

---

## Step 2 — Adapter layer: `provider/pkg/adapters/<name>_adapter.go`

1. Add a compile-time interface check:
   ```go
   var _ proxmox.<Name>Operations = (*<Name>Adapter)(nil)
   ```
2. The adapter struct holds `client proxmox.Client`.
3. Constructor: `func New<Name>Adapter(client proxmox.Client) *<Name>Adapter`.
4. Implement every method of `<Name>Operations`:
   - Convert domain inputs → API struct → call `ha.client.{Get,Post,Put,Delete}`.
   - Wrap errors: `fmt.Errorf("failed to create <name> resource: %w", err)`.
   - Convert API response → domain outputs on the way back.
5. Define path constant(s) at the top: `const <name>ResourcePath = "/cluster/<name>/"`.

---

## Step 3 — Adapter test: `provider/pkg/adapters/<name>_adapter_test.go`

Use the httptest-based pattern (see `ha_adapter_test.go`):

- Package: `adapters_test`.
- Each test function covers one operation (Create, Get, Update, Delete) plus error cases.
- Each subtest:
  1. Creates a mock HTTP server with `testutils.CreateMockServer`.
  2. Builds `config.Config{PveURL: server.URL, ...}`.
  3. Calls `adapters.NewProxmoxAdapter(cfg)`, then `.Connect(ctx)`.
  4. Calls `adapters.New<Name>Adapter(proxmoxAdapter)`.
  5. Asserts on both the captured HTTP request **and** the returned domain value.
- All subtests call `t.Parallel()`.
- All test functions are table-driven (see Testing Rules in copilot-instructions.md).

---

## Step 4 — Resource: `provider/pkg/provider/resources/<name>/<name>.go`

1. Package comment and copyright header.
2. Compile-time interface checks for all infer interfaces the resource implements:
   ```go
   var (
       _ = (infer.CustomResource[proxmox.<Name>Inputs, proxmox.<Name>Outputs])((*<Name>)(nil))
       _ = (infer.CustomDelete[proxmox.<Name>Outputs])((*<Name>)(nil))
       _ = (infer.CustomUpdate[proxmox.<Name>Inputs, proxmox.<Name>Outputs])((*<Name>)(nil))
       _ = (infer.CustomRead[proxmox.<Name>Inputs, proxmox.<Name>Outputs])((*<Name>)(nil))
   )
   ```
3. Resource struct holds only `<Name>Ops proxmox.<Name>Operations` — never an adapter directly.
4. Every CRUD method:
   - Checks `request.DryRun` and returns early with a preview response if true.
   - Nil-checks `<name>.<Name>Ops` and returns `errors.New("<Name>Operations not configured")`.
   - Uses `p.GetLogger(ctx).Debugf(...)` for observability.
5. Import order: stdlib → blank → third-party → `github.com/pulumi/` → `github.com/hctamu/pulumi-pve/`.

---

## Step 5 — Resource tests: `provider/pkg/provider/resources/<name>/<name>_test.go`

- Package: `<name>_test`.
- Define a local `mock<Name>Operations` struct with `func` fields for each method.
- All test functions are table-driven.
- Test at minimum: Create (success + preview + nil ops), Delete (success + nil ops),
  Update (success), Read (success + not found).

---

## Step 6 — Register in `provider/pkg/provider/provider.go`

1. Add a wiring function:
   ```go
   func new<Name>ResourceWithConfig(cfg *config.Config) *<name>.<Name> {
       proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
       <name>Adapter := adapters.New<Name>Adapter(proxmoxAdapter)
       return &<name>.<Name>{<Name>Ops: <name>Adapter}
   }
   ```
2. Add `infer.Resource(new<Name>ResourceWithConfig(cfg))` to the `Resources` slice in `NewProviderWithConfig`.

---

## Step 7 — Verify

Run in order:
```bash
cd provider && go build ./...          # must compile
make test_provider                     # all tests must pass
make lint                              # must produce zero warnings
```

Fix any issues before declaring the resource done.
