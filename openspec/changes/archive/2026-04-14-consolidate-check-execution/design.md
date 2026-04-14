# Design: Consolidate Scorecard Check Execution

## Check() refactor

### Check name validation

Move validation before `scRun()`. Filter unknown checks, log warnings,
and pass only valid checks to `scRun()`.

```go
var validChecks []string
for _, n := range mc.Checks {
    if _, ok := checksAllChecks[n]; !ok {
        log.Warn()...Msg("Unknown scorecard check specified.")
        continue  // skip instead of break
    }
    validChecks = append(validChecks, n)
}
```

### Single scRun() call

```go
allRes, err := scRun(ctx, scc.ScRepo,
    sc.WithRepoClient(scc.ScRepoClient),
    sc.WithChecks(validChecks),
)
if err != nil {
    return nil, err
}
```

### Issue text from Result.Checks

Iterate `allRes.Checks` with the same threshold comparison and notify
text building logic. Per-check errors are logged and skipped instead of
breaking the loop.

### SARIF from same result

Pass `&allRes` directly to the refactored upload function instead of
calling `uploadSARIF()` which would re-run `scRun()`.

## uploadSARIF refactor

Rename to `uploadSARIFResult()`. Accept `*sc.Result` instead of running
checks internally. Remove `scRepo` and `scRepoClient` parameters.

Change detection (commit SHA comparison) remains in this function — it
gates the upload step, not the scan.

## Error handling

| Scenario | Before | After |
|----------|--------|-------|
| Unknown check name | `break` | `continue` (skip, run remaining) |
| `scRun()` error | `break` | `return nil, err` (infrastructure failure) |
| `CheckResult.Error` | `break` | `continue` (skip, run remaining) |
