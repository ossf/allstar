# How to opt-out
> Help! I'm getting issues created by Allstar and I don't want them.

## Allstar is configured in the opt-out strategy
To determine if Allstar is configured in the opt-out strategy, there will be a repo named `.allstar` in your organization, with a file named `allstar.yaml`. In that file will be the setting:

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
To determine if Allstar is configured in the opt-in strategy, there may be a repo named `.allstar` in your organization, with a file named `allstar.yaml`. In that file may be the setting:

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

Another way your repo may be opted-in is a file in your repo named `.allstar/allstar.yaml` with contents such as:

    optConfig:
      optIn: true

Removing that file will disable Allstar.
