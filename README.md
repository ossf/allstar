# **Allstar**

Allstar is a GitHub App installed on organizations or repositories to set and
enforce security policies. Its goal is to be able to detect any GitHub setting
or repository file contents that may be risky or not follow security best
practices. If Allstar finds a repository to be out of compliance, it will
take an action such as create an issue.

The specific policies are intended to be highly configurable, to try to meet the
needs of different project communities and organizations. Also, developing and
contributing new policies is intended to be easy.

Allstar is developed under the [OpenSSF](https://openssf.org/) and as a part of
the [Securing Critical Projects Working
Group](https://github.com/ossf/wg-securing-critical-projects). The OpenSSF runs
[an instance of Allstar here](https://github.com/apps/allstar-app) for anyone to
install and use on their GitHub organizations. However, Allstar can be run by
anyone if need be, see [the operator docs](operator.md) for more details.

## **Quick start**

[Install Allstar](https://github.com/apps/allstar-app) on your organizations and
repositories. Follow the [quick start
instructions](quick-start.md) to setup the configuration files needed to enable
Allstar on your repositories.

For more details on advanced configuration, see below.

### [Help! I'm getting issues created by Allstar and I don't want them.](opt-out.md)

## **Enable Configuration**

Allstar can be enabled on individual repositories at the app level, with
the option of enabling or disabling each security policy individually. For
organization-level configuration, create a repository named `.allstar` in your
organization. Then create a file called `allstar.yaml` in that
repository.

Allstar can either be set to an opt-in or opt-out strategy. In opt-in, only
those repositories explicitly listed are enabled. In opt-out, all repositories
are enabled, and repositories may be explicitly listed to opt-out. Allstar is
set to opt-in by default, and therefore is not enabled on any repository
immediately after installation. To continue with the default opt-in strategy,
list the repositories for Allstar to be enabled on in your organization like so:

```
optConfig:
  optInRepos:
  - repo-one
  - repo-two
```

To switch to the opt-out strategy, set that option to true:

```
optConfig:
  optOutStrategy: true
```

If you with to enable Allstar on all but a few repositories, you may use opt-out
and list the repositories to disable:

```
optConfig:
  optOutStrategy: true
  optOutRepos:
  - repo-one
  - repo-two
```

### Repository Override

Individual repositories can also opt in or out using configuration files inside
those repositories. For example, if the organization is configured with the
opt-out strategy, a repository may opt itself out by including the file
`.allstar/allstar.yaml` with the contents:

```
optConfig:
  optOut: true
```

Conversely, this allows repositories to opt-in and enable Allstar when the
organization is configured with the opt-in strategy. Because opt-in is the
default strategy, this is how Allstar works if the `.allstar` repository doesn't
exist.

In the organization-level `allstar.yaml`, repository override may be disabled with the setting:

```
optConfig:
  disableRepoOverride: true
```

This allows an organization-owner to have a central point of approval for
repositories to request an opt-out through a GitHub PR. Understandably, Allstar
or individual policies may not make sense for all repositories.

### Policy Enable

Each individual policy configuration file (see below) also contains the exact
same `optConfig` configuration object. This allows granularity to enable
policies on individual repositories. A policy will not take action unless
it is enabled **and** Allstar is enabled as a whole.

### Definition

https://pkg.go.dev/github.com/ossf/allstar@v0.0.0-20210728182754-005854d69ba7/pkg/config#OrgOptConfig
https://pkg.go.dev/github.com/ossf/allstar@v0.0.0-20210728182754-005854d69ba7/pkg/config#RepoOptConfig

## **Actions**

Each policy can be configured with an action that Allstar will take when it
detects a repository to be out of compliance.

- `log`: This is the default action, and actually takes place for all
  actions. All policy run results and details are logged. Logs are currently
  only visible to the app operator, plans to expose these are under discussion.
- `issue`: This action creates a GitHub issue. Only one issue is created per
  policy, and the text describes the details of the policy violation. If the
  issue is already open, it is pinged with a comment every 24 hours (not
  currently user configurable). Once the violation is addressed, the issue
  should be closed by Allstar within 5-10 minutes.
- `fix`: This action is policy specific. The policy will make the changes to the
  GitHub settings to correct the policy violation. Not all policies will be able
  to support this (see below).

Proposed, not yet implemented, actions. Further discussion and definition needed.

- `block`: Allstar can set a [GitHub Status
  Check](https://docs.github.com/en/github/collaborating-with-pull-requests/collaborating-on-repositories-with-code-quality-features/about-status-checks)
  and block any PR in the repository from being merged.
- `email`: Allstar could send an email somehow.
- `rpc`: Allstar could send an rpc to some organization-specific system.

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


### Branch Protection

This policy's config file is named `branch_protection.yaml`, and the [config
definitions are
here](https://pkg.go.dev/github.com/ossf/allstar@v0.0.0-20210728182754-005854d69ba7/pkg/policies/branch#OrgConfig).

The branch protection policy checks that GitHub's [branch protection
settings](https://docs.github.com/en/github/administering-a-repository/defining-the-mergeability-of-pull-requests/about-protected-branches)
are setup correctly according to the specified configuration. The `fix` action
is not yet implemented, but should be possible and is planned.

### Binary Artifacts

This policy's config file is named `binary_artifacts.yaml`, and the [config
definitions are
here](https://pkg.go.dev/github.com/ossf/allstar@v0.0.0-20210728182754-005854d69ba7/pkg/policies/binary#OrgConfig).

This policy incorporates the [check from scorecard](https://github.com/ossf/scorecard/#scorecard-checks). The `fix` action is not planned.

### Outside Collaborators

This policy's config file is named `outside.yaml`, and the [config definitions
are
here](https://pkg.go.dev/github.com/ossf/allstar@v0.0.0-20210728182754-005854d69ba7/pkg/policies/outside#OrgConfig).

This policy checks if any [Outside
Collaborators](https://docs.github.com/en/organizations/managing-access-to-your-organizations-repositories/adding-outside-collaborators-to-repositories-in-your-organization)
have either administrator(default) or push(optional) access to the
repository. Typically only organization members have this access.

The `fix` action is not implemented. I suppose Allstar could remove the access,
we need to check the permissinos we are currently requesting to see if this is
possible.

### SECURITY.md

This policy's config file is named `security.yaml`, and the [config definitions
are
here](https://pkg.go.dev/github.com/ossf/allstar@v0.0.0-20210728182754-005854d69ba7/pkg/policies/security#OrgConfig).

This policy checks that the repository has a security policy file in `SECURITY.md` and that it is not empty.

The `fix` action is not implemented. Possibly look into how the `.github`
organizaiton level repository works, and/or see how to open a PR with a set
contents defined in config.

### Future Policies

- Ensure dependabot is enabled.
- Check that dependencies are pinned/frozen.
- More checks from scorecard.

## **Example Config Repository** ##

See [this repo](https://github.com/GoogleContainerTools/.allstar) as an example
of Allstar config being used. As the organization adimistrator, consider a
README.md with some information on how Allstar is being used in your
organization.

## **Contribute Policies**

[Interface definition.](pkg/policydef/policydef.go)

Both the [SECURITY.md](pkg/policies/security/security.go) and [Outside
Collaborators](pkg/policies/outside/outside.go) policies are quite simple to
understand and good examples to copy.
