# What's new with Allstar

Major features and changes added to Allstar.

## Added since last release

-

## Release v3.0

- Branch Protection policy is more complete with support for
  `requireSignedCommits`,` enforceOnAdmins`,
  `requireCodeOwnerReviews`. [Link](pkg/policies/branch/branch.go)

- You may now opt-out repos that are forks with the `optOutForkedRepos` option.

- GitHub Actions policy added to allow/require/deny configured actions in
  workflows. [Docs](README.md#github-actions)

- Generic Scorecard policy added to run any Scorecard check with a score
  threshold. [Docs](README.md#generic-scorecard-check)

- Issue creation and pinging can be enabled / disabled based on a weekly
  schedule. [Link](pkg/config/config.go)

- The Outside Collaborators policy now allows
  exemptions. [Link](pkg/policies/outside/outside.go)

- When the Allstar action is changed from `issue` to `fix`. Existing issues
  will be closed.

- Issue ping duration is configurable at the operator level with
  `NOTICE_PING_DURATION_HOURS`. [Link](pkg/config/operator/operator.go)

- Org config may now point to a secondary repository for config and merge
  overrides. [Docs](README.md#org-level-base-and-merge-configuration-location)

- Individual repo config files are now allowed to be placed in the central org
  config repository. Example: in the `.allstar` repo, you can have a
  `<repo-name>/branch_protection.yaml` file with specific settings for that
  repo. [Docs](README.md#repo-policy-configurations-in-the-org-repo)

- Binary Artifacts policy configuration updated to have an ignore
  list. [Link](pkg/policies/binary/binary.go)

- Dangerous Workflow policy added. This policy checks the GitHub Actions
  workflow configuration files (.github/workflows), for any patterns that match
  known dangerous behavior. [Docs](README.md#dangerous-workflow)

## Release v2.0

- Branch Protection added the `requireStatusChecks` setting to ensure listed
  status checks are set in protection settings. Also enforces the
  `requireUpToDateBranch` option, if `requireStatusChecks` is set.

- You may now opt-out of repos marked as "archived" in GitHub with the
  `optOutArchivedRepos` option.

- Binary Artifacts policy issue text improved.

- A custom footer can be added to all issues created in an organization with
  the `issueFooter` option.

- Branch Protection now supports the "fix" action.

## Proposed functionality changes in v2.0

- Option `testingOwnerlessAllowed` in Outside Collaborator policy. Currently
  defaults true, proposal to default to false in next release.

  - Note: this was temporarily enabled in Jan, but then turned off due to a bug.

## Pre v2.0

Regular releases were not made before v2.0, so all previous notes are here.

- All issues for an org can be routed to a single repo using the `issueRepo`
  setting.

- Org config can now be located in `.github/allstar` as a secondary location
  after the `.allstar` repo.

- Issues can be created with a custom label using the `issueLabel` option.

- Private or Public repositories can be opt-out as a group with the
  `optOutPrivateRepos` or `optOutPublicRepos` options.

- We will retroactively call this Allstar v1.0: Allstar announced
  https://openssf.org/blog/2021/08/11/introducing-the-allstar-github-app/

- Initial policies and features built

- Allstar was proposed to the OpenSSF Securing Critical Projects WG and
  accepted https://youtu.be/o3SiBDUTCrw?t=300
