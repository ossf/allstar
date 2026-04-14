# Tasks: Consolidate Scorecard Check Execution

## Phase 1: OpenSpec and archive

- [x] 1.1 Archive `openspec/changes/evidence-upload/` to
      `openspec/changes/archive/2026-03-28-evidence-upload/`
- [x] 1.2 Create proposal, design, and tasks for this change

## Phase 2: Refactor Check() and uploadSARIF()

- [x] 2.1 Refactor `Check()` to use a single `scRun()` call with all checks
- [x] 2.2 Refactor `uploadSARIF()` to accept `*sc.Result`
- [x] 2.3 Update error handling: skip instead of break for unknown checks
      and per-check errors; restore explanatory comment

## Phase 3: Update tests

- [x] 3.1 Update `scorecard_test.go` — existing TestCheck works as-is;
      add TestCheckUnknownCheckSkipped and TestCheckPerCheckErrorSkipped
- [x] 3.2 Update `sarif_test.go` for new `uploadSARIF()` signature

## Verification

- [x] `go test -v ./pkg/policies/scorecard/...` (20 tests pass)
- [x] `go vet ./pkg/policies/scorecard/... ./cmd/allstar/`
- [x] `go build ./cmd/allstar/`
- [x] PR #807 merged; follow-up #816 open
