# What's new with Allstar

Major features and changes added to Allstar.

## Added since last release

-

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
