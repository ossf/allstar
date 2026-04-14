# Proposal: Consolidate Scorecard Check Execution

## Summary

Replace the per-check loop + separate SARIF batch run with a single
`sc.Run()` call. Currently, when `upload.sarif: true` is configured,
every Scorecard check runs **N+1 times** — N times individually for issue
text, then once more as a batch for SARIF and results output. This refactor
eliminates the duplication.

## Motivation

The dual execution path was introduced as a pragmatic choice during the
evidence-upload feature (see `openspec/changes/archive/2026-03-28-evidence-upload/`):
the existing per-check loop was preserved to avoid risk, and SARIF generation
was added as a second `scRun()` call. Now that the feature is tested and
merged, the duplication should be removed.

### Cost of duplication

- **CPU/I/O**: Each check downloads and analyzes the repository tarball.
  Running checks N+1 times means N+1x the compute cost.
- **API calls**: Each `scRun()` call consumes GitHub API quota for repository
  metadata, branch protection, workflow files, etc.
- **Latency**: A full org scan of 10 repos with 20 checks takes ~10 minutes.
  Eliminating the duplicate run could reduce this significantly.

### Why consolidation is safe

Investigation of Scorecard's `Run()` function (v5.4.0) confirms:

- All checks run concurrently in separate goroutines via `runEnabledChecks()`
- Per-check errors are captured in `CheckResult.Error`, not as a top-level
  return error
- `Result.Checks` always contains all results — passing, failing, and errored
- The top-level `error` from `Run()` is only for infrastructure failures
  (repo clone, client init)
- The per-check loop's break-on-error behavior is actually more fragile
  than batch execution — it loses results for subsequent checks

## Current state

```
for _, n := range mc.Checks:        ← N sequential scRun() calls
    scRun(WithChecks([n]))
    validate result, build notify text

if mc.Upload.SARIF:
    uploadSARIF()                    ← 1 additional batch scRun()
        scRun(WithChecks(mc.Checks))
        resultToSARIF()
        collectResult()
        uploadToCodeScanning()
```

## Proposed state

```
scRun(WithChecks(validChecks))       ← 1 batch scRun() call

for _, res := range result.Checks:   ← iterate for issue text
    build notify text (same logic)

if mc.Upload.SARIF:
    uploadSARIFResult(&result)       ← use same result
        resultToSARIF()
        collectResult()
        uploadToCodeScanning()
```

## Scope

### In scope

- Consolidate `Check()` to a single `scRun()` call
- Refactor `uploadSARIF()` to accept a `*sc.Result`
- Improve error handling: skip errored checks instead of breaking
- Update tests

### Out of scope

- Change detection repositioning (remains scoped to SARIF upload)
- Config model changes
- New features

## Behavioral changes

| Behavior | Before | After |
|----------|--------|-------|
| Check execution count | N+1 | 1 |
| Unknown check name | Breaks loop, loses subsequent | Filtered before Run(), remaining execute |
| Per-check error | Breaks loop, loses subsequent | Skipped, remaining still produce results |
| Check execution order | Sequential | Concurrent (Scorecard runs all in parallel) |
