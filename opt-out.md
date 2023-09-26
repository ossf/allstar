# How to disable Allstar 
If you are receiving unwanted issues created by Allstar, follow the instructions on this page to disable the app on your project. 

[Disable Allstar, org-level opt-out strategy](#disable-allstar-org-level-opt-out-strategy)  
[Disable Check, repository level](#disable-a-specific-check-with-repo-override)  
[Disable Allstar, repo level](#disable-allstar-with-repo-override)


## Disable Allstar, org-level opt-out strategy

These instructions disable Allstar on a repository when Allstar is configured at the organization level using the opt-out strategy. 
   
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

### Disable a specific check with repo-override

To opt-out of a specific check in your repo create `.allstar/control-name.yaml` and add
```
optConfig:
  optOut: true
```

Merge this file to disable Allstar on your repository. 

### Disable allstar with repo-override

To disable Allstar using repo-override, create a file in your repo named
`.allstar/allstar.yaml` with the contents:

```
optConfig:
  optOut: true
```

Merge this file to disable Allstar on your repository. 
