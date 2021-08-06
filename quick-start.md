# **Quick Start**

> Ok, I have installed Allstar on my account/organization, now what?

By default, Allstar installed on your organization will not take any actions. To
quickly enable Allstar on all of your repositories:

1. Create a repository named `.allstar`.
1. Create a file named `allstar.yaml` with the contents:
   ```
   optConfig:
     optOutStrategy: true
   ```
1. Create four files with the names `branch_protection.yaml`,
   `binary_artifacts.yaml`, `outside.yaml`, and `security.yaml` with the
   contents:

   ```
   optConfig:
     optOutStrategy: true
   action: issue
   ```
This will enable Allstar and all the policies on all repositories with the
default settings. The `issue` action will create GitHub issues in each repository for
violations of the security policy.

If you want to only enable a few repositories in the organization, change
`allstar.yaml` to look like this:

```
optConfig:
  optInRepos:
  - repo-one
  - repo-two
```

You can leave the other files the same as those policies will run only on the
repositories that Allstar is enabled on in the top level config above.

## Repository level

If you don't wish to create an org-level `.allstar` repository, Allstar can still be
used. All the defaults at the org-level config will be assumed. One of those is
the `disableRepoOverride` setting, which will be `false`. This allows individual
repositories to opt-in when the org-level setting is at the default opt-in strategy. To
enable Allstar on a single repository:

1. Create a directory named `.allstar/`.
1. Create a file named `allstar.yaml` with the contents:
   ```
   optConfig:
     optIn: true
   ```
1. Create four files with the names `branch_protection.yaml`,
   `binary_artifacts.yaml`, `outside.yaml`, and `security.yaml` with the
   contents:

   ```
   optConfig:
     optIn: true
   action: issue
   ```
## More details

See the [main README](README.md) for more details on all the configuration
options available.

## Example Config Repository

See [this repo](https://github.com/GoogleContainerTools/.allstar) as an example
of Allstar config being used.
