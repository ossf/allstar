# How to opt-out
> Help! I'm getting issues created by Allstar and I don't want them.

To discontinue receiving issues created by Allstar, first determine whether there is a repo named `.allstar` in your organization.

If there is no `.allstar` repo in your org, then Allstar is configured at the repository level. Look for a file in your repo named `.allstar/allstar.yaml` with contents such as:
```
    optConfig:
      optIn: true
```
Simply remove the `.allstar/allstar.yaml`  file to disable Allstar on you repository.


## Org Level Configurations
If there is a repo named .allstar in your org, you will need to dig a little to determine your org's settings. 

In the organization's repo named `.allstar`, look for a file named `allstar.yaml`. In this file, find the following optConfig settings to determine whether Allstar is configured in the Opt Out or Opt In strategy:


    optConfig:
      optOutStrategy: true/false

## Allstar is configured in the opt-out strategy
If the setting is set to true, your org is using the Opt Out strategy.

    optConfig:
      optOutStrategy: true

To opt-out, submit a PR to that `.allstar` repo, and add the name of your repository to the opt-out list. ex:

    optConfig:
      optOutStrategy: true
      optOutRepos:
      - my-repo-name-here

### With repo-override

If that org-level `allstar.yaml` config has the line `disableRepoOverride: false`, or if that line doesn't exist (default is false). Then you may optionally opt-out by creating a file in your repo instead of sending a PR to the org-level `.allstar` repo. Create a file in your repo named `.allstar/allstar.yaml` with the contents:

    optConfig:
      optOut: true

> If you see `disableRepoOverride: true` in the top-level config, this will not work.

## Allstar is configured in the opt-in strategy
If the setting is set to false, your repo is using the Opt In strategy.

    optConfig:
      optOutStrategy: false

Opt-in is the default strategy, so if that repo, file, or setting is missing: Allstar is set to opt-in. If Allstar is set to opt-in and you are seeing Allstar actions (issues created), then your repo must be explicitly opted-in somewhere. Check that org-level `allstar.yaml` file for your repo. It may look like this:

    optConfig:
      optInRepos:
      - other-repo
      - other-repo-two
      - my-repo-name-here
      - yet-another-repo

Sumit a PR to the `.allstar` repo removing your repo name from that list.

