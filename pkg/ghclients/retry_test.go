// Copyright 2026 Allstar Authors

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ghclients

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/google/go-github/v84/github"
	"github.com/hashicorp/go-retryablehttp"
)

func TestRetryRoundTripperRetriesGatewayErrors(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		switch requests.Add(1) {
		case 1:
			http.Error(w, "temporary bad gateway", http.StatusBadGateway)
		case 2:
			http.Error(w, "temporary gateway timeout", http.StatusGatewayTimeout)
		default:
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write([]byte(`{"name":"demo"}`)); err != nil {
				t.Errorf("response write failed: %v", err)
			}
		}
	}))
	defer server.Close()

	client := newTestGitHubClient(t, server, 2)
	repository, _, err := client.Repositories.Get(context.Background(), "demo", "demo")
	if err != nil {
		t.Fatalf("Repositories.Get returned an unexpected error: %v", err)
	}
	if got, want := repository.GetName(), "demo"; got != want {
		t.Errorf("repository name = %q, want %q", got, want)
	}
	if got, want := requests.Load(), int32(3); got != want {
		t.Errorf("request count = %d, want %d", got, want)
	}
}

func TestRetryRoundTripperDoesNotRetryOtherServerErrors(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := newTestGitHubClient(t, server, 2)
	_, response, err := client.Repositories.Get(context.Background(), "demo", "demo")
	if err == nil {
		t.Fatal("Repositories.Get returned nil error for HTTP 500")
	}
	if response == nil || response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("response = %#v, want HTTP 500", response)
	}
	if got, want := requests.Load(), int32(1); got != want {
		t.Errorf("request count = %d, want %d", got, want)
	}
}

func TestRetryRoundTripperDoesNotRetryNonIdempotentRequests(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Error(w, "temporary bad gateway", http.StatusBadGateway)
	}))
	defer server.Close()

	client := newTestGitHubClient(t, server, 2)
	_, response, err := client.Issues.Create(
		context.Background(), "demo", "demo", &github.IssueRequest{Title: github.Ptr("demo")})
	if err == nil {
		t.Fatal("Issues.Create returned nil error for HTTP 502")
	}
	if response == nil || response.StatusCode != http.StatusBadGateway {
		t.Fatalf("response = %#v, want HTTP 502", response)
	}
	if got, want := requests.Load(), int32(1); got != want {
		t.Errorf("request count = %d, want %d", got, want)
	}
}

func TestRetryRoundTripperReturnsLastGatewayResponse(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Error(w, "gateway timeout", http.StatusGatewayTimeout)
	}))
	defer server.Close()

	client := newTestGitHubClient(t, server, 2)
	_, response, err := client.Repositories.Get(context.Background(), "demo", "demo")
	if err == nil {
		t.Fatal("Repositories.Get returned nil error for persistent HTTP 504")
	}
	if response == nil || response.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("response = %#v, want HTTP 504", response)
	}
	if got, want := requests.Load(), int32(3); got != want {
		t.Errorf("request count = %d, want %d", got, want)
	}
}

func newTestGitHubClient(t *testing.T, server *httptest.Server, retryMax int) *github.Client {
	t.Helper()

	transport := newRetryRoundTripper(server.Client().Transport)
	retryTransport, ok := transport.(*retryablehttp.RoundTripper)
	if !ok {
		t.Fatalf("transport has type %T, want *retryablehttp.RoundTripper", transport)
	}
	retryTransport.Client.RetryWaitMin = 0
	retryTransport.Client.RetryWaitMax = 0
	retryTransport.Client.RetryMax = retryMax

	client := github.NewClient(&http.Client{Transport: transport})
	baseURL, err := url.Parse(server.URL + "/")
	if err != nil {
		t.Fatalf("url.Parse returned an unexpected error: %v", err)
	}
	client.BaseURL = baseURL
	return client
}
