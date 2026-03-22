# Design: Evidence Upload for Allstar Scorecard Policy

Companion to [proposal.md](proposal.md). This document captures the technical
design decisions.

## Configuration model

**Decision:** `upload: {sarif: true}` nested under the Scorecard policy config
(`scorecard.yaml`).

```yaml
# .allstar/scorecard.yaml
optConfig:
  optOutStrategy: true
action: issue
checks:
  - Binary-Artifacts
  - Signed-Releases
threshold: 8
upload:
  sarif: true
```

**Rationale:**

- Nested `upload` struct is extensible for v6 formats (`intoto`, `oscal`)
  without polluting the top-level config
- Scorecard policy level (not top-level `allstar.yaml`) keeps the feature
  scoped while testing
- Boolean per-format allows independent enable/disable
- Follows Allstar's existing flat + nested config patterns

**Go types:**

```go
type UploadConfig struct {
    SARIF bool `json:"sarif"`
}

// OrgConfig (concrete value with defaults):
Upload UploadConfig `json:"upload"`

// RepoConfig (pointer for "was this set?" override semantics):
Upload *UploadConfig `json:"upload,omitempty"`
```

**Alternatives considered:**

| Option | Rejected because |
|--------|-----------------|
| `sarifUpload: true` (flat boolean) | Not extensible for v6 formats |
| `action: [issue, sarif]` (multi-action array) | Requires refactoring Policy interface across all 9 policies |
| `upload: {enabled: true, format: sarif, ...}` (nested with format key) | Over-engineered before need is proven |

## Execution model

**Decision:** Dual execution — keep the per-check loop for issue text
generation, add a second full `sc.Run()` call for SARIF when upload is enabled.

**Rationale:**

The current Scorecard policy runs each check individually in a loop
(`scorecard.go:159-239`), building `NotifyText` for GitHub issues. It `break`s
on first error, yielding partial results. This model cannot produce valid SARIF
because `Result.AsSARIF()` needs a complete `scorecard.Result` from a single
`sc.Run()` call with all checks.

Refactoring the per-check loop to a single `Run()` would change error handling
semantics for the existing issue action. Dual execution preserves existing
behavior while adding SARIF capability.

**Flow:**

```
Check() {
    // Path 1: existing per-check loop (preserved)
    for _, check := range mc.Checks {
        result := scRun(repo, []string{check})
        // build NotifyText for issues
    }

    // Path 2: full run for SARIF (new, opt-in)
    if mc.Upload.SARIF {
        fullResult := scRun(repo, mc.Checks)  // all checks at once
        sarif := fullResult.AsSARIF(...)
        uploadSARIF(sarif)
    }
}
```

**Cost:** One additional Scorecard scan per repo when upload is enabled. This
is acceptable because:
- Upload is opt-in
- Change detection skips the upload when results haven't changed
- Allstar already scans every 5 minutes; the additional scan adds ~seconds

## SARIF generation

**Decision:** Use `scorecard/v5.Result.AsSARIF()` (already available as a
direct dependency at v5.4.0).

**AsSARIF() signature:**

```go
func (r *Result) AsSARIF(
    showDetails bool,
    logLevel log.Level,
    writer io.Writer,
    checkDocs docs.Doc,
    policy *spol.ScorecardPolicy,
    opts *options.Options,
) error
```

**Parameters** (pattern from scorecard-action `format.go`):

- `showDetails` = `true`
- `logLevel` = `sclog.DefaultLevel`
- `writer` = `bytes.Buffer`
- `checkDocs` = `checks.Read()` (scorecard check documentation)
- `policy` = constructed from Allstar's `checks` + `threshold` config:
  ```go
  policy := &spol.ScorecardPolicy{
      Version: 1,
      Policies: map[string]*spol.CheckPolicy{},
  }
  for _, check := range configuredChecks {
      policy.Policies[check] = &spol.CheckPolicy{
          Score: int32(threshold),
          Mode:  spol.CheckPolicy_ENFORCED,
      }
  }
  ```
- `opts` = minimal `options.Options{Repo: "owner/repo"}`

## Upload mechanism

**Decision:** Use `go-github/v74`'s `CodeScanning.UploadSarif()`.

scorecard-action delegates upload to the `github/codeql-action/upload-sarif`
GitHub Actions step. Allstar is a standalone app, so it calls the GitHub API
directly.

**GitHub API details:**

- Endpoint: `POST /repos/{owner}/{repo}/code-scanning/sarifs`
- Format: SARIF 2.1.0 only (no other formats supported by GitHub)
- Encoding: gzip compressed, base64 encoded
- Size limit: 10 MB compressed
- Permission: `security_events: write`
- Response: 202 Accepted

**go-github types:**

```go
type SarifAnalysis struct {
    CommitSHA *string `json:"commit_sha,omitempty"`
    Ref       *string `json:"ref,omitempty"`
    Sarif     *string `json:"sarif,omitempty"`
    ToolName  *string `json:"tool_name,omitempty"`
}
```

## Rate limiting

**Decision:** Change detection via SHA-256 hash of SARIF content. Skip upload
if unchanged since last upload.

**Rationale:** Allstar scans every 5 minutes. Most repos won't change between
cycles, so the vast majority of uploads get skipped. This mirrors how
`pkg/issue/issue.go` uses SHA-256 hashes to detect whether issue text changed.

**Implementation:** In-memory hash map (package-level, mutex-protected),
following the `pkg/scorecard/scorecard.go` caching pattern.

**Future improvement:** An `interval` field (`upload: {sarif: true, interval: 24h}`)
could be added if change detection proves insufficient for large orgs.

## Error handling

**Decision:** Non-blocking. Log a warning on upload failure but don't affect
the policy check result (`Result.Pass`).

**Rationale:** The policy check outcome shouldn't depend on whether GitHub
accepted the SARIF upload. Transient API failures (rate limits, timeouts)
should not disrupt the enforcement loop.

## Permission requirements

The Allstar GitHub App requires `security_events: write` permission for SARIF
upload. This is a new permission not currently declared.

**Rollout:**
- Self-hosted operators add the permission to their GitHub App immediately
- Public Allstar App (operated by OpenSSF) requires separate coordination
- The feature is dormant until the permission is granted (upload fails with
  403, logged as warning)

## Testability

All new functions use package-level mockable function variables, following the
existing pattern in `scorecard.go`:

```go
var (
    configFetchConfig func(...)  // existing
    scorecardGet      func(...)  // existing
    scRun             func(...)  // existing
    sarifUpload       func(...)  // new
)
```

Tests replace these with mocks. No integration tests against real GitHub API.
