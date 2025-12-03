# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is a Pulumi provider for managing Proxmox VE (PVE) resources. It's built using the [pulumi-go-provider](https://github.com/pulumi/pulumi-go-provider) framework and allows infrastructure-as-code management of Proxmox resources like VMs, storage, pools, HA groups, users, groups, roles, and ACLs.

## Development Environment

Development MUST be done inside the provided devcontainer. The devcontainer includes all necessary tools and dependencies. Open the repository in VSCode and use "Reopen in Container" to start.

## Common Commands

### Building and Installing
```bash
# Build provider binary and generate all SDKs (dotnet, go, nodejs, python, java)
make build

# Build provider only (without SDKs)
make provider

# Install provider locally (copies binary to $GOPATH/bin and sets up nodejs/dotnet SDKs)
make build install
```

### Testing
```bash
# Run all provider tests
make test_provider

# Run tests in a specific package
cd provider && go test -v -count=1 -cover -timeout 2h -parallel 4 ./pkg/provider/resources/vm

# Run a specific test
cd provider && go test -v -count=1 -cover -timeout 2h ./pkg/provider/resources/vm -run TestParseCPU

# Run all tests including SDK tests
make test_all
```

### Linting
```bash
# Run golangci-lint on provider code
make lint
```

### SDK Generation
```bash
# Generate schema file from provider
make generate_schema

# Generate specific SDK
make sdk/go
make sdk/nodejs
make sdk/python
make sdk/dotnet
make sdk/java
```

### Local Development with Pulumi
```bash
# Initialize a new stack for testing
make stack_init

# Preview changes
make preview

# Deploy resources
make update

# Refresh state
make refresh

# Destroy resources and remove stack
make down
```

## Architecture

### Provider Structure

The provider follows a clean architecture with clear separation of concerns:

- **[provider/cmd/pulumi-resource-pve/main.go](provider/cmd/pulumi-resource-pve/main.go)**: Entry point that starts the provider server
- **[provider/pkg/provider/provider.go](provider/pkg/provider/provider.go)**: Main provider registration that lists all resources and configuration
- **[provider/pkg/config/config.go](provider/pkg/config/config.go)**: Provider configuration struct (PveURL, PveUser, PveToken, SSHUser, SSHPass)
- **[provider/pkg/proxmox/](provider/pkg/proxmox/)**: Domain layer with business logic and interfaces
  - Domain models (HAInputs, HAOutputs, HAState, etc.)
  - Operation interfaces (HAOperations, ProxmoxClient)
  - API-level structs (HaResource) used for HTTP communication
- **[provider/pkg/adapters/](provider/pkg/adapters/)**: Adapter layer implementing domain interfaces
  - [proxmox_adapter.go](provider/pkg/adapters/proxmox_adapter.go): HTTP client adapter wrapping luthermonson/go-proxmox
  - [ha_adapter.go](provider/pkg/adapters/ha_adapter.go): HA resource operations adapter
  - Comprehensive unit tests with mock HTTP servers
- **[provider/pkg/provider/resources/](provider/pkg/provider/resources/)**: Pulumi resource implementations
  - [vm/](provider/pkg/provider/resources/vm/): Virtual machine resource with full CRUD + Diff
  - [storage/](provider/pkg/provider/resources/storage/): Storage file resource
  - [pool/](provider/pkg/provider/resources/pool/): Resource pool management
  - [ha/](provider/pkg/provider/resources/ha/): High availability group resource (refactored to use adapters)
  - [user/](provider/pkg/provider/resources/user/): User management
  - [group/](provider/pkg/provider/resources/group/): Group management
  - [role/](provider/pkg/provider/resources/role/): Role management
  - [acl/](provider/pkg/provider/resources/acl/): Access control list management
  - [utils/](provider/pkg/provider/resources/utils/): Shared utility functions

### Layered Architecture Pattern

The provider uses a three-layer architecture for better testability and maintainability:

1. **Domain Layer** ([provider/pkg/proxmox/](provider/pkg/proxmox/)): Contains business logic, domain models, and interfaces
   - Defines what operations are available (e.g., HAOperations interface)
   - Domain models represent business concepts (HAInputs, HAOutputs)
   - Independent of infrastructure concerns (HTTP, API details)

2. **Adapter Layer** ([provider/pkg/adapters/](provider/pkg/adapters/)): Implements domain interfaces
   - ProxmoxAdapter: Wraps the luthermonson/go-proxmox HTTP client
   - HAAdapter: Implements HAOperations by converting domain models to API structs
   - Each adapter has comprehensive unit tests with mock HTTP servers
   - Adapters handle API-specific concerns (URL formatting, request/response marshaling)

3. **Resource Layer** ([provider/pkg/provider/resources/](provider/pkg/provider/resources/)): Pulumi resource implementations
   - Implements pulumi-go-provider infer interfaces
   - Uses adapters to interact with Proxmox API
   - Handles Pulumi-specific concerns (state management, diffs, validation)

### Resource Implementation Pattern

Resources in this provider follow the pulumi-go-provider infer pattern. Each resource implements:

