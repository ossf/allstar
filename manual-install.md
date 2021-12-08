# **Manual Installation**

These directions walk you through manually installing Allstar on your organization or repository. 
For a faster setup that installs Allstar on all your repositories, see the Quickstart[TODO: link].
[TODO: finish intro]

[TODO: insert decision tree]

[TODO: insert links to three different Install Options below]

## Install Allstar on your Organization, **Opt Out Strategy** (Recommended)

1. Create a repository named `.allstar`.
2. Create a file named `allstar.yaml` with the contents:
   ```
   optConfig:
     optOutStrategy: true
   ```
   
3. Optional: Opt Out Repositories 
To opt some repositories out, change `allstar.yaml` to look like this:
   ```
   optConfig:
     optOutRepos:
     - repo-one
     - repo-two
   ```

To opt-out all private/public repositories, add `optOutPrivateRepos` or `optOutPublicRepos`. For example:
   ```
   optConfig:
     optOutStrategy: true
     optOutPrivateRepos: true
     optOutPublicRepos: false
   ```
4. Optional: Disable Resitory Override 

The repository override setting gives repositories the ability to opt themselves in or out of Allstar settings independent of configurations at the org level. 
If you prefer to strictly enforce your org-level settings on your repositories, you can disable repository override. Repositories will not be able to change Allstar settings that affect them without filing a PR to request org-level changes. 
To disable repository override, add the following to `allstar.yaml`:
   ```
   optConfig:
     disableRepoOverride: true
   ```

5. Required: To enable your policies, create four files with the names:
- `branch_protection.yaml`
- `binary_artifacts.yaml` 
- `outside.yaml`
- `security.yaml` 

In each of these four files, add the following contents:
   ```
   optConfig:
     optOutStrategy: true
   action: [choose action]
   ```
You will need to choose the action you would like Allstar to take when a policy is violated: `log`, `issue`, or `fix`. See [Actions](readme.md#actions) for more information about each policy. If you are unsure, we suggest using `issue` as a sensible default that can be changed later. For example:
   ```
   optConfig:
     optOut: true
   action: issue
   ```
## Install Allstar on your Organization, **Opt In Strategy**

1. Create a repository named `.allstar`.
2. Create a file named `allstar.yaml` with the contents:
   ```
   optConfig:
     optOutStrategy: false
   ```
3. Required: Add repositories to be opted in (Allstar will not run on any repositories if you do not specify which ones to opt in.)
To opt in some repositories, change `allstar.yaml` to look like this:
   ```
   optConfig:
    optOutRepos:
    - repo-one
    - repo-two
    ```

4. Optional: Disable Resitory Override 

The repository override setting gives repositories the ability to opt themselves in or out of Allstar settings independent of configurations at the org level. 
If you prefer to strictly enforce your org-level settings on your repositories, you can disable repository override. Repositories will not be able to change Allstar settings that affect them without filing a PR to request org-level changes. 
To disable repository override, add the following to `allstar.yaml`:
   ```
   optConfig:
     disableRepoOverride: true
   ```
   
5. Required: To enable your policies, create four files with the names:
- `branch_protection.yaml`
- `binary_artifacts.yaml` 
- `outside.yaml`
- `security.yaml` 

In each of these four files, add the following contents:
   ```
   optConfig:
     optOutStrategy: false
   action: [choose action]
   ```
You will need to choose the action you would like Allstar to take when a policy is violated: `log`, `issue`, or `fix`. See [Actions](readme.md#actions) for more information about each policy. If you are unsure, we suggest using `issue` as a sensible default that can be changed later. For example:
   ```
   optConfig:
     optOut: false
   action: issue
   ```
## Repository level

If you don't wish to create an org-level `.allstar` repository, Allstar can still be
used. All the defaults at the org-level config will be assumed. One of those is
the `disableRepoOverride` setting, which will be `false`. This allows individual
repositories to opt-in when the org-level setting is at the default opt-in strategy. 

To enable Allstar on a single repository:

1. In the repository, create a directory named `.allstar/`.
2. Create a file named `allstar.yaml` with the contents:
   ```
   optConfig:
     optIn: true
   ```
3. To enable your policies, create four files with the names:
- `branch_protection.yaml`
- `binary_artifacts.yaml` 
- `outside.yaml`
- `security.yaml` 

In each of these four files, add the following contents:
   ```
   optConfig:
     optOutStrategy: false
   action: [choose action]
   ```
You will need to choose the action you would like Allstar to take when a policy is violated: `log`, `issue`, or `fix`. See [Actions](readme.md#actions) for more information about each policy. If you are unsure, we suggest using `issue` as a sensible default that can be changed later. For example:
   ```
   optConfig:
     optOut: false
   action: issue
   ```
## More Options

See [Policies](README.md#policies) for more details on all the additional configuration
options available for each policy.

## Example Config Repository

See [this repo](https://github.com/GoogleContainerTools/.allstar) as an example
of Allstar config being used.
