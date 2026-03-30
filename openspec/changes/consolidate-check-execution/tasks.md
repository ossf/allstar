# Tasks: Consolidate Scorecard Check Execution

## Phase 1: OpenSpec and archive

- [ ] 1.1 Archive `openspec/changes/evidence-upload/` to
      `openspec/changes/archive/2026-03-28-evidence-upload/`
- [ ] 1.2 Create proposal, design, and tasks for this change

## Phase 2: Refactor Check() and uploadSARIF()

- [ ] 2.1 Refactor `Check()` to use a single `scRun()` call with all checks
- [ ] 2.2 Refactor `uploadSARIF()` to accept `*sc.Result` (rename to
      `uploadSARIFResult()`)
- [ ] 2.3 Update error handling: skip instead of break for unknown checks
      and per-check errors

## Phase 3: Update tests

- [ ] 3.1 Update `scorecard_test.go` mock expectations for single `scRun()`
- [ ] 3.2 Update `sarif_test.go` for new `uploadSARIFResult()` signature

## Verification

- [ ] `go test -v ./pkg/policies/scorecard/...`
- [ ] `go vet ./pkg/policies/scorecard/... ./cmd/allstar/`
- [ ] `go build ./cmd/allstar/`
