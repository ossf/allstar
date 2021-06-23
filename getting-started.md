# **Getting Started**

> Ok, I have installed Allstar on my account/organization, now what?

By default, Allstar installed on your organization will not take any actions. To
quickly enable Allstar on all of your repos:

1. Create a repository named `.allstar`.
1. Create a file named `allstar.yaml` with the contents:
   ```
   optConfig:
     optOutStrategy: true
   ```
1. Create a file named `branch_protection.yaml` with the contents:
   ```
   optConfig:
     optOutStrategy: true
   action: issue
   ```
This will enable Allstar and the Branch Protection policy on all repos with the
default settings. The `issue` action will create GitHub issues in each repo for
violations of the security policy.

For all the available options in `allstar.yaml` see the comments on [the config
definition
here](https://github.com/ossf/allstar/blob/main/pkg/config/config.go#L29-L50). Similarly
For the options on the Branch Protection policy see [the config definition
here](https://github.com/ossf/allstar/blob/main/pkg/policies/branch/branch.go#L35-L62).

For example, if you want to enable Allstar on only a few repos, `allstar.yaml`
would look like this:
```
optConfig:
  optInRepos:
  - repo-one
  - repo-two
```
You can leave `branch_protection.yaml` the same and that policy will run only
run on the repos that Allstar is enabled on in the top level config above.

## Repo level

If you don't wish to create an org-level `.allstar` repo, Allstar can still be
used. All the defaults at the org-level config will be assumed. One of those is
the `disableRepoOverride` setting, which will be `false`. This allows individual
repos to opt-in when the org-level setting is at the default opt-in strategy. To
enable Allstar on a single repo:

1. Create a directory named `.allstar/`.
1. Create a file named `allstar.yaml` with the contents:
   ```
   optConfig:
     optIn: true
   ```
1. Create a file named `branch_protection.yaml` with the contents:
   ```
   optConfig:
     optIn: true
   action: issue
   ```
For repo-level config details see the corresponding definitions: [top
level](https://github.com/ossf/allstar/blob/main/pkg/config/config.go#L52-L66),
[branch
protection](https://github.com/ossf/allstar/blob/main/pkg/policies/branch/branch.go#L64-L87).

## Additional Policies

In addition to the Branch Protection policy described above, future policies will be
developed and [documented here](https://github.com/ossf/allstar/tree/main/pkg/policies).
