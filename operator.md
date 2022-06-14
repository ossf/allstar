# Operator Instructions

You don't need to run an instance of Allstar to use it. The OpenSSF runs an
instance that anyone can [install and use
here](https://github.com/apps/allstar-app). However, you may wish to create and
run your own instance of Allstar for security or customization reasons.

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


> **Note:** As Allstar is developed, it may evolve the permissions needed or start
> listening for webhooks, please follow along development in this repo.

## Get ID and key.

See [the
instructions](https://docs.github.com/en/developers/apps/building-github-apps/authenticating-with-github-apps)
to create and download a private key. Also note down the App ID in the General /
About section of your new app.

Upload the private key contents to a supported service by [Go CDK Runtime
Configuration](https://gocloud.dev/howto/runtimevar/). Also note, the Runtime
Configuration library will support a local file as well.

Edit `pkg/config/operator/operator.go` and set the AppID and KeySecret
link. Alternatively, you can provide the AppID and KeySecret as environment
variables `APP_ID` and `KEY_SECRET`. You may need to edit
`pkg/ghclients/ghclients.go` and add a new import line for your secret service,
ex: `_ "gocloud.dev/runtimevar/gcpsecretmanager"`.

> **Warning, this is not a recommended practice for security.** If you are
  not using a supported runtime you may provide the contents of the private key
  directly in the environment variable `PRIVATE_KEY`. Allstar will only use this
  if the contents of `KEY_SECRET` is set exactly to `direct`.

## Run Allstar.

Build `cmd/allstar/` and run in any environment. No cli configuration
needed. Allstar does not currently listen to webhooks, so no incoming network
configuration needed. Only outgoing calls to GitHub are made. Allstar is
currently stateless. It is best to only run one instance to avoid potential race
conditions on enforcement actions, ex: pinging an issue twice at the same time.
