package reviewbot

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v43/github"
	"github.com/rs/zerolog/log"
)

// re-requested reviews should remove last review
// - fire event

type PullRequestInfo struct {
	owner          string
	repo           string
	user           string
	installationId int64
	headSHA        string
	number         int
}

func runPRCheck(config Config, pr PullRequestInfo) error {
	tr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appID, pr.installationId, config.GitHub.PrivateKeyPath)
	if err != nil {
		log.Error().Interface("pr", pr).Err(err).Msg("Could not read key")
		return err
	}

	client := github.NewClient(&http.Client{Transport: tr})
	ctx := context.Background()

	// TODO: get repo-level overwrites, if available
	minReviewsRequired := config.MinReviewsRequired

	// List of approvers to verify
	var approvalCandidates = map[string]bool{
		// Add PR Creator as someone to check
		pr.user: true,
	}

	optListReviews := &github.ListOptions{PerPage: 100}

	// Check reviews
	for {
		reviews, resp, err := client.PullRequests.ListReviews(ctx, pr.owner, pr.repo, pr.number, optListReviews)
		if err != nil {
			log.Error().Interface("pr", pr).Err(err).Msg("Could not list reviews")
			return err
		}

		for _, review := range reviews {
			login := review.GetUser().GetLogin()
			association := review.GetAuthorAssociation()
			state := review.GetState()

			// Ignore accounts without association with the repo and comments
			if association == "NONE" || state == "COMMENTED" {
				continue
			}

			log.Debug().Interface("pr", pr).Str("login", login).Str("association", association).Str("state", state).Msg("Found a review candidate")

			if state == "APPROVED" {
				approvalCandidates[login] = true
			} else {
				delete(approvalCandidates, login)
			}
		}

		if resp.NextPage == 0 {
			break
		}

		optListReviews.Page = resp.NextPage
	}

	// Points for approval
	var points uint64 = 0

	for login := range approvalCandidates {
		permissionLevel, _, err := client.Repositories.GetPermissionLevel(ctx, pr.owner, pr.repo, login)
		if err != nil {
			return err
		}

		permission := permissionLevel.GetPermission()
		isAuthorized := permission == "admin" || permission == "write"

		log.Debug().Interface("pr", pr).Str("login", login).Str("permission", permission).Msg("Approver Authorization")

		if isAuthorized {
			points++

			if points == minReviewsRequired {
				// no need to waste resources - we have enough authorized approvers
				break
			}
		}
	}

	log.Info().Interface("pr", pr).Uint64("points", points).Msg("Check's State")

	statusComplete := "completed"
	titlePrefix := "⭐️ Allstar Pull Request Review Bot - "
	text := fmt.Sprintf("PR has %d authorized approvals, %d required", points, minReviewsRequired)
	timestamp := github.Timestamp{
		Time: time.Now(),
	}

	check := github.CreateCheckRunOptions{
		Name:        "Allstar Review Bot",
		Status:      &statusComplete,
		CompletedAt: &timestamp,
		Output: &github.CheckRunOutput{
			Text: &text,
		},
		HeadSHA: pr.headSHA,
	}

	if points >= minReviewsRequired {
		conclusion := "success"
		title := titlePrefix + conclusion
		summary := "Pull request has enough authorized approvals"

		check.Conclusion = &conclusion
		check.Output.Title = &title
		check.Output.Summary = &summary
	} else {
		conclusion := "failure"
		title := titlePrefix + conclusion

		delta := minReviewsRequired - points
		deltaMessage := fmt.Sprintf("need %d more approval(s)", delta)

		summary := "Pull request does not have enough authorized approvals - " + deltaMessage

		check.Conclusion = &conclusion
		check.Output.Title = &title
		check.Output.Summary = &summary
	}

	checkRun, _, err := client.Checks.CreateCheckRun(ctx, pr.owner, pr.repo, check)
	if err != nil {
		return err
	}

	log.Info().Interface("pr", pr).Interface("Check Run", checkRun).Msg("Created Check Run")

	return nil
}
