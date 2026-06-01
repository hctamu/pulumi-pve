# CLAUDE.md

See [.github/copilot-instructions.md](.github/copilot-instructions.md) for full project documentation (architecture, code style, testing patterns, implementation quirks).

## Quick Reference

Pulumi provider for Proxmox VE, written in Go using the `pulumi-go-provider` infer framework. Multi-language SDKs are generated — never edit `sdk/`.

### Build & Test

```bash
make provider          # build provider binary (fast)
make lint              # golangci-lint (run before committing)
make test_provider     # all unit tests
make generate_schema   # regenerate schema.json after changing inputs/outputs
make codegen           # schema + all SDKs
```

Single package/test:
```bash
cd provider && go test -v -count=1 -cover -timeout 2h -parallel 4 ./pkg/provider/resources/vm
cd provider && go test -v -count=1 -cover -timeout 2h ./pkg/provider/resources/vm -run TestParseCPU
```

`make lint` uses `--path-prefix provider` — don't invoke golangci-lint directly.

### Architecture (three layers)

1. **Domain** (`provider/pkg/proxmox/`) — interfaces + models
2. **Adapters** (`provider/pkg/adapters/`) — HTTP/SSH implementations
3. **Resources** (`provider/pkg/provider/resources/`) — Pulumi CRUD, depend only on domain interfaces

Wiring: `provider/pkg/provider/provider.go`. HA resource is the reference implementation.

### Key Conventions

- Copyright header required on every `.go` file (see copilot-instructions)
- Import ordering: stdlib / blank / third-party / pulumi / local (5 groups)
- All tests: table-driven, `t.Parallel()` in function and subtests
- Adapter tests: `testutils.CreateMockServer` with `{"data": ...}` envelope
- Resource tests: mock interface structs, no HTTP server
- Method receivers: meaningful names (e.g., `adapter`), not single letters
- Line limit: 120 chars (golines formatter)
- Commit format: Conventional Commits — `type(scope): description`

### New Resource Checklist

See [.github/prompts/new-resource.prompt.md](.github/prompts/new-resource.prompt.md) for the full scaffold guide.

### Integration Testing

```bash
export PULUMI_CONFIG_PASSPHRASE=<passphrase>
make up       # first deploy
make update   # re-deploy after changes
make preview  # dry-run
make down     # destroy + cleanup
```
