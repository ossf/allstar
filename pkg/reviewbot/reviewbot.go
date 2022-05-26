package reviewbot

import (
	"fmt"
	"net/http"

	"github.com/google/go-github/v43/github"
	"github.com/rs/zerolog/log"
)

const secretToken = "FooBar"
const appID = 169668

type Config struct {
	// Configuration for GitHub
	// TODO: future: option to get the below values from Secret Manager. See: `setKeySecret` in pkg/config/operator/operator.go
	GitHub struct {
		// The GitHub App's id.
		// See: https://docs.github.com/en/developers/apps/building-github-apps/authenticating-with-github-apps#authenticating-as-a-github-app
		AppId int64

		// Path to private key
		PrivateKeyPath string

		// See https://docs.github.com/en/developers/webhooks-and-events/webhooks/securing-your-webhooks
		SecretToken string
	}

	// The global minimum reviews reqiuired for approval
	MinReviewsRequired uint64

	// Port to listen on
	Port uint64
}

type WebookHandler struct {
	config Config
}

// Handle GitHub Webhooks for Review Bot.
//
// Example:
//   config := Config{...}
//   reviewbot.HandleWebhooks(&config)
//
func HandleWebhooks(config *Config) error {
	w := WebookHandler{*config}

	http.HandleFunc("/", w.HandleRoot)

	address := fmt.Sprintf(":%d", config.Port)

	return http.ListenAndServe(address, nil)
}

// Handle the root path
func (h *WebookHandler) HandleRoot(w http.ResponseWriter, r *http.Request) {
	// Validate payload
	payload, err := github.ValidatePayload(r, []byte(secretToken))
	if err != nil {
		log.Error().Interface("payload", payload).Err(err).Msg("Got an invalid payload")
		w.WriteHeader(400)
		if _, err := fmt.Fprintln(w, "Got an invalid payload"); err != nil {
			log.Error().Err(err).Msg("Failed to write http response")
		}
		return
	}

	// Parse the webhook to a GitHub event type
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		log.Error().Interface("event", event).Err(err).Msg("Failed to parse the webhook payload")
		w.WriteHeader(400)
		if _, err := fmt.Fprintln(w, "Failed to parse the webhook payload"); err != nil {
			log.Error().Err(err).Msg("Failed to write http response")
		}
		return
	}

	var pr PullRequestInfo

	// Extract relevant PR information from event, if is a PR-related event
	switch event := event.(type) {
	case *github.PullRequestEvent:
		pr = PullRequestInfo{
			owner:          event.GetRepo().GetOwner().GetLogin(),
			repo:           event.GetRepo().GetName(),
			user:           event.GetPullRequest().GetUser().GetLogin(),
			installationId: event.GetInstallation().GetID(),
			headSHA:        event.PullRequest.GetHead().GetSHA(),
			number:         event.GetPullRequest().GetNumber(),
		}
	default:
		log.Warn().Interface("event", event).Msg("Unknown event")
		w.WriteHeader(400)
		if _, err := fmt.Fprintln(w, "Unknown GitHub Event"); err != nil {
			log.Error().Err(err).Msg("Failed to write http response")
		}
		return
	}

	log.Info().Interface("pr", pr).Msg("Handling Pull Request Review Event")

	// Run PR Check
	err = runPRCheck(h.config, pr)
	if err != nil {
		log.Error().Interface("pr", pr).Err(err).Msg("Error handling webhook")
		w.WriteHeader(500)
		if _, err := fmt.Fprintln(w, "Error handling webhook"); err != nil {
			log.Error().Err(err).Msg("Failed to write http response")
		}
		return
	}
}
