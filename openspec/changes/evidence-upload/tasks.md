# Tasks: Evidence Upload for Allstar Scorecard Policy

Implementation checklist organized by phase. Each phase is a separate commit.
TDD: write tests before implementation.

## Phase 1: Config model

- [ ] 1.1 Add `UploadConfig` struct to `pkg/policies/scorecard/scorecard.go`
- [ ] 1.2 Add `Upload` field to `OrgConfig`, `RepoConfig`, `mergedConfig`
- [ ] 1.3 Update `mergeInRepoConfig()` to merge `Upload` field
- [ ] 1.4 Write config merge tests (upload set at org level, overridden at
      repo level, disabled by default)

## Phase 2: SARIF generation

- [ ] 2.1 Write tests for SARIF generation (mock `scRun`, verify output)
- [ ] 2.2 Create `pkg/policies/scorecard/sarif.go`
- [ ] 2.3 Implement `generateSARIF()` — run full `sc.Run()` with all checks,
      construct `ScorecardPolicy` from config, call `Result.AsSARIF()`
- [ ] 2.4 Add mockable function variables for new dependencies

## Phase 3: SARIF upload

- [ ] 3.1 Write tests for upload (mock `CodeScanning.UploadSarif()`, verify
      `SarifAnalysis` fields)
- [ ] 3.2 Implement `uploadToCodeScanning()` — get repo default branch + HEAD
      SHA, compress + encode SARIF, call `CodeScanning.UploadSarif()`
- [ ] 3.3 Write tests for gzip + base64 compression round-trip

## Phase 4: Change detection

- [ ] 4.1 Write tests for change detection (upload once, skip on same hash,
      upload on different hash)
- [ ] 4.2 Implement `uploadSARIFIfNeeded()` — orchestrate generation, hash
      comparison, conditional upload
- [ ] 4.3 Add in-memory hash map with mutex

## Phase 5: Integration

- [ ] 5.1 Write integration test (Check() with `upload.sarif: true` calls
      upload; with `false` does not)
- [ ] 5.2 Add `uploadSARIF` call in `Check()` after per-check loop, guarded
      by `mc.Upload.SARIF`
- [ ] 5.3 Verify non-blocking error handling (upload error does not affect
      `Result.Pass`)

## Phase 6: Documentation

- [ ] 6.1 Add upload config section to `README.md` under Generic Scorecard
      Check
- [ ] 6.2 Document `security_events: write` permission in `operator.md`

## Verification

- [ ] `golangci-lint run ./pkg/policies/scorecard/...`
- [ ] `go test -v ./pkg/policies/scorecard/...`
- [ ] `go build ./cmd/allstar/`
- [ ] `go test ./...`
