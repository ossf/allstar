# Operator Instructions

Every Allstar deployment is operated by whoever uses it: you create a GitHub
App for your organization and run the process that authenticates as it. This
document covers both, plus the configuration available to you.

> [!NOTE]
> The OpenSSF previously ran a hosted Allstar app that anyone could install.
> It has been retired along with the infrastructure it ran on; see
> [ossf/scorecard#5208](https://github.com/ossf/scorecard/issues/5208).
> If you are moving off it, see [Migrating off the hosted
> app](README.md#migrating-off-the-hosted-app) — your existing configuration
> carries over unchanged.

The instructions below run Allstar as a persistent service. To run it as a
scheduled GitHub Action instead, create the app as described in [Create a
GitHub App](#create-a-github-app) and then follow the [GitHub Actions
installation directions](github-action-installation.md).

## Create a GitHub App

Follow [the instructions
here](https://docs.github.com/en/developers/apps/building-github-apps/creating-a-github-app)
to create a new app.

* **Name/Description/Homepage URL** Something specific to your instance.
* **Callback URL** Leave blank, Allstar does not auth as a user.
* **Request user authorization (OAuth) during installation** uncheck.
* **Webhooks/Subscribe to events** Uncheck and leave blank. Allstar does not
  listen for webhooks at this time.
* **Permissions** Follow this example: ![image](https://user-images.githubusercontent.com/771387/121067612-1bbc5200-c780-11eb-9bd3-214dfe808bf7.png)

> **Note:** If you plan to use the SARIF upload feature (`upload: {sarif: true}`
> in `scorecard.yaml`), your GitHub App also needs the **Code scanning alerts**
> repository permission set to **Read & write**. This is required for uploading
> SARIF results to the Code Scanning API.

> **Note:** As Allstar is developed, it may evolve the permissions needed or start
> listening for webhooks, please follow along development in this repo.

## Install the GitHub App

After creating the GitHub App, install it into each organization where you
want Allstar to enforce policies. From the GitHub App settings page, select
**Install App**, choose the organization, and complete the installation.

Creating the app and configuring its credentials is not sufficient on its own.
Allstar can authenticate successfully without any installations, but it will
not have repositories on which to enforce policies until the app is installed.

## Get ID and key.

See [the
instructions](https://docs.github.com/en/developers/apps/building-github-apps/authenticating-with-github-apps)
to create and download a private key. Also note down the App ID in the General /
About section of your new app.

Set the App ID in the `APP_ID` environment variable. Allstar has no default
here and will refuse to start without it.

For the private key, you have two options.

**A secret store.** Upload the private key contents to any service supported by
[Go CDK Runtime Configuration](https://gocloud.dev/howto/runtimevar/) — which
includes a local file — and set `KEY_SECRET` to the corresponding runtimevar
URL. For example:

```shell
export KEY_SECRET="gcpsecretmanager://projects/my-project/secrets/allstar-private-key?decoder=bytes"
```

Allstar registers the GCP Secret Manager driver by default. For any other
backend, add the matching import to `pkg/ghclients/ghclients.go`, ex:
`_ "gocloud.dev/runtimevar/awssecretsmanager"`.

**The environment directly.** Set `KEY_SECRET` to exactly `direct` and put the
key contents in `PRIVATE_KEY`. This is the default, because it is the only
option that needs no external infrastructure, and it is what the [GitHub
Action](github-action-installation.md) deployment uses with an encrypted
environment secret.

> **Warning:** supplying the key through `PRIVATE_KEY` puts it in the process
  environment, where it is visible to anything that can read the process or a
  crash dump. Prefer a secret store where you have one, and where you don't,
  make sure the surrounding platform protects the variable — as GitHub Actions
  environment secrets do.

Both values may also be changed in `pkg/config/operator/operator.go` if you are
building your own image, but the environment variables take precedence and are
the supported path.

## Run Allstar.

Build `cmd/allstar/` and run in any environment. No cli configuration
needed. Allstar does not currently listen to webhooks, so no incoming network
configuration needed. Only outgoing calls to GitHub are made. Allstar is
currently stateless. It is best to only run one instance to avoid potential race
conditions on enforcement actions, ex: pinging an issue twice at the same time.

### Reference deployment

The now-retired OpenSSF-operated instance ran as a single container provisioned
with 2 CPUs, 12 GB of memory, and 100 GB of disk. Treat that as a starting
point rather than a requirement, and scale based on how many repositories your
installations cover.

It set two non-default options:

* `ALLSTAR_NUM_WORKERS=1`, limiting Allstar to one organization or installation
  at a time (the default is 5).
* `DO_NOTHING_ON_OPT_OUT=true`, so that opted-out repositories are skipped
  before any policy check runs, rather than being checked and having their
  existing Allstar issues closed.

### Quick start for local testing

To validate your setup before deploying, build and run Allstar locally:

```shell
go build -o allstar ./cmd/allstar/

APP_ID=<your-app-id> \
KEY_SECRET=direct \
PRIVATE_KEY="$(cat /path/to/private-key.pem)" \
GITHUB_ALLOWED_ORGS=<your-org> \
ALLSTAR_LOG_LEVEL=debug \
DO_NOTHING_ON_OPT_OUT=true \
./allstar -once
```

Use `-once` to run a single enforcement cycle and exit. You can also filter
to a specific policy or repository:

```shell
./allstar -once -policy "OpenSSF Scorecard" -repo "myorg/myrepo"
```

## Configuration via Environment Variables

Allstar supports various operator configuration options which can be set via environment variables:

| Name                       | Description                                                                                                                                      | Default |
|----------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------|---------|
| APP_ID                     | The application ID of the created GitHub App. Required; Allstar exits at startup if it is unset.                                                 ||
| PRIVATE_KEY                | The raw value of the private key for the GitHub App. Required when KEY_SECRET is "direct".                                                       ||
| KEY_SECRET                 | A gocloud.dev runtimevar URL for a secret containing the private key, or "direct" to read the key from PRIVATE_KEY.                              | direct  |
| ALLSTAR_GHE_URL            | The URL of the GitHub Enterprise instance to use. Leave empty to use github.com                                                                  ||
| DO_NOTHING_ON_OPT_OUT      | Boolean flag which defines if allstar should do nothing and skip the corresponding checks when a repository is opted out.                        | false   |
| ALLSTAR_LOG_LEVEL          | The minimum logging level that allstar should use when emitting logs. Acceptable values are: panic ; fatal ; error ; warn ; info ; debug ; trace | info    |
| NOTICE_PING_DURATION_HOURS | The duration (in hours) to wait between pinging notice actions, such as updating a GitHub issue.                                                 | 24      |

## Self-hosted GitHub Enterprise specifics

In case you want to operate Allstar with a self-hosted GitHub Enterprise instance, you need to set the `ALLSTAR_GHE_URL` environment variable to the URL of your GitHub Enterprise instance URL.
The different API endpoints for API and upload are appended automatically.

Example:

Given, your GHE instance URL is "https://my-ghe.example.com", you need to set the following environment variables:

```shell
export ALLSTAR_GHE_URL="https://my-ghe.example.com"
export GH_HOST="my-ghe.example.com"  # This is used by the Scorecard dependency. Might result in errors if not set.
```
