# How to disable Allstar 
If you are receiving unwanted issues created by Allstar, follow the instructions on this page to disable the app on your project. 

Allstar is highly configurable, so to disable it you need to know:

-  Whether Allstar was installed at the organization level or directly on your
    repository
-  Whether Allstar was configured using the opt-in or opt-out strategy 
  (if it was installed at the organization level)

If you already know how Allstar is configured on your organization or repository,
follow the instructions for the appropriate configuration:

[Disable Allstar, org-level opt-out strategy](#disable-allstar-org-level-opt-out-strategy)  
[Disable Allstar, org-level opt-in strategy](#disable-allstar-org-level-opt-in-strategy)  
[Disable Allstar, repository level](#disable-allstar-repository-level)

If you did not install Allstar yourself and do not know which instructions to
follow, you should contact your administrator to find out how Allstar is
configured on your organization.

If you are unable to contact the administrator, you can still disable of
Allstar, but it will take a little more work. Follow [these instructions
](#determine-how-allstar-is-configured)to figure out how Allstar is configured on your project. 

## Determine how Allstar is configured
<details>
  <summary>Click to expand</summary>

Follow these instructions if you are unable to contact your administrator to
find out how Allstar is configured on your organization or repository.

In your organization, find the repository named `.allstar`. 

In the `.allstar` repository, find the file named `allstar.yaml.`

In that file, look for a setting that says:

```
    optConfig:

      optOutStrategy: 
```

-  If `optOutStrategy` is set to `true`, follow the [opt-out strategy
    instructions](#disable-allstar-org-level-opt-out-strategy).

-  If `optOutStrategy` is set to `false`, follow the [opt-in strategy
    instructions](#disable-allstar-org-level-opt-in-strategy).

If this setting, file, or repository does not exist, it means that your project has been opted-in elsewhere and you will need to determine where:

Check the org-level `allstar.yaml` file for your repo. It may look like this:

```
optConfig:
  optInRepos:
  - other-repo
  - other-repo-two
  - my-repo-name-here
  - yet-another-repo
```

If your repository is on the `optInRepos` list, follow the [opt-in strategy
instructions](#disable-allstar-org-level-opt-in-strategy).  
    
If your repository is not listed in the allstar.yaml file, it means Allstar is
configured directly on your repository. Follow the [repository-level instuctions](#disable-allstar-repository-level).
</details>

## Disable Allstar, org-level opt-out strategy

These instuctions disable Allstar on a repository when Allstar is configured at the organization level using the opt-out strategy. 
   
In the `.allstar` repository in your organization, open the file named
`allstar.yaml`.   

Find the `optOutStrategy` setting: 

```
optConfig:
  optOutStrategy: true
```

To opt-out, submit a PR to the `.allstar` repo, and add the name of your
repository to the opt-out list:

```
optConfig:
  optOutStrategy: true
  optOutRepos:
  - my-repo-name-here
```

Allstar will be disabled on your repository when the pull request is merged. 

### Alternative option: with repo-override

This alternative option uses the `repo-override` setting to avoid the need to
submit a pull request to the organization's `.allstar` repo, but works only if:

-  the org-level `allstar.yaml` config has the line `disableRepoOverride:
    false` 

or 

-  the org-level `allstar.yaml` config file does not the include
    `disableRepoOverride` setting (which defaults to `false`).

If `disableRepoOverride` is set to `true`, the following instructions will not
work.  

To disable Allstar using repo-override, create a file in your repo named
`.allstar/allstar.yaml` with the contents:

```
optConfig:
  optOut: true
```

Merge this file to disable Allstar on your repository. 

## Disable Allstar, org-level opt-in strategy

These instuctions disable Allstar on a repository when Allstar is configured at the organization level using the opt-in strategy. 

In the org-level .allstar repository, open the `allstar.yaml` file. Find the
`optInRepos` setting:

```
optConfig:
  optInRepos:
  - other-repo
  - other-repo-two
  - my-repo-name-here
  - yet-another-repo
```

Submit a pull request to the `.allstar` repo that removes your repo name from that list.  

When the pull request is merged, Allstar should be disabled on your repository. If you still continue to receive issues, though, it means your project was also opted-in at the repository level. You must also follow the [repository-level instructions](disable-allstar-repository-level). 

## Disable Allstar, repository level

These instuctions disable Allstar when it is configured directly on your repository (not at the organization level). 

Look in your repository for a file named `.allstar/allstar.yaml`. It
    should contain this setting:

```
optConfig:
  optIn: true
```

Remove the `.allstar/allstar.yaml` file from your repository to
    disable Allstar.
