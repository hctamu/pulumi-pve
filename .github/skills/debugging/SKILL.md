---
name: debugging
description: "Use when: debugging a bug, tracing an issue, or finding root cause. Systematic approach to finding and fixing bugs in the Pulumi Proxmox provider"
type: skill
---

# debugging Skill

**IMPORTANT: Every single answer you give when using this skill MUST start with the 🪲 emoji.**

When debugging a bug in this codebase, follow this systematic flow:

Start at the **domain layer** (interface contract) → **adapter layer** (HTTP/API) → **resource layer** (Pulumi CRUD).

### Step 1: Domain Contract (`provider/pkg/proxmox/<name>.go`)
- Understand the `<Name>Operations` interface: what inputs does it expect? What should it return?
- Check the domain models (structs) — are they correct for what the API supports?
- Look at the API struct — does it match the Proxmox documentation?
- **Question to ask:** Is the contract itself wrong, or is an implementation not meeting it?

### Step 2: Adapter Layer (`provider/pkg/adapters/<name>_adapter.go`)
- Check if the adapter correctly implements the `<Name>Operations` interface.
- Verify HTTP requests are formatted correctly (method, path, body, headers).
- Check error handling — are errors being swallowed, wrapped with context, or returned as-is?
- Verify response parsing — does the adapter correctly extract fields from the Proxmox API response?
- **Question to ask:** Does the HTTP request match Proxmox's API spec? Is the response being parsed correctly?

### Step 3: Resource Layer (`provider/pkg/provider/resources/<name>/<name>.go`)
- Check if the resource correctly calls the adapter (via the interface).
- Verify nil-checks on the operations field before every use.
- Check if inputs are being correctly transformed before passing to the adapter.
- Verify outputs are being correctly set from the adapter's returned domain model.
- **Question to ask:** Is the resource correctly wiring the domain model to Pulumi inputs/outputs?

## Common Bug Patterns

| Symptom | Check First |
|---------|------------|
| Silent failure (no error, but state doesn't change) | Nil-check on operations field in resource |
| Panic or null pointer dereference | Adapter is returning nil when resource didn't expect it |
| Wrong field value in output | Response parsing in adapter, or resource output mapping |
| API returns error | Adapter HTTP request (method, path, body) doesn't match Proxmox spec |
| Test passes locally but fails in CI | Mock server setup differs from adapter test (use `testutils.CreateMockServer`) |

## Tools & Commands

```bash
# Run tests for a single resource while debugging
cd provider && go test -v -count=1 -cover -timeout 2h ./pkg/provider/resources/vm -run TestCreate

# Run a single adapter test
cd provider && go test -v -count=1 -cover -timeout 2h ./pkg/adapters -run TestVMAdapterCreate

# View the domain models for a resource
grep -A 20 "type.*struct {" provider/pkg/proxmox/vm.go

# Check what the adapter sends to Proxmox
# (Look at adapter tests to see captured HTTP requests)
cd provider && go test -v ./pkg/adapters -run TestVMAdapterCreate 2>&1 | grep -i "request\|body"
```

## Nil-Check Gotcha

Always nil-check the operations field before use in resources — it fails **silently** if you don't:

```go
// BAD — will silently do nothing if ops is nil
if err := inputs.vm.ops.Create(ctx, vm); err != nil {
    // ...
}

// GOOD — panics loudly so you know something is wrong
if inputs.vm.ops == nil {
    return nil, fmt.Errorf("vm operations not initialized")
}
if err := inputs.vm.ops.Create(ctx, vm); err != nil {
    // ...
}
```
