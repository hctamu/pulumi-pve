# Fix Lint

You are fixing `golangci-lint` failures in this repository.

---

## Step 1 — Run lint and capture output

```bash
make lint
```

Capture the full output. Each issue has the form:
```
provider/pkg/foo/bar.go:42:5: <linter>: <message>
```

---

## Step 2 — Categorise issues

Group by linter type before fixing:

| Linter | What it checks | How to fix |
|--------|---------------|------------|
| `goheader` | Missing/wrong copyright header | Add the exact header below as a `/* ... */` block comment at the very top of the file, before `package` |
| `gci` | Import ordering | Re-order imports into four groups (see below) |
| `gofumpt` | Formatting | Run `gofumpt -w <file>` or let the editor format on save |
| `revive` | Code style | Read the message; common fixes: remove unused vars, simplify expressions |
| `gosec` | Security | Read the message; do not suppress with `//nolint` unless genuinely a false positive |
| `gocritic` | Code quality | Read the message; typical fixes: use `errors.As`, remove redundant type conversions |
| `lll` | Line too long (>120 chars) | Break the line; for strings use concatenation or a variable |

---

## Required copyright header (exact text — every `.go` file)

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

Place this **before** the `package` declaration. No blank line between the comment and `package`.

---

## Required import ordering (gci)

Four groups separated by blank lines, in this order:

```go
import (
    // 1. Standard library
    "context"
    "fmt"

    // 2. Blank imports (rare)
    _ "embed"

    // 3. Third-party
    "github.com/stretchr/testify/assert"

    // 4. Pulumi packages
    p "github.com/pulumi/pulumi-go-provider"
    "github.com/pulumi/pulumi-go-provider/infer"

    // 5. Local (this module)
    "github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)
```

Empty groups can be omitted. Do **not** mix groups.

---

## Step 3 — Apply fixes

Fix all issues reported by lint. For formatting issues, you can also run:

```bash
cd provider && gofumpt -w ./...
```

---

## Step 4 — Verify

```bash
make lint
```

The output must be empty (exit code 0) before the fix is complete.
If new issues appear, repeat from Step 2.
