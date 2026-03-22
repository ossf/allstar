# Tasks: Evidence Upload for Allstar Scorecard Policy

Implementation checklist organized by phase. Each phase is a separate commit.
TDD: write tests before implementation.

## Phase 1: Config model

- [x] 1.1 Add `UploadConfig` struct to `pkg/policies/scorecard/scorecard.go`
- [x] 1.2 Add `Upload` field to `OrgConfig`, `RepoConfig`, `mergedConfig`
- [x] 1.3 Update `mergeInRepoConfig()` to merge `Upload` field
- [x] 1.4 Write config merge tests (upload set at org level, overridden at
      repo level, disabled by default)

## Phase 2: SARIF generation

- [x] 2.1 Write tests for SARIF generation (mock `scRun`, verify output)
- [x] 2.2 Create `pkg/policies/scorecard/sarif.go`
- [x] 2.3 Implement `generateSARIF()` — run full `sc.Run()` with all checks,
      construct `ScorecardPolicy` from config, call `Result.AsSARIF()`
- [x] 2.4 Add mockable function variables for new dependencies

## Phase 3: SARIF upload

- [x] 3.1 Write tests for upload (mock `CodeScanning.UploadSarif()`, verify
      `SarifAnalysis` fields)
- [x] 3.2 Implement `uploadToCodeScanning()` — get repo default branch + HEAD
      SHA, compress + encode SARIF, call `CodeScanning.UploadSarif()`
- [x] 3.3 Write tests for gzip + base64 compression round-trip

## Phase 4: Change detection

- [x] 4.1 Write tests for change detection (upload once, skip on same commit
      SHA, upload on new commit SHA)
- [x] 4.2 Implement `uploadSARIF()` — orchestrate generation, commit SHA
      comparison, conditional upload
- [x] 4.3 Add in-memory commit SHA map with mutex

## Phase 5: Integration

- [x] 5.1 Write integration test (Check() with `upload.sarif: true` calls
      upload; with `false` does not)
- [x] 5.2 Add `uploadSARIF` call in `Check()` after per-check loop, guarded
      by `mc.Upload.SARIF`
- [x] 5.3 Verify non-blocking error handling (upload error does not affect
      `Result.Pass`)

## Phase 6: Documentation

- [x] 6.1 Add upload config section to `README.md` under Generic Scorecard
      Check
- [x] 6.2 Document `Code scanning alerts: Read & write` permission in `operator.md`

## Verification

- [x] `go vet ./pkg/policies/scorecard/...`
- [x] `go test -v ./pkg/policies/scorecard/...` (18 tests passing)
- [x] `go build ./cmd/allstar/`
- [x] Manual test: self-hosted operator SARIF upload to Code Scanning
- [x] Manual test: change detection skips upload on second cycle
