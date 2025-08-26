# GitHub Action installation

These directions will help you run Allstar via GitHub Actions. If at all
possible, use the OpenSSF managed Allstar app instead (either using
[quickstart](README.md#quickstart-installation) or
[manual](README.md#manual-installation) methods) to avoid the burden of setup,
securing, maintaining, and troubleshooting this solution.

* [Create a GitHub App for Allstar](#create-a-github-app-for-allstar)
* [Setup the Allstar GitHub Action](#setup-the-allstar-github-action)
    * [Create your organization .allstar control repository](#create-your-organization-allstar-control-repository)
    * [Setup a recurring GitHub Action to run Allstar](#setup-a-recurring-github-action-to-run-allstar)
    * [Create the prod deployment environment](#create-the-prod-deployment-environment)
* [Monitoring](#monitoring)
* [Maintenance](#maintenance)
    * [Update the version of Allstar image used](#update-the-version-of-allstar-image-used)

## Create a GitHub App for Allstar

An "App" is like a service account: It is a user-like thing with a set of
permissions in your GitHub organization. Private key authentication can be used
to allow a GitHub Action (or anything) to authenticate as the "App".

See [Allstar - Operator - Create a GitHub App](operator.md#create-a-github-app)
When you create the app user make sure to record the `App ID` value.

## Setup the Allstar GitHub Action

The Allstar GitHub Action requires some setup before it is usable in a new
organization.

### Create your organization .allstar control repository
You must create a `.allstar` control repo to hold your Allstar configuration
as well as the GitHub Actions job to run Allstar.

The steps in [quickstart installation](README.md#quickstart-installation) or
[manual installation](README.md#manual-installation) can be used to create the
`.allstar` control repository. **Ignore the steps to install
the OpenSSF managed Allstar app into your organization!**

### Setup a recurring GitHub Action to run Allstar

1. Copy [`examples/gha-allstar-run.yml`](https://github.com/ossf/allstar/blob/main/examples/gha-allstar-run.yml)
   into `.github/workflows/allstar-run.yml` in your new `.allstar` control
   repository.
1. Edit `.github/workflows/allstar-run.yml`:
  1. You can update when the job runs by modifying its `schedule`:
     ~~~
     schedule:
       # M-F at 6:00am UTC
       - cron: '0 6 * * 1-5'
     ~~~
  1. You should check the version of Allstar container image used and update it
     if needed following [Update the version of Allstar image used](#update-the-version-of-allstar-image-used)
1. Commit changes and merge into `main`

The job will not function at this point.

### Create the prod deployment environment

To protect secrets we utilize the deployment environment feature of GitHub
Actions.

* In your GitHub organization under Settings -> Environments
  create the `prod` environment
* Uncheck "Allow administrators to bypass configured protection rules"
* Under "Deployment branches" switch to "Selected Branches"
* Click "Add deployment branch rule" and enter `main` then click "Add rule"
* Under "Environment variables" click "Add variable"
  * Name: `APP_ID`
  * Value: Enter the App ID for the app user
  * Click "Add variable" to complete
* Under "Environment secrets" click "Add secret"
  * Name: `PRIVATE_KEY`
  * Value: Paste the contents of the private key PEM downloaded in [Private key](#private-key)
  * Click "Add secret" to complete
* From this point, future Allstar GitHub Action runs on `main` should function.

## Monitoring

The example GitHub Action includes a post-processing stage named `analyze` that
parses Allstar output and generates a helpful overview. To see the summary:

* Under your `.allstar` control repo navigate to the Actions tab
* Under the Actions menu on the left, select "Allstar Enforcement Action"
* A list of enforcement actions will be shown - Click the run you would like to
  inspect
* Under the standard GitHub action pipeline display the "analyze summary" should
  be shown providing Scan Results by Check and Scan Results by Repository summaries
* Two artifacts are also generated:
  * `allstar-results.zip` - JSON versions of the analyze results
  * `allstar-scan` - Raw logs and errors from the Allstar run

## Maintenance

### Update the version of Allstar image used

The Allstar project publishes new container images as part of each release.
These are available from the [allstar container repository](https://github.com/ossf/allstar/pkgs/container/allstar/versions?filters%5Bversion_type%5D=tagged).

To update:

* Open a PR to update [.github/workflows/allstar-run.yml](.github/workflows/allstar-run.yml)
  with the new SHA256 fingerprint of the image you wish to use.
  * To find the fingerprint, go to the [Allstar containers page](https://github.com/ossf/allstar/pkgs/container/allstar)
    and find the most recent tag ending in `-gha` then click on `Digest ...`.
    Copy the SHA256 fingerprint.
  * Find the lines below in `allstar-run.yml` and update value after the `@`.
    For example:

  ~~~yaml
  container:
   image: ghcr.io/ossf/allstar@sha256:b9a32c3f54f3e96aa06003eb48acb9d4c32a70b5ec49bdc4f91b942b32b14969 # v4.4-gha
  ~~~

* Once reviewed and merged make sure to monitor the action under
  Actions and address any issues.

