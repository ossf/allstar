# Proposal: Evidence Upload for Allstar Scorecard Policy

## Summary

Add an evidence upload capability to Allstar's Scorecard policy, starting with
SARIF upload to GitHub's Code Scanning API. This enables organization
administrators to get Scorecard findings in each repository's
**Security > Code Scanning** tab without requiring per-repository workflow setup.

This feature positions Allstar as a downstream evidence consumer aligned with
the [Scorecard v6 direction](https://github.com/ossf/scorecard/pull/4952),
which repositions Scorecard as an "open source security evidence engine."

## Motivation

### Problem

Organizations using Allstar's Scorecard policy get violation notifications via
GitHub issues, but findings do not appear in the GitHub Security tab. This
creates a gap:

- **Org admins** want centralized security visibility across all repositories
- **Security teams** want findings in the standard Security > Code Scanning
  dashboard alongside CodeQL, Dependabot, and other SAST/SCA tools
- **Compliance workflows** often require evidence in the Security tab for audit
  purposes

The alternative — deploying
[scorecard-action](https://github.com/ossf/scorecard-action) per repository —
requires per-repo workflow setup, which doesn't scale for large organizations.

### Why now

1. **Scorecard v5.4.0** provides `Result.AsSARIF()` for SARIF 2.1.0 generation
2. **Scorecard v6** (ossf/scorecard#4952) defines Allstar as an evidence
   consumer — this feature is Phase 1 of that architecture
3. **go-github/v74** (already an Allstar dependency) includes
   `CodeScanning.UploadSarif()` for the GitHub Code Scanning API

### Why Allstar

Allstar is the natural home for org-wide SARIF upload because:

- It already monitors all repositories in an organization continuously
- It already runs Scorecard checks via the Scorecard policy
- It has GitHub App authentication with per-installation API access
- It supports org-level configuration with repo-level overrides

scorecard-action generates SARIF and delegates upload to the
`github/codeql-action/upload-sarif` GitHub Actions step. Allstar is a
standalone app (not a GitHub Action), so it uploads SARIF directly via the
GitHub API.

## Current state

Allstar's Scorecard policy (`pkg/policies/scorecard/scorecard.go`):

- Runs configured Scorecard checks individually in a loop
- Compares scores against a configurable threshold
- Creates GitHub issues when checks fail (via the `issue` action)
- Does NOT generate SARIF or upload to Code Scanning

## Scope

### In scope

- **SARIF upload** to GitHub Code Scanning API via `go-github/v74`
- **Configuration** via `upload: {sarif: true}` on `scorecard.yaml`
- **Change detection** to avoid redundant uploads (SHA-256 hash comparison)
- **Non-blocking error handling** (upload failures don't affect policy results)
- **Documentation** for self-hosted operators (permission requirements)

### Out of scope

- Multi-action array refactor (`action: [issue, sarif]`)
- Upload interval configuration (change detection is sufficient for Phase 1)
- v6 evidence formats (in-toto, OSCAL, Gemara) — Phase 2 after v6 ships
- OSPS Baseline conformance enforcement — Phase 3
- Public Allstar App permission update (separate OpenSSF coordination)

### Future phases

| Phase | Deliverable | Trigger |
|-------|-------------|---------|
| Phase 1 (this) | SARIF upload to GitHub Code Scanning | Now |
| Phase 2 | Evidence bundle upload (in-toto, Gemara, OSCAL) | Scorecard v6 ships |
| Phase 3 | OSPS Baseline conformance enforcement at org level | v6 conformance layer stable |

## Ecosystem alignment

Per the [Scorecard v6 proposal](https://github.com/ossf/scorecard/pull/4952):

> Allstar, a Scorecard sub-project, continuously monitors GitHub organizations
> and enforces Scorecard check results as policies. OSPS conformance output
> could enable Allstar to enforce Baseline conformance at the organization level.

Scorecard v6 design principles relevant to this feature:

- **"Evidence is the product."** Scorecard produces evidence; Allstar consumes
  and uploads it.
- **"All consumers are equal."** Allstar consumes Scorecard output through
  published interfaces (`Result.AsSARIF()`), not special integration.
- **"Formats are presentation."** SARIF is one view of the evidence model.
  The upload infrastructure is designed to support additional formats when v6
  ships.

## Config file naming

Both Allstar and Scorecard use `scorecard.yaml` but in different locations:

- **Allstar**: `.allstar/scorecard.yaml` (in the `.allstar` org config repo)
- **Scorecard annotations**: `scorecard.yaml` in the repo root or `.github/`

No path overlap. The `upload` key does not conflict with Scorecard's
`annotations` schema.

## Approval

This is a self-contained feature within the Allstar codebase. Per the
PR-driven governance approach:

- Open a well-documented PR on `ossf/allstar`
- Self-hosted operators can use it immediately (requires adding
  `security_events: write` to their GitHub App)
- Public Allstar App permission update coordinated separately with OpenSSF
