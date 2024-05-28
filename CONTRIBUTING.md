# Contributing

All community members must abide by the [OpenSSF Code of
Conduct.](https://openssf.org/community/code-of-conduct/)

* Feel free to open issues for bugs, feature requests, discussion,
  questions, help, proposals, etc.
* If you want to contribute a small fix or feature, open a PR.
* If you want to contribute something larger, a discussion or proposal
  issue may be appropriate.
* Please update docs when contributing features.
* When contributing large features, upate [whats-new.md](whats-new.md)
* All git commits must have [DCO](https://wiki.linuxfoundation.org/dco)

## Contributor Ladder

Allstar follows the [OpenSSF Scorecard contributor ladder](https://github.com/ossf/scorecard/blob/main/CONTRIBUTOR_LADDER.md).

Details on the previous Allstar contributor ladder can be found [here](/contributor-ladder.md).

## Community

Allstar is a part of the [OpenSSF Scorecard](https://github.com/ossf/scorecard) project.

We're hanging out in [#allstar](https://openssf.slack.com/archives/C02UQ2RL0HM) on the OpenSSF Slack workspace.

Meetings and additional community details are [here](https://github.com/ossf/scorecard#connect-with-the-scorecard-community).

## Development

* Run tests: `go test -v ./...`
* Run lint: `golangci-lint run`
* Local testing: See [operator.md](operator.md) to setup a test instance for yourself.

## Contribute Policies

[Interface definition.](pkg/policydef/policydef.go)

Both the [SECURITY.md](pkg/policies/security/security.go) and [Outside
Collaborators](pkg/policies/outside/outside.go) policies are quite simple to
understand and good examples to copy.
