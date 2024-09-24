[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/ossf/allstar/badge)](https://api.scorecard.dev/projects/github.com/ossf/allstar)

<img align="right" src="artwork/openssf_allstar_alt.png" width="300" height="400">

# **Allstar**

## Overview

-  [What Is Allstar?](#what-is-allstar)

## What's new with Allstar

- [whats-new.md](whats-new.md)

## Disabling Unwanted Issues

-  [Help! I'm getting issues created by Allstar and I don't want them!](#disabling-unwanted-issues-1) 

## Getting Started

-  [Background](#background)
-  [Org-Level Options](#org-level-options)
-  [Installation Options](#installation-options)
    - [Quickstart Installation](#quickstart-installation)
    - [Manual Installation](#manual-installation)

## Policies and Actions
- [Actions](#actions)
- [Policies](#policies)

## Advanced
- [Configuration Definitions](#configuration-definitions)
- [Example Configurations](#example-config-repository)
- [Run Your Own Instance of Allstar](operator.md)

## Contribute
- [Contributing](#contributing)
________
________

## Overview

### What is Allstar?

Allstar is a GitHub App that continuously monitors GitHub organizations or
repositories for adherence to security best practices.  If Allstar detects a
security policy violation, it creates an issue to alert the repository or
organization owner.  For some security policies, Allstar can also automatically
change the project setting that caused the violation, reverting it to the
expected state.

Allstar’s goal is to give you finely tuned control over the files and settings
that affect the security of your projects.  You can choose which security
policies to monitor at both the organization and repository level, and how to
handle policy violations.  You can also develop or contribute new policies.

Allstar is developed as a part of the [OpenSSF Scorecard](https://github.com/ossf/scorecard) project.

## [What's new with Allstar](whats-new.md)

## Disabling Unwanted Issues
If you're getting unwanted issues created by Allstar, follow [these directions](opt-out.md) to opt out. 

## Getting Started

### Background

Allstar is highly configurable. There are three main levels of controls: 

- **Org level**: Organization administrators can choose to enable Allstar on: 
   -  all repositories in the org; 
   -  most repositories, except some that are opted out; 
   -  just a few repositories that are opted in. 

These configurations are done in the organization's `.allstar` repository.

- **Repo level:** Repository maintainers in an organization that uses
   Allstar can choose to opt their repository in or out of organization-level
   enforcements. Note: these repo-level controls are only functional when "repo
   override" is allowed in the org-level settings. These configurations are
   done in the repository's `.allstar` directory.

- **Policy level:** Administrators or maintainers can choose which policies
   are enabled on specific repos and which actions Allstar takes when a policy
   is violated. These configurations are done in a policy yaml file in either
   the organization's `.allstar` repository (admins), or the repository's
   `.allstar` directory (maintainers). 

### Org-Level Options 

Before installing Allstar at the org level, you should decide approximately how many repositories
you want Allstar to run on. This will help you choose between the Opt-In and
Opt-Out strategies. 

-  The Opt In strategy allows you to manually add the repositories you'd
   like Allstar to run on. If you do not specify any repositories, Allstar will
   not run despite being installed. Choose the Opt In strategy if you want to enforce
   policies on only a small number of your total repositories, or want to try
   out Allstar on a single repository before enabling it on more. Since the
   v4.3 release, globs are supported to easily add multiple repositories with
   a similar name.

-  The Opt Out strategy (recommended) enables Allstar on all repositories
   and allows you to manually select the repositories to opt out of Allstar
   enforcements. You can also choose to opt out all public repos, or all
   private repos. Choose this option if you want to run Allstar on all
   repositories in an organization, or want to opt out only a small number of
   repositories or specific type (i.e., public vs. private) of repository.
   Since the v4.3 release, globs are supported to easily add multiple
   repositories with a similar name.

<table>
<thead>
<tr>
<th></th>
<th><strong>Opt Out (Recommended)</strong><br>
<strong>optOutStrategy = true</strong></th>
<th><strong>Opt In</strong><br>
<strong>optOutStrategy = false</strong></th>
</tr>
</thead>
<tbody>
<tr>
<td>Default behavior </td>
<td>All repos are enabled</td>
<td>No repos are enabled </td>
</tr>
<tr>
<td>Manually adding repositories</td>
<td>Manually adding repos disables Allstar on those repos</td>
<td>Manually adding repos enables Allstar on those repos</td>
</tr>
<tr>
<td>Additional configurations</td>
<td>optOutRepos: Allstar will be disabled on the listed repos<br>
<br>
optOutPrivateRepos: if true, Allstar will be disabled on all private repos
<br>
<br>
optOutPublicRepos: if true, Allstar will be disabled on all public
repos<br>
<br>
(optInRepos: this setting will be ignored)</td>
<td>optInRepos: Allstar will be enabled on the listed repos <br>
<br>
(optOutRepos: this setting will be ignored)</td>
</tr>
<tr>
<td>Repo Override </td>
<td>If true: Repos can opt out of their organization's Allstar enforcements
using the settings in their own repo file. Org level opt-in settings that
apply to that repository are ignored. <br>
<br>
If false: repos cannot opt out of Allstar enforcements as configured at the
org level. </td>
<td>If true: Repos can opt in to their organization's Allstar enforcements even
if they are not configured for the repo at the org level. Org level opt-out
settings that apply to that repository are ignored.<br>
<br>
If false: Repos cannot opt into Allstar enforcements if they are not
configured at the org level. </td>
</tr>
</tbody>
</table>

### Installation Options

Both the Quickstart and Manual Installation options involve installing the Allstar app. You may review the permissions requested. The app asks for read access to most settings and file contents to detect security compliance. It requests write access to issues and checks so that it can create issues and allow the `block` action.

#### Quickstart Installation 
This installation option will enable Allstar using the
Opt Out strategy on all repositories in your  organization. All current policies
will be enabled, and Allstar will alert you of
policy violations by filing an issue. This is the quickest and easiest way to start using Allstar, and you can still change any configurations later. 

Effort: very easy 

Steps:

1.  Install the Allstar app
    1.  [Open the installation
        page](https://github.com/apps/allstar-app) and click Configure
    1.  If you have multiple organizations, select the one you want to
        install Allstar on
    1.  Select "All Repositories" under Repository Access, even if you
        plan to disable Allstar on some repositories later
1.  Fork the sample repository
    1.  [Open the sample repository](https://github.com/jeffmendoza/dot-allstar-quickstart)
        and click the "Use this template" button
    1.  In the field for Repository Name, type `.allstar`
    1.  Click "Create repository from template"

That's it! All current Allstar [policies](#policies) are now enabled on all
your repositories. Allstar will create an issue if a policy is violated. 

To change any configurations, see the [manual installation directions](manual-install.md).

#### Manual Installation
This installation option will walk you through creating
configuration files according to either the Opt In or Opt Out strategy. This
option provides more granular control over configurations right from the start.

Effort: moderate

Steps:   
1) Install the [Allstar app](https://github.com/apps/allstar-app) (choose "All
Repositories" under Repository Access,  even if you don't plan to use Allstar on
all your repositories)  
2) Follow the [manual installation directions](manual-install.md) to create org-level or 
repository-level Allstar config files and individual policy files.  

## Policies and Actions

## **Actions**

Each policy can be configured with an action that Allstar will take when it
detects a repository to be out of compliance.

- `log`: This is the default action, and actually takes place for all
  actions. All policy run results and details are logged. Logs are currently
  only visible to the app operator, plans to expose these are under discussion.
- `issue`: This action creates a GitHub issue. Only one issue is created per
  policy, and the text describes the details of the policy violation. If the
  issue is already open, it is pinged with a comment every 24 hours without updates
  (not currently user configurable). If the policy result changes, a new comment
  will be left on the issue and linked in the issue body. Once the violation is
  addressed, the issue will be automatically closed by Allstar within 5-10 minutes.
- `fix`: This action is policy specific. The policy will make the changes to the
  GitHub settings to correct the policy violation. Not all policies will be able
  to support this (see below).

Proposed, but not yet implemented actions. Definitions will be added in the
future.

- `block`: Allstar can set a [GitHub Status
  Check](https://docs.github.com/en/github/collaborating-with-pull-requests/collaborating-on-repositories-with-code-quality-features/about-status-checks)
  and block any PR in the repository from being merged if the check fails.
- `email`: Allstar would send an email to the repository administrator(s).
- `rpc`: Allstar would send an rpc to some organization-specific system.

### **Action configuration**

Two settings are available to configure the issue action:

- `issueLabel` is available at the organization and repository level. Setting it
  will override the default `allstar` label used by Allstar to identify its
  issues.

- `issueRepo` is available at the organization level. Setting it will force all
  issues created in the organization to be created in the repository specified.

## **Policies**

Similar to the Allstar app enable configuration, all policies are enabled and
configured with a yaml file in either the organization's `.allstar` repository,
or the repository's `.allstar` directory. As with the app, policies are opt-in
by default, also the default `log` action won't produce visible results. A
simple way to enable all policies is to create a yaml file for each policy with
the contents:

```
optConfig:
  optOutStrategy: true
action: issue
```

The details of how the `fix` action works for each policy is detailed below. If omitted below, the `fix` action is not applicable.

### Branch Protection

This policy's config file is named `branch_protection.yaml`, and the [config
definitions are
here](https://pkg.go.dev/github.com/ossf/allstar/pkg/policies/branch#OrgConfig).

The branch protection policy checks that GitHub's [branch protection
settings](https://docs.github.com/en/github/administering-a-repository/defining-the-mergeability-of-pull-requests/about-protected-branches)
are setup correctly according to the specified configuration. The issue text
will describe which setting is incorrect. See [GitHub's
documentation](https://docs.github.com/en/github/administering-a-repository/defining-the-mergeability-of-pull-requests/about-protected-branches)
for correcting settings.

The `fix` action will change the branch protection settings to be in compliance with the specified policy configuration.

### Binary Artifacts

This policy's config file is named `binary_artifacts.yaml`, and the [config
definitions are
here](https://pkg.go.dev/github.com/ossf/allstar/pkg/policies/binary#OrgConfig).

This policy incorporates the [check from
scorecard](https://github.com/ossf/scorecard/#scorecard-checks). Remove the
binary artifact from the repository to achieve compliance. As the scorecard
results can be verbose, you may need to run [scorecard
itself](https://github.com/ossf/scorecard) to see all the detailed information.

### Outside Collaborators

This policy's config file is named `outside.yaml`, and the [config definitions
are
here](https://pkg.go.dev/github.com/ossf/allstar/pkg/policies/outside#OrgConfig).

This policy checks if any [Outside
Collaborators](https://docs.github.com/en/organizations/managing-access-to-your-organizations-repositories/adding-outside-collaborators-to-repositories-in-your-organization)
have either administrator(default) or push(optional) access to the
repository. Only organization members should have this access, as otherwise
untrusted members can change admin level settings and commit malicious code.

### SECURITY.md

This policy's config file is named `security.yaml`, and the [config definitions
are
here](https://pkg.go.dev/github.com/ossf/allstar/pkg/policies/security#OrgConfig).

This policy checks that the repository has a security policy file in
`SECURITY.md` and that it is not empty. The created issue will have a link to
the [GitHub
tab](https://docs.github.com/en/code-security/getting-started/adding-a-security-policy-to-your-repository)
that helps you commit a security policy to your repository.

### Dangerous Workflow

This policy's config file is named `dangerous_workflow.yaml`, and the [config
definitions are
here](https://pkg.go.dev/github.com/ossf/allstar/pkg/policies/workflow#OrgConfig).

This policy checks the GitHub Actions workflow configuration files
(`.github/workflows`), for any patterns that match known dangerous
behavior. See the [OpenSSF Scorecard
documentation](https://github.com/ossf/scorecard/blob/main/docs/checks.md#dangerous-workflow)
for more information on this check.

### Generic Scorecard Check

This policy's config file is named `scorecard.yaml`, and the [config definitions
are
here](https://pkg.go.dev/github.com/ossf/allstar/pkg/policies/scorecard#OrgConfig).

This policy runs any scorecard check listed in the `checks` configuration. All
checks run must have a score equal or above the `threshold` setting. Please see
the [OpenSSF Scorecard
documentation](https://github.com/ossf/scorecard/blob/main/docs/checks.md)
for more information on each check.

### GitHub Actions

This policy's config file is named `actions.yaml`, and the [config definitions
are
here](https://pkg.go.dev/github.com/ossf/allstar/pkg/policies/action#OrgConfig).

This policy checks the GitHub Actions workflow configuration files
(`.github/workflows`) (and workflow runs in some cases) in each repo to ensure
they are in line with rules (eg. require, deny) defined in the
organization-level config for the policy.

### Repository Administrators

This policy's config file is named `admin.yaml`, and the [config definitions
are
here](https://pkg.go.dev/github.com/ossf/allstar/pkg/policies/admin#OrgConfig).

This policy checks that by default all repositories must have a user or group assigned as an Administrator. It allows you to optionally configure if users are allowed to be administrators (as opposed to teams).

### Future Policies

- Ensure dependabot is enabled.
- Check that dependencies are pinned/frozen.

## **Example Config Repository**

See [this repo](https://github.com/GoogleContainerTools/.allstar) as an example
of Allstar config being used. As the organization administrator, consider a
README.md with some information on how Allstar is being used in your
organization.

## Advanced

### Configuration Definitions

- [Organization level enable configuration](https://pkg.go.dev/github.com/ossf/allstar/pkg/config#OrgOptConfig)
- [Repository Override enable configuration]( https://pkg.go.dev/github.com/ossf/allstar/pkg/config#RepoOptConfig)

### Secondary Org-Level configuration location

By default, org-level configuration files, such as the `allstar.yaml` file
above, are expected to be in a `.allstar` repository. If this repository does
not exist, then the `.github` repository `allstar` directory is used as a
secondary location. To clarify, for `allstar.yaml`:

| Precedence | Repository | Path |
| - | - | - |
| Primary | `.allstar` | `allstar.yaml` |
| Secondary | `.github` | `allstar/allstar.yaml` |

This is also true for the org-level configuration files for the individual
policies, as described below.

### Repo policy configurations in the Org Repo

Allstar will also look for repo-level policy configurations in the
organization's `.allstar` repository, under the directory with the same name as
the repository. This configuration is used regardless of whether "repo override"
is disabled.

For example, Allstar will lookup the policy configuration for a given repo
`myapp` in the following order:

| Repository | Path | Condition |
| - | - | - |
| `myapp` | `.allstar/branch_protection.yaml` | When "repo override" is allowed. |
| `.allstar` | `myapp/branch_protection.yaml` | All times. |
| `.allstar` | `branch_protection.yaml` | All times. |
| `.github` | `allstar/myapp/branch_protection.yaml` | If `.allstar` repo does not exist. |
| `.github` | `allstar/branch_protection.yaml` | If `.allstar` repo does not exist. |

### Org-level Base and Merge Configuration Location

For org-level Allstar and policy configuration files, you may specify the field
`baseConfig` to specify another repository that contains base Allstar
configuration. This is best explained with an example.

Suppose you have multiple GitHub organizations, but want to maintain a single
Allstar configuration. Your main organization is "acme", and the repository
`acme/.allstar` contains `allstar.yaml`:

```yaml
optConfig:
  optOutStrategy: true
issueLabel: allstar-acme
issueFooter: Issue created by Acme security team.
```

You also have a satellite GitHub organization named "acme-sat". You want to
re-use the main config, but apply some changes on top by disabling Allstar on
certain repositories. The repository `acme-sat/.allstar` contains
`allstar.yaml`:

```yaml
baseConfig: acme/.allstar
optConfig:
  optOutRepos:
  - acmesat-one
  - acmesat-two
```

This will use all the config from `acme/.allstar` as the base config, but then
apply any changes in the current file on top of the base configuration. The
method this is applied is described as a [JSON Merge
Patch](https://datatracker.ietf.org/doc/html/rfc7396). The `baseConfig` must be
a GitHub `<org>/<repository>`.

## **Contributing**

See [CONTRIBUTING.md](CONTRIBUTING.md)
