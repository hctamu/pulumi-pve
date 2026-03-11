# New Adapter Test

You are adding or completing unit tests for an existing adapter in
`provider/pkg/adapters/<name>_adapter_test.go`.

Use the pattern established in `ha_adapter_test.go`. Follow every rule below.

---

## Setup pattern (repeat for every test function)

```go
server, captured := testutils.CreateMockServer(t, func(w http.ResponseWriter, r *http.Request, capturedReq *testutils.MockRequest) {
    // 1. Assert request method and path
    assert.Equal(t, http.MethodPost, r.Method)
    assert.Equal(t, "/cluster/<name>/", r.URL.Path)

    // 2. Decode and assert request body
    var body proxmox.<Name>Resource
    err := json.NewDecoder(strings.NewReader(capturedReq.Body)).Decode(&body)
    require.NoError(t, err)
    assert.Equal(t, "expected-value", body.SomeField)

    // 3. Write mock response
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte(`{"data": {...}}`))
})
defer server.Close()

cfg := &config.Config{
    PveURL:   server.URL,
    PveUser:  "test@pam",
    PveToken: "test-token",
}

proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
err := proxmoxAdapter.Connect(context.Background())
require.NoError(t, err)

adapter := adapters.New<Name>Adapter(proxmoxAdapter)
```

---

## Rules

1. **Package**: `adapters_test` (external test package — never `adapters`).
2. **All test functions are table-driven.** Every case goes in a `tests := []struct{...}` slice.
3. **Every test function and every subtest calls `t.Parallel()`.**
4. **Cover both happy path and error cases.** Error cases use `testutils.CreateMockServer` with a
   non-200 response or a server that returns malformed JSON.
5. **Assert on both sides:**
   - The `captured` struct (what was actually sent to the server).
   - The return value from the adapter method.
6. **Do not share a single server across subtests.** Each subtest creates its own server.
7. **HTTP 500 error case pattern:**
   ```go
   server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
       w.WriteHeader(http.StatusInternalServerError)
       _, _ = w.Write([]byte(`{"errors": {"detail": "internal error"}}`))
   })
   defer server.Close()
   ```
8. Copyright header required (see new-resource.prompt.md Step 1 for the exact text).
9. Import order: stdlib → blank → third-party → `github.com/pulumi/` → `github.com/hctamu/pulumi-pve/`.

---

## Minimum test coverage per adapter

| Operation | Cases to test |
|-----------|--------------|
| Create    | success, HTTP 500 |
| Get       | success, HTTP 404/500 |
| Update    | success, HTTP 500 |
| Delete    | success, HTTP 500 |

Add additional cases for interesting edge cases (e.g. optional fields, empty responses).

---

## Verify

```bash
cd provider && go test -v ./pkg/adapters/... -run Test<Name>Adapter
make lint
```