- `infer.CustomResource[Inputs, Outputs]` - Base resource interface
- `infer.CustomCreate` - Create operation
- `infer.CustomRead` - Read/refresh operation
- `infer.CustomUpdate` - Update operation
- `infer.CustomDelete` - Delete operation
- `infer.CustomDiff` (optional) - Custom diff logic for complex resources

The VM resource is the most complex, implementing all CRUD operations plus custom diff logic for handling disk resizing, configuration changes, and determining when replacements are necessary.

### Adapter Pattern Benefits

The new architecture (currently implemented for HA resource, to be rolled out to others) provides:

- **Testability**: Adapters can be tested independently with mock HTTP servers
- **Separation of Concerns**: Domain logic separated from API communication
- **Type Safety**: Clear boundaries between domain models and API structs
- **Maintainability**: Changes to API details don't affect domain logic
- **Mockability**: Domain interfaces can be easily mocked for resource tests

### Client Architecture

The ProxmoxAdapter ([provider/pkg/adapters/proxmox_adapter.go](provider/pkg/adapters/proxmox_adapter.go)) wraps the [luthermonson/go-proxmox](https://github.com/luthermonson/go-proxmox) client:
1. Uses `sync.Once` to ensure only one HTTP client is created per adapter instance
2. Reads configuration from provider config (PveURL, PveUser, PveToken)
3. Creates HTTP client with TLS verification disabled (required for Proxmox self-signed certs)
4. Provides Get/Post/Put/Delete methods that handle API response unwrapping

The ProxmoxAdapter implements the ProxmoxClient interface, which defines basic HTTP operations. This interface allows for easy mocking in tests via the SetClient method.

### Schema Generation

The provider uses Pulumi's infer system to automatically generate the schema from Go struct tags. The schema is generated by running the provider binary with schema generation commands (see [Makefile](Makefile) line 43-45). This schema is then used to generate SDKs for all supported languages.

### Testing Strategy

The provider uses multiple testing approaches:

- **Adapter Unit Tests**: Test HTTP communication with mock servers
  - Use httptest.Server to create test HTTP endpoints
  - Verify request methods, paths, headers, and bodies
  - Test both success and error scenarios
  - Example: [ha_adapter_test.go](provider/pkg/adapters/ha_adapter_test.go) has 17 tests with 83.9% coverage

- **Resource Unit Tests**: Test resource logic with mocked adapters
  - Mock domain interfaces (HAOperations, etc.)
  - Test CRUD operations and state management
  - Verify Pulumi-specific behavior (diffs, validation)

- **Table-Driven Tests**: Used for utility functions and parsers
  - Multiple test cases with input/expected output pairs
  - Easy to add new test cases

- **Integration Tests**: Can be run against real Proxmox
  - Located in [examples/yaml/](examples/yaml/)
  - Test full end-to-end provider behavior

## Code Style Requirements

All Go code must follow the linting rules defined in [.golangci.yml](.golangci.yml):

- **Copyright headers**: All `.go` files must include the Apache 2.0 license header (enforced by goheader linter)
- **Import ordering**: Use gci formatter with standard → blank → default → github.com/pulumi/ → local prefix ordering
- **Formatting**: Code must be formatted with gofumpt
- **Security**: gosec linter enforces security best practices (e.g., properly handling credentials)
- **No protobuf v1**: Use `google.golang.org/protobuf` instead of deprecated `github.com/golang/protobuf`

Run `make lint` before committing to ensure compliance.

## Provider Configuration

The provider requires the following configuration:

```yaml
config:
  pve:pveUrl: "https://proxmox-host:8006"
  pve:pveUser: "user@pam"
  pve:pveToken: "token-id=secret"
  pve:sshUser: "root"  # Optional, for operations requiring SSH
  pve:sshPass: "password"  # Optional, for operations requiring SSH
```

Configuration can also be set via environment variables:
- `PVE_API_URL` overrides `pveUrl` if set

## Release Process

Releases are automated via GitHub Actions:
1. Create a new release on GitHub with tag format `v*.*.*` (e.g., `v1.2.3`)
2. The [release.yml](.github/workflows/release.yml) workflow automatically builds and publishes SDKs to:
   - NPM: https://www.npmjs.com/settings/hctamu/packages
   - NuGet: https://www.nuget.org/packages/Hctamu.Pve
   - PyPI: https://pypi.org/project/pulumi-pve/

## Key Dependencies

- **github.com/pulumi/pulumi-go-provider**: Framework for building Pulumi providers in Go
- **github.com/luthermonson/go-proxmox**: Proxmox API client library
- **github.com/pulumi/pulumi/sdk/v3**: Pulumi SDK for provider development
- **github.com/vitorsalgado/mocha/v3**: HTTP mocking for tests
- **golang.org/x/crypto**: SSH client implementation

## Important Notes

- The provider disables TLS verification for Proxmox API calls (common for self-signed certs)
- VM resource supports both full clones and linked clones
- Disk resizing is supported but requires careful diff logic to avoid unnecessary replacements
- The provider uses Pulumi's infer system for automatic schema generation from Go types
- Test parallelism is set to 4 (`TESTPARALLELISM := 4` in Makefile)
